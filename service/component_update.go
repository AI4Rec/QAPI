package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	goversion "github.com/hashicorp/go-version"
)

const (
	ManagedComponentCLIProxyAPI = "cliproxyapi"
	ManagedComponentSub2API     = "sub2api"

	ComponentUpdateActionApply    = "update"
	ComponentUpdateActionRollback = "rollback"
)

const componentReleaseCacheTTL = 5 * time.Minute

var componentVersionPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

type managedComponentSpec struct {
	Repository  string
	CurrentLink string
	LegacyPath  string
	Previous    string
}

var managedComponentSpecs = map[string]managedComponentSpec{
	ManagedComponentCLIProxyAPI: {
		Repository:  "router-for-me/CLIProxyAPI",
		CurrentLink: "/opt/cliproxyapi/current",
		LegacyPath:  "/opt/cliproxyapi/cli-proxy-api",
		Previous:    "/opt/cliproxyapi/previous",
	},
	ManagedComponentSub2API: {
		Repository:  "Wei-Shaw/sub2api",
		CurrentLink: "/opt/sub2api/current",
		Previous:    "/opt/sub2api/previous",
	},
}

type ComponentReleaseStatus struct {
	Component         string `json:"component"`
	Repository        string `json:"repository"`
	CurrentVersion    string `json:"current_version"`
	LatestVersion     string `json:"latest_version"`
	ReleaseName       string `json:"release_name"`
	ReleaseNotes      string `json:"release_notes"`
	ReleaseURL        string `json:"release_url"`
	PublishedAt       string `json:"published_at"`
	UpdateAvailable   bool   `json:"update_available"`
	RollbackAvailable bool   `json:"rollback_available"`
	UpdaterAvailable  bool   `json:"updater_available"`
	CheckedAt         int64  `json:"checked_at"`
}

type componentGitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

type cachedComponentRelease struct {
	release   componentGitHubRelease
	expiresAt time.Time
}

type managedComponentInspection struct {
	CurrentVersion    string
	RollbackAvailable bool
}

var componentReleaseCache = struct {
	sync.Mutex
	items map[string]cachedComponentRelease
}{items: map[string]cachedComponentRelease{}}

func IsManagedComponent(component string) bool {
	_, ok := managedComponentSpecs[component]
	return ok
}

func GetComponentReleaseStatus(ctx context.Context, component string, force bool) (*ComponentReleaseStatus, error) {
	spec, ok := managedComponentSpecs[component]
	if !ok {
		return nil, errors.New("unsupported component")
	}

	inspection, inspectErr := inspectManagedComponent(ctx, component)
	if inspectErr != nil {
		currentVersion, err := readManagedComponentVersion(ctx, component, spec)
		if err != nil {
			return nil, err
		}
		inspection.CurrentVersion = currentVersion
		inspection.RollbackAvailable = managedComponentLinkExists(spec.Previous)
	}
	release, err := getLatestComponentRelease(ctx, spec.Repository, force)
	if err != nil {
		return nil, err
	}

	status := &ComponentReleaseStatus{
		Component:         component,
		Repository:        spec.Repository,
		CurrentVersion:    inspection.CurrentVersion,
		LatestVersion:     normalizeComponentVersion(release.TagName),
		ReleaseName:       release.Name,
		ReleaseNotes:      release.Body,
		ReleaseURL:        release.HTMLURL,
		PublishedAt:       release.PublishedAt,
		RollbackAvailable: inspection.RollbackAvailable,
		UpdaterAvailable:  managedComponentUpdaterAvailable(),
		CheckedAt:         common.GetTimestamp(),
	}
	status.UpdateAvailable = componentVersionIsNewer(status.LatestVersion, status.CurrentVersion)
	return status, nil
}

func RunComponentUpdate(ctx context.Context, action string, component string, targetVersion string) (string, error) {
	if !IsManagedComponent(component) {
		return "", errors.New("unsupported component")
	}
	if action != ComponentUpdateActionApply && action != ComponentUpdateActionRollback {
		return "", errors.New("unsupported component update action")
	}
	if action == ComponentUpdateActionApply && !componentVersionPattern.MatchString(targetVersion) {
		return "", errors.New("invalid target version")
	}

	updaterPath := componentUpdaterPath()
	if _, err := os.Stat(updaterPath); err != nil {
		return "", errors.New("component updater is not installed")
	}

	args := []string{"-n", updaterPath, action, component}
	if action == ComponentUpdateActionApply {
		args = append(args, normalizeComponentVersion(targetVersion))
	}
	output, err := exec.CommandContext(ctx, "sudo", args...).CombinedOutput()
	message := strings.TrimSpace(string(output))
	if len(message) > 4096 {
		message = message[len(message)-4096:]
	}
	if err != nil {
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("component updater failed: %s", message)
	}
	return message, nil
}

