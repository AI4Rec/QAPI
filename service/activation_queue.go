package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	activationReconcileInterval = 30 * time.Minute
	activationDispatchInterval  = 30 * time.Second
	activationExecutionTimeout  = 60 * time.Second
	activationWindowGrace       = 5 * time.Second
	activationMaxConcurrency    = 2
	activationWarmupModel       = "gpt-5.4"
	activationQueuePausedOption = "ActivationQueuePaused"
)

type ActivationReconcileSummary struct {
	Discovered int      `json:"discovered"`
	Scheduled  int      `json:"scheduled"`
	Errors     []string `json:"errors"`
}

type ActivationQueueOverview struct {
	Paused          bool                        `json:"paused"`
	LastReconcileAt int64                       `json:"last_reconcile_at"`
	Targets         []*model.ActivationTarget   `json:"targets"`
	Jobs            []model.ActivationQueueItem `json:"jobs"`
}

type activationSnapshot struct {
	TargetKey     string
	SourceType    string
	SourceID      string
	DisplayName   string
	PlanType      string
	Available     bool
	WindowSeconds int64
	ResetAt       int64
	Metadata      map[string]any
}

type codexCredential struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
}

var (
	activationQueueOnce      sync.Once
	activationReconcileLock  atomic.Bool
	activationLastReconciled atomic.Int64
	activationDispatchWake   = make(chan struct{}, 1)
	activationSlots          = make(chan struct{}, activationMaxConcurrency)
)

func StartActivationQueue() {
	activationQueueOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"activation queue started: reconcile=%s dispatch=%s",
				activationReconcileInterval,
				activationDispatchInterval,
			))
			reconcileTicker := time.NewTicker(activationReconcileInterval)
			dispatchTicker := time.NewTicker(activationDispatchInterval)
			defer reconcileTicker.Stop()
			defer dispatchTicker.Stop()

			_, _ = ReconcileActivationQueue(context.Background())
			dispatchActivationJobs()
			for {
				select {
				case <-reconcileTicker.C:
					_, _ = ReconcileActivationQueue(context.Background())
				case <-dispatchTicker.C:
					dispatchActivationJobs()
				case <-activationDispatchWake:
					dispatchActivationJobs()
				}
			}
		})
	})
}

func IsActivationQueuePaused() bool {
	common.OptionMapRWMutex.RLock()
	value := common.OptionMap[activationQueuePausedOption]
	common.OptionMapRWMutex.RUnlock()
	paused, _ := strconv.ParseBool(value)
	return paused
}

func SetActivationQueuePaused(paused bool) error {
	if err := model.UpdateOption(activationQueuePausedOption, strconv.FormatBool(paused)); err != nil {
		return err
	}
	if !paused {
		notifyActivationDispatcher()
	}
	return nil
}

func GetActivationQueueOverview(limit int) (*ActivationQueueOverview, error) {
	targets, err := model.ListActivationTargets()
	if err != nil {
		return nil, err
	}
	jobs, err := model.ListActivationQueue(limit)
	if err != nil {
		return nil, err
	}
	return &ActivationQueueOverview{
		Paused:          IsActivationQueuePaused(),
		LastReconcileAt: activationLastReconciled.Load(),
		Targets:         targets,
		Jobs:            jobs,
	}, nil
}

func SetActivationTargetEnabled(id int64, enabled bool) error {
	if err := model.SetActivationTargetEnabled(id, enabled); err != nil {
		return err
	}
	if enabled {
		_, _ = ReconcileActivationQueue(context.Background())
	}
	return nil
}

func RunActivationJobNow(id int64) error {
	if err := model.RunActivationJobNow(id); err != nil {
		return err
	}
	notifyActivationDispatcher()
	return nil
}

func SnoozeActivationJob(id int64, duration time.Duration) error {
	if duration < time.Minute {
		duration = time.Minute
	}
	return model.SnoozeActivationJob(id, time.Now().Add(duration).Unix())
}

func SkipActivationJob(id int64) error {
	return model.SkipActivationJob(id)
}

