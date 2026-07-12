package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ActivationJobStatus string

const (
	ActivationJobStatusPending   ActivationJobStatus = "pending"
	ActivationJobStatusRunning   ActivationJobStatus = "running"
	ActivationJobStatusSucceeded ActivationJobStatus = "succeeded"
	ActivationJobStatusFailed    ActivationJobStatus = "failed"
	ActivationJobStatusSkipped   ActivationJobStatus = "skipped"
	ActivationJobStatusCancelled ActivationJobStatus = "cancelled"
)

const (
	ActivationSourceCPAAccount   = "cpa_account"
	ActivationSourceCodexChannel = "codex_channel"
)

type ActivationTarget struct {
	ID              int64  `json:"id" gorm:"primaryKey"`
	TargetKey       string `json:"target_key" gorm:"type:varchar(191);uniqueIndex"`
	SourceType      string `json:"source_type" gorm:"type:varchar(32);index"`
	SourceID        string `json:"source_id" gorm:"type:varchar(191);index"`
	DisplayName     string `json:"display_name" gorm:"type:varchar(255)"`
	PlanType        string `json:"plan_type" gorm:"type:varchar(64);index"`
	Enabled         bool   `json:"enabled"`
	AutoManaged     bool   `json:"auto_managed"`
	Available       bool   `json:"available"`
	WindowSeconds   int64  `json:"window_seconds" gorm:"bigint"`
	ResetAt         int64  `json:"reset_at" gorm:"bigint;index"`
	LastSeenAt      int64  `json:"last_seen_at" gorm:"bigint;index"`
	LastActivatedAt int64  `json:"last_activated_at" gorm:"bigint"`
	LastStatus      string `json:"last_status" gorm:"type:varchar(32)"`
	LastError       string `json:"last_error" gorm:"type:text"`
	Metadata        string `json:"metadata" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;index"`
}

