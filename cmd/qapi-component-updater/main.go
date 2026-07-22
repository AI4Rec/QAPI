package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	actionUpdate   = "update"
	actionRollback = "rollback"
	actionInspect  = "inspect"
	maxBinaryBytes = 512 << 20
)

var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

type componentSpec struct {
	name         string
	repository   string
	installDir   string
	currentLink  string
	previousLink string
	binaryName   string
	serviceName  string
	healthURL    string
}

var components = map[string]componentSpec{
	"cliproxyapi": {
		name:         "cliproxyapi",
		repository:   "router-for-me/CLIProxyAPI",
		installDir:   "/opt/cliproxyapi",
		currentLink:  "/opt/cliproxyapi/current",
		previousLink: "/opt/cliproxyapi/previous",
		binaryName:   "cli-proxy-api",
		serviceName:  "cliproxyapi.service",
		healthURL:    "http://127.0.0.1:8317/",
	},
	"sub2api": {
		name:         "sub2api",
		repository:   "Wei-Shaw/sub2api",
		installDir:   "/opt/sub2api",
		currentLink:  "/opt/sub2api/current",
		previousLink: "/opt/sub2api/previous",
		binaryName:   "sub2api",
		serviceName:  "sub2api.service",
		healthURL:    "http://127.0.0.1:18080/health",
	},
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func main() {
	if os.Geteuid() != 0 {
		fatal(errors.New("qapi-component-updater must run as root"))
	}
	if len(os.Args) < 3 {
		fatal(errors.New("usage: qapi-component-updater <update|rollback|inspect> <cliproxyapi|sub2api> [version]"))
	}

	action := os.Args[1]
	spec, ok := components[os.Args[2]]
	if !ok {
		fatal(errors.New("unsupported component"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error
	switch action {
	case actionUpdate:
		if len(os.Args) != 4 || !versionPattern.MatchString(os.Args[3]) {
			fatal(errors.New("a valid target version is required"))
		}
		err = updateComponent(ctx, spec, os.Args[3])
	case actionRollback:
		if len(os.Args) != 3 {
			fatal(errors.New("rollback does not accept a target version"))
		}
		err = rollbackComponent(ctx, spec)
	case actionInspect:
		if len(os.Args) != 3 {
			fatal(errors.New("inspect does not accept a target version"))
		}
		err = inspectComponent(spec)
	default:
		err = errors.New("unsupported action")
	}
	if err != nil {
		fatal(err)
	}
}

func inspectComponent(spec componentSpec) error {
	releasesDir := filepath.Join(spec.installDir, "releases")
	currentTarget, err := validatedLinkTarget(spec.currentLink, releasesDir)
	if err != nil {
		return fmt.Errorf("current release is unavailable: %w", err)
	}
	rollbackAvailable := false
	if previousTarget, previousErr := validatedLinkTarget(spec.previousLink, releasesDir); previousErr == nil {
		rollbackAvailable = previousTarget != currentTarget
	}
	fmt.Printf("current_version=%s\nrollback_available=%t\n", filepath.Base(currentTarget), rollbackAvailable)
	return nil
}

func updateComponent(ctx context.Context, spec componentSpec, version string) error {
	currentTarget, err := validatedLinkTarget(spec.currentLink, filepath.Join(spec.installDir, "releases"))
	if err != nil {
		return fmt.Errorf("current release is unavailable: %w", err)
	}
	if filepath.Base(currentTarget) == version {
		fmt.Printf("%s is already running %s\n", spec.name, version)
		return nil
	}

	if spec.name == "sub2api" {
		if err := backupSub2API(ctx, version); err != nil {
			return err
		}
	}

	releaseDir := filepath.Join(spec.installDir, "releases", version)
	binaryPath := filepath.Join(releaseDir, spec.binaryName)
	if _, err := os.Stat(binaryPath); errors.Is(err, os.ErrNotExist) {
		if err := downloadRelease(ctx, spec, version, releaseDir); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if err := atomicSymlink(currentTarget, spec.previousLink); err != nil {
		return fmt.Errorf("failed to preserve previous release: %w", err)
	}
	if err := activateRelease(ctx, spec, releaseDir, currentTarget); err != nil {
		return err
	}
	fmt.Printf("updated %s from %s to %s\n", spec.name, filepath.Base(currentTarget), version)
	return nil
}

func rollbackComponent(ctx context.Context, spec componentSpec) error {
	releasesDir := filepath.Join(spec.installDir, "releases")
	currentTarget, err := validatedLinkTarget(spec.currentLink, releasesDir)
	if err != nil {
		return err
	}
	previousTarget, err := validatedLinkTarget(spec.previousLink, releasesDir)
	if err != nil {
		return errors.New("no rollback release is available")
	}
	if currentTarget == previousTarget {
		return errors.New("rollback release matches the current release")
	}
	if spec.name == "sub2api" {
		if err := backupSub2API(ctx, filepath.Base(previousTarget)); err != nil {
			return err
		}
	}

	if err := activateRelease(ctx, spec, previousTarget, currentTarget); err != nil {
		return err
	}
	if err := atomicSymlink(currentTarget, spec.previousLink); err != nil {
		return fmt.Errorf("rollback succeeded but previous link could not be updated: %w", err)
	}
	fmt.Printf("rolled back %s from %s to %s\n", spec.name, filepath.Base(currentTarget), filepath.Base(previousTarget))
	return nil
}

func downloadRelease(ctx context.Context, spec componentSpec, version string, releaseDir string) error {
	release, err := fetchRelease(ctx, spec.repository, version)
	if err != nil {
		return err
	}
	assetName := releaseAssetName(spec, version)
	assetURL := ""
	checksumURL := ""
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			assetURL = asset.DownloadURL
		case "checksums.txt":
			checksumURL = asset.DownloadURL
		}
	}
	if assetURL == "" || checksumURL == "" {
		return fmt.Errorf("release %s does not contain the required linux asset or checksums", version)
	}

	tempDir, err := os.MkdirTemp(filepath.Join(spec.installDir, "releases"), ".update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, assetName)
	if err := downloadFile(ctx, assetURL, archivePath, 512<<20); err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	checksumsPath := filepath.Join(tempDir, "checksums.txt")
	if err := downloadFile(ctx, checksumURL, checksumsPath, 2<<20); err != nil {
		return fmt.Errorf("failed to download release checksums: %w", err)
	}
	if err := verifyChecksum(archivePath, checksumsPath, assetName); err != nil {
		return err
	}

	stagingDir := releaseDir + ".partial-" + strconv.Itoa(os.Getpid())
	if err := os.Mkdir(stagingDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)
	if err := extractBinary(archivePath, filepath.Join(stagingDir, spec.binaryName), spec.binaryName); err != nil {
		return err
	}
	if err := os.Rename(stagingDir, releaseDir); err != nil {
		return fmt.Errorf("failed to publish release directory: %w", err)
	}
	return nil
}

func fetchRelease(ctx context.Context, repository string, version string) (githubRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repository+"/releases/tags/"+version, nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "qapi-component-updater")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub release lookup returned HTTP %d", response.StatusCode)
	}
	release := githubRelease{}
	if err := common.DecodeJson(io.LimitReader(response.Body, 4<<20), &release); err != nil {
		return githubRelease{}, err
	}
	if release.TagName != version {
		return githubRelease{}, errors.New("GitHub release tag did not match the requested version")
	}
	return release, nil
}

func releaseAssetName(spec componentSpec, version string) string {
	number := strings.TrimPrefix(version, "v")
	arch := runtime.GOARCH
	if spec.name == "sub2api" && arch == "arm64" {
		return "sub2api_" + number + "_linux_arm64.tar.gz"
	}
	if spec.name == "sub2api" {
		return "sub2api_" + number + "_linux_amd64.tar.gz"
	}
	if arch == "arm64" {
		arch = "aarch64"
	}
	return "CLIProxyAPI_" + number + "_linux_" + arch + ".tar.gz"
}

func downloadFile(ctx context.Context, url string, destination string, maxBytes int64) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 2 * time.Minute}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxBytes {
		return errors.New("download exceeded the maximum allowed size")
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxBytes {
		return errors.New("download exceeded the maximum allowed size")
	}
	return nil
}

func verifyChecksum(archivePath string, checksumsPath string, assetName string) error {
	checksums, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}
	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if len(expected) != sha256.Size*2 {
		return errors.New("release checksum is missing or invalid")
	}

	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return errors.New("release checksum verification failed")
	}
	return nil
}