func ReconcileActivationQueue(ctx context.Context) (ActivationReconcileSummary, error) {
	summary := ActivationReconcileSummary{Errors: make([]string, 0)}
	if !activationReconcileLock.CompareAndSwap(false, true) {
		return summary, errors.New("activation queue reconciliation is already running")
	}
	defer activationReconcileLock.Store(false)

	cpaSnapshots, cpaErr := discoverCPAActivationTargets(ctx)
	if cpaErr != nil {
		summary.Errors = append(summary.Errors, "CPA: "+cpaErr.Error())
	} else {
		discovered, scheduled, err := reconcileActivationSnapshots(cpaSnapshots, model.ActivationSourceCPAAccount)
		summary.Discovered += discovered
		summary.Scheduled += scheduled
		if err != nil {
			summary.Errors = append(summary.Errors, "CPA reconcile: "+err.Error())
		}
	}

	channelSnapshots, channelErr := discoverCodexChannelActivationTargets(ctx)
	if channelErr != nil {
		summary.Errors = append(summary.Errors, "Codex channels: "+channelErr.Error())
	} else {
		discovered, scheduled, err := reconcileActivationSnapshots(channelSnapshots, model.ActivationSourceCodexChannel)
		summary.Discovered += discovered
		summary.Scheduled += scheduled
		if err != nil {
			summary.Errors = append(summary.Errors, "Codex channel reconcile: "+err.Error())
		}
	}

	activationLastReconciled.Store(common.GetTimestamp())
	notifyActivationDispatcher()
	if len(summary.Errors) > 0 {
		return summary, errors.New(strings.Join(summary.Errors, "; "))
	}
	return summary, nil
}

func reconcileActivationSnapshots(snapshots []activationSnapshot, sourceType string) (int, int, error) {
	seen := make([]string, 0, len(snapshots))
	scheduledCount := 0
	for _, snapshot := range snapshots {
		seen = append(seen, snapshot.TargetKey)
		metadata, err := common.Marshal(snapshot.Metadata)
		if err != nil {
			return len(seen), scheduledCount, err
		}
		target := &model.ActivationTarget{
			TargetKey:     snapshot.TargetKey,
			SourceType:    snapshot.SourceType,
			SourceID:      snapshot.SourceID,
			DisplayName:   snapshot.DisplayName,
			PlanType:      snapshot.PlanType,
			Enabled:       true,
			AutoManaged:   true,
			Available:     snapshot.Available,
			WindowSeconds: snapshot.WindowSeconds,
			ResetAt:       snapshot.ResetAt,
			LastSeenAt:    common.GetTimestamp(),
			Metadata:      string(metadata),
		}
		if err := model.UpsertActivationTarget(target); err != nil {
			return len(seen), scheduledCount, err
		}
		stored, err := model.GetActivationTargetByKey(snapshot.TargetKey)
		if err != nil || stored == nil {
			if err == nil {
				err = errors.New("activation target not found after upsert")
			}
			return len(seen), scheduledCount, err
		}
		if !stored.Enabled || !snapshot.Available || snapshot.WindowSeconds <= 0 {
			_ = model.CancelPendingActivationJobsForTarget(stored.ID, "")
			continue
		}
		scheduledAt := time.Now().Add(activationWindowGrace).Unix()
		if snapshot.ResetAt > 0 {
			scheduledAt = snapshot.ResetAt + int64(activationWindowGrace.Seconds())
		}
		job, created, err := model.ScheduleActivationJob(
			stored.ID,
			stored.TargetKey,
			snapshot.ResetAt,
			scheduledAt,
			"automatic",
		)
		if err != nil {
			return len(seen), scheduledCount, err
		}
		if created {
			scheduledCount++
		}
		_ = model.CancelPendingActivationJobsForTarget(stored.ID, job.DedupeKey)
	}
	if err := model.MarkMissingActivationTargetsUnavailable(sourceType, seen); err != nil {
		return len(seen), scheduledCount, err
	}
	return len(seen), scheduledCount, nil
}