type ActivationJob struct {
	ID              int64               `json:"id" gorm:"primaryKey"`
	JobID           string              `json:"job_id" gorm:"type:varchar(64);uniqueIndex"`
	TargetID        int64               `json:"target_id" gorm:"index"`
	DedupeKey       string              `json:"dedupe_key" gorm:"type:varchar(255);uniqueIndex"`
	ScheduledAt     int64               `json:"scheduled_at" gorm:"bigint;index"`
	Status          ActivationJobStatus `json:"status" gorm:"type:varchar(32);index"`
	Trigger         string              `json:"trigger" gorm:"type:varchar(32)"`
	Attempt         int                 `json:"attempt"`
	MaxAttempts     int                 `json:"max_attempts"`
	StartedAt       int64               `json:"started_at" gorm:"bigint"`
	FinishedAt      int64               `json:"finished_at" gorm:"bigint"`
	PreviousResetAt int64               `json:"previous_reset_at" gorm:"bigint"`
	NewResetAt      int64               `json:"new_reset_at" gorm:"bigint"`
	Result          string              `json:"result" gorm:"type:text"`
	Error           string              `json:"error" gorm:"type:text"`
	CreatedAt       int64               `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64               `json:"updated_at" gorm:"bigint;index"`
}

type ActivationQueueItem struct {
	ActivationJob
	Target ActivationTarget `json:"target"`
}

func (target *ActivationTarget) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if target.CreatedAt == 0 {
		target.CreatedAt = now
	}
	if target.UpdatedAt == 0 {
		target.UpdatedAt = now
	}
	return nil
}

func (job *ActivationJob) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if job.CreatedAt == 0 {
		job.CreatedAt = now
	}
	if job.UpdatedAt == 0 {
		job.UpdatedAt = now
	}
	return nil
}

func UpsertActivationTarget(target *ActivationTarget) error {
	if target == nil || target.TargetKey == "" {
		return errors.New("activation target key is required")
	}
	target.UpdatedAt = common.GetTimestamp()
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "target_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_type", "source_id", "display_name", "plan_type", "available",
			"window_seconds", "reset_at", "last_seen_at", "metadata", "updated_at",
		}),
	}).Create(target).Error
}

func GetActivationTargetByKey(targetKey string) (*ActivationTarget, error) {
	var target ActivationTarget
	if err := DB.Where("target_key = ?", targetKey).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &target, nil
}

func GetActivationTarget(id int64) (*ActivationTarget, error) {
	var target ActivationTarget
	if err := DB.First(&target, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &target, nil
}

func ListActivationTargets() ([]*ActivationTarget, error) {
	var targets []*ActivationTarget
	err := DB.Order("enabled desc, reset_at asc, id asc").Find(&targets).Error
	return targets, err
}

func MarkMissingActivationTargetsUnavailable(sourceType string, seenTargetKeys []string) error {
	query := DB.Model(&ActivationTarget{}).Where("source_type = ?", sourceType)
	if len(seenTargetKeys) > 0 {
		query = query.Where("target_key NOT IN ?", seenTargetKeys)
	}
	return query.Updates(map[string]any{
		"available":   false,
		"last_status": "missing",
		"updated_at":  common.GetTimestamp(),
	}).Error
}

func SetActivationTargetEnabled(id int64, enabled bool) error {
	return DB.Model(&ActivationTarget{}).Where("id = ?", id).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": common.GetTimestamp(),
	}).Error
}

func UpdateActivationTargetResult(id int64, status string, errorMessage string, resetAt int64, activatedAt int64) error {
	updates := map[string]any{
		"last_status": status,
		"last_error":  errorMessage,
		"reset_at":    resetAt,
		"updated_at":  common.GetTimestamp(),
	}
	if activatedAt > 0 {
		updates["last_activated_at"] = activatedAt
	}
	return DB.Model(&ActivationTarget{}).Where("id = ?", id).Updates(updates).Error
}

func ScheduleActivationJob(targetID int64, targetKey string, resetAt int64, scheduledAt int64, trigger string) (*ActivationJob, bool, error) {
	if targetID <= 0 || targetKey == "" {
		return nil, false, errors.New("activation target is required")
	}
	if scheduledAt <= 0 {
		scheduledAt = common.GetTimestamp()
	}
	dedupeKey := fmt.Sprintf("%s:%d", targetKey, resetAt)
	jobID, err := GenerateSystemTaskID()
	if err != nil {
		return nil, false, err
	}
	job := &ActivationJob{
		JobID:           "activation_" + jobID,
		TargetID:        targetID,
		DedupeKey:       dedupeKey,
		ScheduledAt:     scheduledAt,
		Status:          ActivationJobStatusPending,
		Trigger:         trigger,
		MaxAttempts:     4,
		PreviousResetAt: resetAt,
	}
	if err := DB.Create(job).Error; err != nil {
		var existing ActivationJob
		if findErr := DB.Where("dedupe_key = ?", dedupeKey).First(&existing).Error; findErr == nil {
			return &existing, false, nil
		}
		return nil, false, err
	}
	return job, true, nil
}

func ListActivationQueue(limit int) ([]ActivationQueueItem, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 300 {
		limit = 300
	}
	var jobs []ActivationJob
	err := DB.Where("status IN ?", []ActivationJobStatus{
		ActivationJobStatusPending,
		ActivationJobStatusRunning,
		ActivationJobStatusFailed,
		ActivationJobStatusSucceeded,
		ActivationJobStatusSkipped,
	}).Order("CASE WHEN status IN ('pending','running') THEN 0 ELSE 1 END, scheduled_at asc, id desc").Limit(limit).Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	targetIDs := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		targetIDs = append(targetIDs, job.TargetID)
	}
	var targets []ActivationTarget
	if len(targetIDs) > 0 {
		if err := DB.Where("id IN ?", targetIDs).Find(&targets).Error; err != nil {
			return nil, err
		}
	}
	targetByID := make(map[int64]ActivationTarget, len(targets))
	for _, target := range targets {
		targetByID[target.ID] = target
	}
	items := make([]ActivationQueueItem, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, ActivationQueueItem{ActivationJob: job, Target: targetByID[job.TargetID]})
	}
	return items, nil
}

func ClaimNextDueActivationJob(now int64) (*ActivationJob, bool, error) {
	var job ActivationJob
	err := DB.Where("status = ? AND scheduled_at <= ?", ActivationJobStatusPending, now).
		Order("scheduled_at asc, id asc").First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	result := DB.Model(&ActivationJob{}).
		Where("id = ? AND status = ?", job.ID, ActivationJobStatusPending).
		Updates(map[string]any{
			"status":     ActivationJobStatusRunning,
			"started_at": now,
			"attempt":    gorm.Expr("attempt + 1"),
			"updated_at": now,
		})
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, false, result.Error
	}
	if err := DB.First(&job, job.ID).Error; err != nil {
		return nil, false, err
	}
	return &job, true, nil
}

func ResetStaleActivationJobs(staleBefore int64) error {
	return DB.Model(&ActivationJob{}).
		Where("status = ? AND started_at > 0 AND started_at < ?", ActivationJobStatusRunning, staleBefore).
		Updates(map[string]any{
			"status":       ActivationJobStatusPending,
			"scheduled_at": common.GetTimestamp(),
			"error":        "stale execution recovered",
			"started_at":   0,
			"updated_at":   common.GetTimestamp(),
		}).Error
}

func FinishActivationJob(id int64, status ActivationJobStatus, resultText string, errorMessage string, newResetAt int64) error {
	now := common.GetTimestamp()
	return DB.Model(&ActivationJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":       status,
		"result":       resultText,
		"error":        errorMessage,
		"new_reset_at": newResetAt,
		"finished_at":  now,
		"updated_at":   now,
	}).Error
}

func RetryActivationJob(id int64, scheduledAt int64, errorMessage string) error {
	return DB.Model(&ActivationJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":       ActivationJobStatusPending,
		"scheduled_at": scheduledAt,
		"error":        errorMessage,
		"started_at":   0,
		"updated_at":   common.GetTimestamp(),
	}).Error
}

func RunActivationJobNow(id int64) error {
	return DB.Model(&ActivationJob{}).
		Where("id = ? AND status IN ?", id, []ActivationJobStatus{ActivationJobStatusPending, ActivationJobStatusFailed}).
		Updates(map[string]any{
			"status":       ActivationJobStatusPending,
			"scheduled_at": common.GetTimestamp(),
			"trigger":      "manual",
			"error":        "",
			"finished_at":  0,
			"updated_at":   common.GetTimestamp(),
		}).Error
}

func SnoozeActivationJob(id int64, scheduledAt int64) error {
	return DB.Model(&ActivationJob{}).
		Where("id = ? AND status = ?", id, ActivationJobStatusPending).
		Updates(map[string]any{"scheduled_at": scheduledAt, "updated_at": common.GetTimestamp()}).Error
}

func SkipActivationJob(id int64) error {
	return FinishActivationJob(id, ActivationJobStatusSkipped, "manually skipped", "", 0)
}

func CancelPendingActivationJobsForTarget(targetID int64, exceptDedupeKey string) error {
	query := DB.Model(&ActivationJob{}).Where("target_id = ? AND status = ?", targetID, ActivationJobStatusPending)
	if exceptDedupeKey != "" {
		query = query.Where("dedupe_key <> ?", exceptDedupeKey)
	}
	return query.Updates(map[string]any{
		"status":      ActivationJobStatusCancelled,
		"finished_at": common.GetTimestamp(),
		"updated_at":  common.GetTimestamp(),
	}).Error
}

func DisableActivationSource(sourceType string, sourceID string) error {
	var targets []ActivationTarget
	if err := DB.Where("source_type = ? AND source_id = ?", sourceType, sourceID).Find(&targets).Error; err != nil {
		return err
	}
	for _, target := range targets {
		if err := DB.Model(&ActivationTarget{}).Where("id = ?", target.ID).Updates(map[string]any{
			"enabled":     false,
			"available":   false,
			"last_status": "archived",
			"updated_at":  common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if err := CancelPendingActivationJobsForTarget(target.ID, ""); err != nil {
			return err
		}
	}
	return nil
}