func extractBinary(archivePath string, destination string, binaryName string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		if header.Size < 0 || header.Size > maxBinaryBytes {
			return errors.New("release binary exceeded the maximum allowed size")
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(tarReader, maxBinaryBytes+1))
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != header.Size {
			return errors.New("release binary size did not match the archive header")
		}
		return nil
	}
	return errors.New("release archive did not contain the expected binary")
}

func activateRelease(ctx context.Context, spec componentSpec, target string, fallback string) error {
	if err := atomicSymlink(target, spec.currentLink); err != nil {
		return err
	}
	if err := runCommand(ctx, "systemctl", "restart", spec.serviceName); err == nil {
		if healthErr := waitForHealth(ctx, spec.healthURL); healthErr == nil {
			return nil
		}
	}

	_ = atomicSymlink(fallback, spec.currentLink)
	_ = runCommand(ctx, "systemctl", "restart", spec.serviceName)
	_ = waitForHealth(ctx, spec.healthURL)
	return errors.New("new release failed health checks and the previous release was restored")
}

func waitForHealth(ctx context.Context, url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	for attempt := 0; attempt < 30; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			response, requestErr := client.Do(request)
			if requestErr == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
				response.Body.Close()
				if response.StatusCode >= 200 && response.StatusCode < 300 {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("service health check timed out")
}

func backupSub2API(ctx context.Context, version string) error {
	postgresUser, err := user.Lookup("postgres")
	if err != nil {
		return fmt.Errorf("failed to find postgres user: %w", err)
	}
	uid, _ := strconv.Atoi(postgresUser.Uid)
	gid, _ := strconv.Atoi(postgresUser.Gid)
	backupRoot := "/var/backups/qapi-components"
	if err := os.MkdirAll(backupRoot, 0o710); err != nil {
		return err
	}
	if err := os.Chown(backupRoot, 0, gid); err != nil {
		return err
	}
	if err := os.Chmod(backupRoot, 0o710); err != nil {
		return err
	}
	backupDir := filepath.Join(backupRoot, "sub2api")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	if err := os.Chown(backupDir, uid, gid); err != nil {
		return err
	}
	if err := os.Chmod(backupDir, 0o700); err != nil {
		return err
	}
	backupPath := filepath.Join(backupDir, time.Now().Format("20060102-150405")+"-before-"+version+".dump")
	if err := runCommand(ctx, "runuser", "-u", "postgres", "--", "pg_dump", "--format=custom", "--file="+backupPath, "sub2api"); err != nil {
		return fmt.Errorf("Sub2API database backup failed: %w", err)
	}
	return nil
}

func validatedLinkTarget(link string, releasesDir string) (string, error) {
	target, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", err
	}
	releasesDir, err = filepath.EvalSymlinks(releasesDir)
	if err != nil {
		return "", err
	}
	target = filepath.Clean(target)
	releasesDir = filepath.Clean(releasesDir)
	if filepath.Dir(target) != releasesDir {
		return "", errors.New("release link points outside the managed releases directory")
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return "", errors.New("release target is not a directory")
	}
	return target, nil
}

func atomicSymlink(target string, link string) error {
	tempLink := link + ".new-" + strconv.Itoa(os.Getpid())
	_ = os.Remove(tempLink)
	if err := os.Symlink(target, tempLink); err != nil {
		return err
	}
	if err := os.Rename(tempLink, link); err != nil {
		_ = os.Remove(tempLink)
		return err
	}
	return nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