func dispatchActivationJobs() {
	if IsActivationQueuePaused() {
		return
	}
	_ = model.ResetStaleActivationJobs(time.Now().Add(-10 * time.Minute).Unix())
	for len(activationSlots) < cap(activationSlots) {
		job, claimed, err := model.ClaimNextDueActivationJob(common.GetTimestamp())
		if err != nil {
			logger.LogWarn(context.Background(), "activation queue claim failed: "+err.Error())
			return
		}
		if !claimed || job == nil {
			return
		}
		activationSlots <- struct{}{}
		claimedJob := job
		gopool.Go(func() {
			defer func() { <-activationSlots }()
			executeActivationJob(claimedJob)
			notifyActivationDispatcher()
		})
	}
}

func executeActivationJob(job *model.ActivationJob) {
	ctx, cancel := context.WithTimeout(context.Background(), activationExecutionTimeout)
	defer cancel()
	target, err := model.GetActivationTarget(job.TargetID)
	if err != nil || target == nil {
		finishActivationFailure(job, fmt.Errorf("activation target unavailable: %w", err))
		return
	}
	if !target.Enabled || !target.Available {
		_ = model.FinishActivationJob(job.ID, model.ActivationJobStatusSkipped, "target disabled or unavailable", "", 0)
		return
	}

	snapshot, err := getActivationSnapshot(ctx, target)
	if err != nil {
		finishActivationFailure(job, err)
		return
	}
	now := common.GetTimestamp()
	manual := job.Trigger == "manual"
	if !manual && snapshot.ResetAt > now+int64(activationWindowGrace.Seconds()) {
		if snapshot.ResetAt > job.PreviousResetAt+60 {
			message := "window already activated by normal traffic"
			_ = model.FinishActivationJob(job.ID, model.ActivationJobStatusSkipped, message, "", snapshot.ResetAt)
			_ = model.UpdateActivationTargetResult(target.ID, "traffic_activated", "", snapshot.ResetAt, 0)
			_, _, _ = reconcileActivationSnapshots([]activationSnapshot{*snapshot}, target.SourceType)
			return
		}
		_ = model.RetryActivationJob(job.ID, snapshot.ResetAt+int64(activationWindowGrace.Seconds()), "window is not due yet")
		return
	}

	if err := activateTarget(ctx, target); err != nil {
		finishActivationFailure(job, err)
		return
	}

	time.Sleep(3 * time.Second)
	verified, verifyErr := getActivationSnapshot(ctx, target)
	if verifyErr != nil {
		finishActivationFailure(job, fmt.Errorf("warm-up sent but verification failed: %w", verifyErr))
		return
	}
	if !manual && verified.ResetAt <= snapshot.ResetAt {
		finishActivationFailure(job, errors.New("warm-up did not advance the activation window"))
		return
	}

	resultText := "activation ping succeeded"
	_ = model.FinishActivationJob(job.ID, model.ActivationJobStatusSucceeded, resultText, "", verified.ResetAt)
	_ = model.UpdateActivationTargetResult(target.ID, "succeeded", "", verified.ResetAt, common.GetTimestamp())
	_, _, _ = reconcileActivationSnapshots([]activationSnapshot{*verified}, target.SourceType)
}

func finishActivationFailure(job *model.ActivationJob, runErr error) {
	if runErr == nil {
		runErr = errors.New("activation failed")
	}
	if job.Attempt < job.MaxAttempts {
		delays := []time.Duration{time.Minute, 3 * time.Minute, 10 * time.Minute}
		index := job.Attempt - 1
		if index < 0 {
			index = 0
		}
		if index >= len(delays) {
			index = len(delays) - 1
		}
		_ = model.RetryActivationJob(job.ID, time.Now().Add(delays[index]).Unix(), runErr.Error())
		return
	}
	_ = model.FinishActivationJob(job.ID, model.ActivationJobStatusFailed, "", runErr.Error(), 0)
	_ = model.UpdateActivationTargetResult(job.TargetID, "failed", runErr.Error(), job.PreviousResetAt, 0)
}