func getLatestComponentRelease(ctx context.Context, repository string, force bool) (componentGitHubRelease, error) {
	componentReleaseCache.Lock()
	cached, ok := componentReleaseCache.items[repository]
	if !force && ok && time.Now().Before(cached.expiresAt) {
		componentReleaseCache.Unlock()
		return cached.release, nil
	}
	componentReleaseCache.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repository+"/releases/latest", nil)
	if err != nil {
		return componentGitHubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "qapi-component-update-checker")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return componentGitHubRelease{}, fmt.Errorf("failed to contact GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return componentGitHubRelease{}, fmt.Errorf("GitHub release check returned HTTP %d", response.StatusCode)
	}

	release := componentGitHubRelease{}
	if err := common.DecodeJson(io.LimitReader(response.Body, 2<<20), &release); err != nil {
		return componentGitHubRelease{}, fmt.Errorf("failed to parse GitHub release: %w", err)
	}
	if !componentVersionPattern.MatchString(release.TagName) {
		return componentGitHubRelease{}, errors.New("GitHub returned an invalid release version")
	}

	componentReleaseCache.Lock()
	componentReleaseCache.items[repository] = cachedComponentRelease{
		release:   release,
		expiresAt: time.Now().Add(componentReleaseCacheTTL),
	}
	componentReleaseCache.Unlock()
	return release, nil
}

func readManagedComponentVersion(ctx context.Context, component string, spec managedComponentSpec) (string, error) {
	if target, err := filepath.EvalSymlinks(spec.CurrentLink); err == nil {
		version := normalizeComponentVersion(filepath.Base(target))
		if componentVersionPattern.MatchString(version) {
			return version, nil
		}
	}

	if component == ManagedComponentCLIProxyAPI {
		output, _ := exec.CommandContext(ctx, spec.LegacyPath, "--version").CombinedOutput()
		match := regexp.MustCompile(`(?i)CLIProxyAPI Version:\s*v?([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)`).FindStringSubmatch(string(output))
		if len(match) == 2 {
			return normalizeComponentVersion(match[1]), nil
		}
	}
	return "", errors.New("failed to detect installed component version")
}

func normalizeComponentVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	return "v" + strings.TrimPrefix(version, "v")
}

func componentVersionIsNewer(latest string, current string) bool {
	latestVersion, latestErr := goversion.NewVersion(strings.TrimPrefix(latest, "v"))
	currentVersion, currentErr := goversion.NewVersion(strings.TrimPrefix(current, "v"))
	if latestErr != nil || currentErr != nil {
		return latest != "" && latest != current
	}
	return latestVersion.GreaterThan(currentVersion)
}

func managedComponentLinkExists(path string) bool {
	target, err := filepath.EvalSymlinks(path)
	return err == nil && target != ""
}

func managedComponentUpdaterAvailable() bool {
	info, err := os.Stat(componentUpdaterPath())
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func componentUpdaterPath() string {
	updaterPath := strings.TrimSpace(os.Getenv("QAPI_COMPONENT_UPDATER_PATH"))
	if updaterPath == "" {
		updaterPath = "/usr/local/sbin/qapi-component-updater"
	}
	return updaterPath
}

func inspectManagedComponent(ctx context.Context, component string) (managedComponentInspection, error) {
	if !IsManagedComponent(component) {
		return managedComponentInspection{}, errors.New("unsupported component")
	}
	updaterPath := componentUpdaterPath()
	if _, err := os.Stat(updaterPath); err != nil {
		return managedComponentInspection{}, err
	}
	output, err := exec.CommandContext(ctx, "sudo", "-n", updaterPath, "inspect", component).CombinedOutput()
	if err != nil {
		return managedComponentInspection{}, fmt.Errorf("component inspection failed: %s", strings.TrimSpace(string(output)))
	}
	return parseManagedComponentInspection(string(output))
}

func parseManagedComponentInspection(output string) (managedComponentInspection, error) {
	inspection := managedComponentInspection{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "current_version":
			inspection.CurrentVersion = normalizeComponentVersion(value)
		case "rollback_available":
			inspection.RollbackAvailable = value == "true"
		}
	}
	if !componentVersionPattern.MatchString(inspection.CurrentVersion) {
		return managedComponentInspection{}, errors.New("component updater returned an invalid installed version")
	}
	return inspection, nil
}