func getActivationSnapshot(ctx context.Context, target *model.ActivationTarget) (*activationSnapshot, error) {
	switch target.SourceType {
	case model.ActivationSourceCPAAccount:
		return getCPAActivationSnapshot(ctx, target.SourceID)
	case model.ActivationSourceCodexChannel:
		channelID, err := strconv.Atoi(target.SourceID)
		if err != nil {
			return nil, err
		}
		return getCodexChannelActivationSnapshot(ctx, channelID)
	default:
		return nil, fmt.Errorf("unsupported activation source: %s", target.SourceType)
	}
}

func activateTarget(ctx context.Context, target *model.ActivationTarget) error {
	switch target.SourceType {
	case model.ActivationSourceCPAAccount:
		return activateCPAAccount(ctx, target.SourceID)
	case model.ActivationSourceCodexChannel:
		channelID, err := strconv.Atoi(target.SourceID)
		if err != nil {
			return err
		}
		return activateCodexChannel(ctx, channelID)
	default:
		return fmt.Errorf("unsupported activation source: %s", target.SourceType)
	}
}

func discoverCPAActivationTargets(ctx context.Context) ([]activationSnapshot, error) {
	files, err := listCPAAuthFiles(ctx)
	if err != nil {
		return nil, err
	}
	snapshots := make([]activationSnapshot, 0, len(files))
	for _, file := range files {
		if !strings.EqualFold(mapString(file, "type"), "codex") {
			continue
		}
		authIndex := mapString(file, "auth_index")
		if authIndex == "" {
			continue
		}
		archived, archiveErr := model.IsOperationalAssetArchived(model.OperationalAssetTypeCPAAccount, model.OperationalCPAAssetKey(authIndex))
		if archiveErr != nil {
			return nil, archiveErr
		}
		if archived {
			continue
		}
		snapshot, err := cpaSnapshotFromFile(ctx, file)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("activation queue CPA discovery skipped account: %v", err))
			continue
		}
		snapshots = append(snapshots, *snapshot)
	}
	return snapshots, nil
}

func getCPAActivationSnapshot(ctx context.Context, authIndex string) (*activationSnapshot, error) {
	files, err := listCPAAuthFiles(ctx)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if mapString(file, "auth_index") == authIndex {
			archived, archiveErr := model.IsOperationalAssetArchived(model.OperationalAssetTypeCPAAccount, model.OperationalCPAAssetKey(authIndex))
			if archiveErr != nil {
				return nil, archiveErr
			}
			if archived {
				return nil, errors.New("CPA account is archived")
			}
			return cpaSnapshotFromFile(ctx, file)
		}
	}
	return nil, errors.New("CPA account not found")
}

func cpaSnapshotFromFile(ctx context.Context, file map[string]any) (*activationSnapshot, error) {
	authIndex := mapString(file, "auth_index")
	idToken := mapObject(file["id_token"])
	accountID := mapString(idToken, "chatgpt_account_id")
	if accountID == "" {
		return nil, errors.New("CPA account id is missing")
	}
	usage, err := fetchCPAUsage(ctx, authIndex, accountID)
	if err != nil {
		return nil, err
	}
	windowSeconds, resetAt := shortActivationWindow(usage)
	email := mapString(file, "email")
	if email == "" {
		email = mapString(file, "name")
	}
	hash := sha256.Sum256([]byte(authIndex))
	return &activationSnapshot{
		TargetKey:     "cpa:" + hex.EncodeToString(hash[:8]),
		SourceType:    model.ActivationSourceCPAAccount,
		SourceID:      authIndex,
		DisplayName:   email,
		PlanType:      usagePlanType(usage),
		Available:     !mapBool(file, "disabled") && !mapBool(file, "unavailable"),
		WindowSeconds: windowSeconds,
		ResetAt:       resetAt,
		Metadata: map[string]any{
			"file_name": mapString(file, "name"),
			"model":     activationWarmupModel,
		},
	}, nil
}

func discoverCodexChannelActivationTargets(ctx context.Context) ([]activationSnapshot, error) {
	var channels []*model.Channel
	err := model.ExcludeArchivedChannels(model.DB.Where("type = ? AND status = ?", constant.ChannelTypeCodex, common.ChannelStatusEnabled)).
		Order("id asc").Find(&channels).Error
	if err != nil {
		return nil, err
	}
	snapshots := make([]activationSnapshot, 0, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.ChannelInfo.IsMultiKey {
			continue
		}
		snapshot, err := codexChannelSnapshot(ctx, channel)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("activation queue Codex discovery skipped channel %d: %v", channel.Id, err))
			continue
		}
		snapshots = append(snapshots, *snapshot)
	}
	return snapshots, nil
}

func getCodexChannelActivationSnapshot(ctx context.Context, channelID int) (*activationSnapshot, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil || channel == nil {
		return nil, errors.New("Codex channel not found")
	}
	return codexChannelSnapshot(ctx, channel)
}

func codexChannelSnapshot(ctx context.Context, channel *model.Channel) (*activationSnapshot, error) {
	credential, err := parseActivationCodexCredential(channel.Key)
	if err != nil {
		return nil, err
	}
	client, err := NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	status, body, err := FetchCodexWhamUsage(ctx, client, channel.GetBaseURL(), credential.AccessToken, credential.AccountID)
	if err != nil || status < 200 || status >= 300 {
		return nil, fmt.Errorf("Codex usage query failed with status %d", status)
	}
	usage := map[string]any{}
	if err := common.Unmarshal(body, &usage); err != nil {
		return nil, err
	}
	windowSeconds, resetAt := shortActivationWindow(usage)
	modelName := firstChannelModel(channel)
	return &activationSnapshot{
		TargetKey:     fmt.Sprintf("channel:%d", channel.Id),
		SourceType:    model.ActivationSourceCodexChannel,
		SourceID:      strconv.Itoa(channel.Id),
		DisplayName:   channel.Name,
		PlanType:      usagePlanType(usage),
		Available:     channel.Status == common.ChannelStatusEnabled,
		WindowSeconds: windowSeconds,
		ResetAt:       resetAt,
		Metadata:      map[string]any{"model": modelName},
	}, nil
}

func parseActivationCodexCredential(raw string) (*codexCredential, error) {
	credential := &codexCredential{}
	if err := common.UnmarshalJsonStr(strings.TrimSpace(raw), credential); err != nil {
		return nil, err
	}
	credential.AccessToken = strings.TrimSpace(credential.AccessToken)
	credential.AccountID = strings.TrimSpace(credential.AccountID)
	if credential.AccessToken == "" || credential.AccountID == "" {
		return nil, errors.New("Codex credential is incomplete")
	}
	return credential, nil
}

func firstChannelModel(channel *model.Channel) string {
	if channel != nil && channel.TestModel != nil {
		if value := strings.TrimSpace(*channel.TestModel); value != "" {
			return value
		}
	}
	if channel != nil {
		for _, value := range strings.Split(channel.Models, ",") {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return activationWarmupModel
}

func activateCodexChannel(ctx context.Context, channelID int) error {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil || channel == nil {
		return errors.New("Codex channel not found")
	}
	credential, err := parseActivationCodexCredential(channel.Key)
	if err != nil {
		return err
	}
	client, err := NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return err
	}
	body, err := activationRequestBody(firstChannelModel(channel))
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(channel.GetBaseURL(), "/") + "/backend-api/codex/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	setActivationCodexHeaders(req, credential.AccessToken, credential.AccountID)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Codex activation returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func activateCPAAccount(ctx context.Context, authIndex string) error {
	files, err := listCPAAuthFiles(ctx)
	if err != nil {
		return err
	}
	var accountID string
	for _, file := range files {
		if mapString(file, "auth_index") == authIndex {
			accountID = mapString(mapObject(file["id_token"]), "chatgpt_account_id")
			break
		}
	}
	if accountID == "" {
		return errors.New("CPA account not found")
	}
	requestBody, err := activationRequestBody(activationWarmupModel)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"auth_index": authIndex,
		"method":     http.MethodPost,
		"url":        "https://chatgpt.com/backend-api/codex/responses",
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"chatgpt-account-id": accountID,
			"Content-Type":       "application/json",
			"Accept":             "text/event-stream",
			"OpenAI-Beta":        "responses=experimental",
			"originator":         "codex_cli_rs",
		},
		"data": string(requestBody),
	}
	outer, err := doCPAManagementRequest(ctx, http.MethodPost, "/v0/management/api-call", payload)
	if err != nil {
		return err
	}
	status := int(mapInt64(outer, "status_code"))
	if status < 200 || status >= 300 {
		return fmt.Errorf("CPA activation returned upstream HTTP %d", status)
	}
	return nil
}

func activationRequestBody(modelName string) ([]byte, error) {
	if strings.TrimSpace(modelName) == "" {
		modelName = activationWarmupModel
	}
	return common.Marshal(map[string]any{
		"model":        modelName,
		"instructions": "Reply with exactly OK.",
		"input": []map[string]any{{
			"role":    "user",
			"content": []map[string]string{{"type": "input_text", "text": "OK"}},
		}},
		"reasoning": map[string]string{"effort": "low", "summary": "auto"},
		"store":     false,
		"stream":    true,
	})
}

func setActivationCodexHeaders(req *http.Request, accessToken string, accountID string) {
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("chatgpt-account-id", accountID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", "codex_cli_rs")
}

func listCPAAuthFiles(ctx context.Context) ([]map[string]any, error) {
	response, err := doCPAManagementRequest(ctx, http.MethodGet, "/v0/management/auth-files", nil)
	if err != nil {
		return nil, err
	}
	rawFiles, _ := response["files"].([]any)
	files := make([]map[string]any, 0, len(rawFiles))
	for _, raw := range rawFiles {
		if file := mapObject(raw); len(file) > 0 {
			files = append(files, file)
		}
	}
	return files, nil
}

func fetchCPAUsage(ctx context.Context, authIndex string, accountID string) (map[string]any, error) {
	payload := map[string]any{
		"auth_index": authIndex,
		"method":     http.MethodGet,
		"url":        "https://chatgpt.com/backend-api/wham/usage",
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"chatgpt-account-id": accountID,
			"Accept":             "application/json",
			"originator":         "codex_cli_rs",
		},
	}
	outer, err := doCPAManagementRequest(ctx, http.MethodPost, "/v0/management/api-call", payload)
	if err != nil {
		return nil, err
	}
	status := int(mapInt64(outer, "status_code"))
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("CPA usage returned upstream HTTP %d", status)
	}
	usage := map[string]any{}
	if err := common.UnmarshalJsonStr(mapString(outer, "body"), &usage); err != nil {
		return nil, err
	}
	return usage, nil
}

func doCPAManagementRequest(ctx context.Context, method string, path string, payload any) (map[string]any, error) {
	keyPath := strings.TrimSpace(os.Getenv("CPA_MANAGEMENT_KEY_FILE"))
	if keyPath == "" {
		keyPath = "/etc/new-api/cpa-management-key"
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:8317"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(keyBytes)))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CPA management returned HTTP %d", resp.StatusCode)
	}
	result := map[string]any{}
	if err := common.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func shortActivationWindow(usage map[string]any) (int64, int64) {
	rateLimit := mapObject(usage["rate_limit"])
	var selectedSeconds int64
	var selectedResetAt int64
	for _, key := range []string{"primary_window", "secondary_window"} {
		window := mapObject(rateLimit[key])
		seconds := mapInt64(window, "limit_window_seconds")
		if seconds <= 0 || seconds >= int64((24*time.Hour).Seconds()) {
			continue
		}
		if selectedSeconds == 0 || seconds < selectedSeconds {
			selectedSeconds = seconds
			selectedResetAt = mapInt64(window, "reset_at")
		}
	}
	return selectedSeconds, selectedResetAt
}

func usagePlanType(usage map[string]any) string {
	if value := mapString(usage, "plan_type"); value != "" {
		return value
	}
	return mapString(mapObject(usage["rate_limit"]), "plan_type")
}

func mapObject(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func mapString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func mapBool(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func mapInt64(values map[string]any, key string) int64 {
	switch value := values[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case string:
		result, _ := strconv.ParseInt(value, 10, 64)
		return result
	default:
		return 0
	}
}

func notifyActivationDispatcher() {
	select {
	case activationDispatchWake <- struct{}{}:
	default:
	}
}
