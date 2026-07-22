package controller

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const defaultCodexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

var cpaFileNameCleaner = regexp.MustCompile(`[^a-zA-Z0-9._+-]+`)
var cpaImportedAccountNamePattern = regexp.MustCompile(`-[0-9a-f]{8}\.json$`)
var cpaTemporaryAccountNamePattern = regexp.MustCompile(`^codex-temp-`)

var cpaAuthFilesCache = struct {
	sync.RWMutex
	refresh   sync.Mutex
	files     []map[string]any
	expiresAt time.Time
}{}

type cpaImportRequest struct {
	Content  string `json:"content"`
	PoolType string `json:"pool_type"`
}

type cpaAccountCandidate struct {
	file        map[string]any
	assetKey    string
	uniqueKey   string
	email       string
	sortName    string
	poolType    string
	costMinor   int64
	disabled    bool
	unavailable bool
	success     float64
	failed      float64
	createdAt   int64
	createdOK   bool
	updatedAt   int64
	updatedOK   bool
	metadata    service.CPAAuthMetadata
}

type cpaAccountPoolSnapshot struct {
	candidates   []cpaAccountCandidate
	assetsByKey  map[string]model.OperationalAsset
	uniqueCounts map[string]int
	active       int
	disabled     int
}

type cpaAccountOutputSnapshot struct {
	AccountOutputs []struct {
		AssetKey              string  `json:"asset_key"`
		CumulativeOutputQuota float64 `json:"cumulative_output_quota"`
	} `json:"account_outputs"`
}

const (
	cpaBatchMaxAccounts   = 2000
	cpaBatchActionDisable = "disable"
	cpaBatchActionEnable  = "enable"
	cpaBatchActionMove    = "move"
	cpaBatchActionCost    = "cost"
	cpaBatchActionArchive = "archive"
	cpaBatchActionDelete  = "delete"
)

type cpaAccountBatchRequest struct {
	Names              []string `json:"names"`
	Action             string   `json:"action"`
	PoolType           string   `json:"pool_type"`
	CostMinor          *int64   `json:"cost_minor"`
	CostDate           int64    `json:"cost_date"`
	CostNote           string   `json:"cost_note"`
	ArchiveReason      string   `json:"archive_reason"`
	DeleteConfirmation string   `json:"delete_confirmation"`
}

func ImportCPAAccounts(c *gin.Context) {
	var req cpaImportRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请粘贴账号 JSON"})
		return
	}
	if len(req.Content) > 10<<20 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "内容不能超过 10MB"})
		return
	}
	poolType, ok := normalizeCPAImportPoolType(req.PoolType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号池类型无效"})
		return
	}

	accounts, err := decodeCPAAccounts(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}

	uploaded := make([]string, 0, len(accounts))
	failed := make([]gin.H, 0)
	for i, account := range accounts {
		name, normalized, normalizeErr := normalizeCPAAccount(account, poolType)
		if normalizeErr != nil {
			failed = append(failed, gin.H{"index": i + 1, "error": normalizeErr.Error()})
			continue
		}
		if uploadErr := uploadCPAAccount(c.Request.Context(), managementKey, name, normalized); uploadErr != nil {
			failed = append(failed, gin.H{"index": i + 1, "error": uploadErr.Error()})
			continue
		}
		uploaded = append(uploaded, name)
	}

	verified := make([]gin.H, 0, len(uploaded))
	warnings := make([]gin.H, 0)
	if len(uploaded) > 0 {
		invalidateCPAAuthFilesCache()
		body, status, listErr := doCPARequest(c.Request.Context(), managementKey, http.MethodGet, "/v0/management/auth-files", nil)
		var list struct {
			Files []map[string]any `json:"files"`
		}
		if listErr != nil || status < 200 || status >= 300 || common.Unmarshal(body, &list) != nil {
			warnings = append(warnings, gin.H{"error": "上传成功，但读取 CPA 账号进行验证失败"})
		} else {
			filesByName := make(map[string]map[string]any, len(list.Files))
			for _, file := range list.Files {
				filesByName[stringValue(file["name"])] = file
			}
			for _, name := range uploaded {
				file := filesByName[name]
				if file == nil {
					warnings = append(warnings, gin.H{"name": name, "error": "上传成功，但 CPA 未返回该账号"})
					continue
				}
				metadata, repaired, resolveErr := service.ResolveAndRepairCPAAuthMetadata(c.Request.Context(), file)
				authIndex := stringValue(file["auth_index"])
				if authIndex == "" {
					warnings = append(warnings, gin.H{"name": name, "error": "上传成功，但 CPA 未返回 auth_index"})
					continue
				}
				displayName := stringValue(file["email"])
				if displayName == "" {
					displayName = metadata.Email
				}
				if displayName == "" {
					displayName = name
				}
				if poolErr := model.SetCPAOperationalAssetPool(&model.OperationalAsset{
					SourceKey:   model.OperationalCPAAssetKey(authIndex),
					SourceRef:   authIndex,
					DisplayName: displayName,
					CreatedBy:   c.GetInt("id"),
					UpdatedBy:   c.GetInt("id"),
				}, poolType, c.GetInt("id")); poolErr != nil {
					warnings = append(warnings, gin.H{"name": name, "error": "上传成功，但账号池归属保存失败"})
				}
				if resolveErr != nil || metadata.AccountID == "" {
					warnings = append(warnings, gin.H{"name": name, "error": "上传成功，但无法解析账号订阅信息"})
					continue
				}
				usage, usageErr := fetchCPAAccountUsage(
					c.Request.Context(),
					managementKey,
					stringValue(file["auth_index"]),
					metadata.AccountID,
				)
				if usageErr != nil {
					warnings = append(warnings, gin.H{"name": name, "error": "上传成功，但额度验证失败"})
					continue
				}
				planType := stringValue(usage["plan_type"])
				if planType == "" {
					planType = metadata.PlanType
				}
				verified = append(verified, gin.H{
					"name": name, "email": metadata.Email, "plan_type": planType, "pool_type": poolType,
					"verified": true, "repaired": repaired, "usage_available": true,
				})
			}
		}
	}

	success := len(failed) == 0
	message := fmt.Sprintf("CPA 上货完成：上传 %d，验证 %d，失败 %d", len(uploaded), len(verified), len(failed))
	if len(warnings) > 0 {
		message += fmt.Sprintf("，警告 %d", len(warnings))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": message,
		"data": gin.H{
			"total": len(accounts), "uploaded": len(uploaded), "verified": len(verified),
			"failed": failed, "warnings": warnings, "files": uploaded, "accounts": verified,
		},
	})
}

func GetCPAAccounts(c *gin.Context) {
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	files, err := listCachedCPAAuthFiles(c.Request.Context(), managementKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 返回格式错误"})
		return
	}

	poolTypeFilter, ok := normalizeCPAAccountPoolType(c.Query("pool_type"), true)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号池类型无效"})
		return
	}
	sortBy, ok := normalizeCPAAccountSortBy(c.Query("sort_by"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "排序字段无效"})
		return
	}
	sortOrder, ok := normalizeCPAAccountSortOrder(c.Query("sort_order"), sortBy)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "排序方向无效"})
		return
	}
	pageInfo := common.GetPageQuery(c)
	snapshot, err := buildCPAAccountPoolSnapshot(files, poolTypeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取 CPA 成本资产失败"})
		return
	}
	sortCPAAccountCandidates(snapshot.candidates, sortBy, sortOrder)

	start := pageInfo.GetStartIdx()
	if start > len(snapshot.candidates) {
		start = len(snapshot.candidates)
	}
	end := pageInfo.GetEndIdx()
	if end > len(snapshot.candidates) {
		end = len(snapshot.candidates)
	}
	pageCandidates := snapshot.candidates[start:end]
	outputByAssetKey := readCPAAccountOutputs()
	accounts := make([]map[string]any, 0, len(pageCandidates))
	usageCandidates := map[string]map[string]string{}
	for _, candidate := range pageCandidates {
		file := candidate.file
		accountID := candidate.metadata.AccountID
		isDisabled, _ := file["disabled"].(bool)
		account := map[string]any{
			"name":                  file["name"],
			"email":                 candidate.email,
			"status":                file["status"],
			"status_message":        file["status_message"],
			"disabled":              isDisabled,
			"unavailable":           file["unavailable"],
			"success":               file["success"],
			"failed":                file["failed"],
			"recent_requests":       file["recent_requests"],
			"created_at":            file["created_at"],
			"updated_at":            file["updated_at"],
			"last_refresh":          file["last_refresh"],
			"next_retry_after":      file["next_retry_after"],
			"plan_type":             candidate.metadata.PlanType,
			"account_id":            accountID,
			"asset_key":             candidate.assetKey,
			"cumulative_output_usd": outputByAssetKey[candidate.assetKey],
			"pool_type":             candidate.poolType,
			"metadata_repaired":     false,
			"unique_key":            candidate.uniqueKey,
		}
		if storedAsset, ok := snapshot.assetsByKey[candidate.assetKey]; ok {
			account["cost_minor"] = storedAsset.CostMinor
			account["cost_date"] = storedAsset.CostDate
			account["cost_note"] = storedAsset.CostNote
		}
		accounts = append(accounts, account)
		if accountID != "" && usageCandidates[candidate.uniqueKey] == nil {
			usageCandidates[candidate.uniqueKey] = map[string]string{
				"auth_index": stringValue(file["auth_index"]),
				"account_id": accountID,
			}
		}
	}

	usageByKey := map[string]any{}
	var usageMu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 6)
	for uniqueKey, candidate := range usageCandidates {
		wg.Add(1)
		go func(key string, item map[string]string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			usage, usageErr := fetchCPAAccountUsage(c.Request.Context(), managementKey, item["auth_index"], item["account_id"])
			usageMu.Lock()
			defer usageMu.Unlock()
			if usageErr != nil {
				usageByKey[key] = map[string]any{"error": usageErr.Error()}
				return
			}
			usageByKey[key] = usage
		}(uniqueKey, candidate)
	}
	wg.Wait()

	for _, account := range accounts {
		uniqueKey := stringValue(account["unique_key"])
		account["duplicate"] = snapshot.uniqueCounts[uniqueKey] > 1
		account["duplicate_count"] = snapshot.uniqueCounts[uniqueKey]
		if usage := usageByKey[uniqueKey]; usage != nil {
			account["usage"] = usage
			if usageMap, ok := usage.(map[string]any); ok {
				if plan := stringValue(usageMap["plan_type"]); plan != "" {
					account["plan_type"] = plan
				}
			}
		}
		delete(account, "unique_key")
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"accounts": accounts,
			"page":     pageInfo.Page, "page_size": pageInfo.PageSize, "total": len(snapshot.candidates),
			"sort_by": sortBy, "sort_order": sortOrder,
			"summary": gin.H{
				"total":      len(snapshot.candidates),
				"unique":     len(snapshot.uniqueCounts),
				"active":     snapshot.active,
				"disabled":   snapshot.disabled,
				"duplicates": len(snapshot.candidates) - len(snapshot.uniqueCounts),
			},
		},
	})
}

func readCPAAccountOutputs() map[string]float64 {
	result := map[string]float64{}
	path := strings.TrimSpace(os.Getenv("QAPI_CAPACITY_SNAPSHOT_PATH"))
	if path == "" {
		path = "/run/qapi-monitor/status.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var snapshot cpaAccountOutputSnapshot
	if common.Unmarshal(data, &snapshot) != nil || common.QuotaPerUnit <= 0 {
		return result
	}
	for _, item := range snapshot.AccountOutputs {
		assetKey := strings.TrimSpace(item.AssetKey)
		if assetKey == "" || item.CumulativeOutputQuota <= 0 || math.IsNaN(item.CumulativeOutputQuota) || math.IsInf(item.CumulativeOutputQuota, 0) {
			continue
		}
		result[assetKey] = item.CumulativeOutputQuota / common.QuotaPerUnit
	}
	return result
}

func listCachedCPAAuthFiles(ctx context.Context, managementKey string) ([]map[string]any, error) {
	now := time.Now()
	cpaAuthFilesCache.RLock()
	if now.Before(cpaAuthFilesCache.expiresAt) {
		files := cpaAuthFilesCache.files
		cpaAuthFilesCache.RUnlock()
		return files, nil
	}
	cpaAuthFilesCache.RUnlock()
	cpaAuthFilesCache.refresh.Lock()
	defer cpaAuthFilesCache.refresh.Unlock()
	cpaAuthFilesCache.RLock()
	if time.Now().Before(cpaAuthFilesCache.expiresAt) {
		files := cpaAuthFilesCache.files
		cpaAuthFilesCache.RUnlock()
		return files, nil
	}
	cpaAuthFilesCache.RUnlock()

	body, status, err := doCPARequest(ctx, managementKey, http.MethodGet, "/v0/management/auth-files", nil)
	if err != nil || status < 200 || status >= 300 {
		return nil, fmt.Errorf("failed to list CPA accounts")
	}
	var list struct {
		Files []map[string]any `json:"files"`
	}
	if err := common.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	cpaAuthFilesCache.Lock()
	cpaAuthFilesCache.files = list.Files
	cpaAuthFilesCache.expiresAt = now.Add(15 * time.Second)
	cpaAuthFilesCache.Unlock()
	return list.Files, nil
}

func invalidateCPAAuthFilesCache() {
	cpaAuthFilesCache.refresh.Lock()
	defer cpaAuthFilesCache.refresh.Unlock()
	cpaAuthFilesCache.Lock()
	cpaAuthFilesCache.files = nil
	cpaAuthFilesCache.expiresAt = time.Time{}
	cpaAuthFilesCache.Unlock()
}

func buildCPAAccountPoolSnapshot(files []map[string]any, poolTypeFilter string) (*cpaAccountPoolSnapshot, error) {
	assetKeys := make([]string, 0, len(files))
	for _, file := range files {
		if !strings.EqualFold(stringValue(file["type"]), "codex") {
			continue
		}
		authIndex := stringValue(file["auth_index"])
		if authIndex == "" {
			continue
		}
		assetKeys = append(assetKeys, model.OperationalCPAAssetKey(authIndex))
	}
	assetsByKey, err := model.GetOperationalAssetsByKeys(model.OperationalAssetTypeCPAAccount, assetKeys)
	if err != nil {
		return nil, err
	}

	snapshot := &cpaAccountPoolSnapshot{
		candidates:   make([]cpaAccountCandidate, 0, len(files)),
		assetsByKey:  assetsByKey,
		uniqueCounts: map[string]int{},
	}
	for _, file := range files {
		if !strings.EqualFold(stringValue(file["type"]), "codex") {
			continue
		}
		authIndex := stringValue(file["auth_index"])
		if authIndex == "" {
			continue
		}
		assetKey := model.OperationalCPAAssetKey(authIndex)
		storedAsset, hasStoredAsset := assetsByKey[assetKey]
		poolType := resolveCPAAccountPoolType(stringValue(file["name"]), storedAsset.PoolType)
		if poolTypeFilter != "" && poolType != poolTypeFilter {
			continue
		}
		if hasStoredAsset && storedAsset.State == model.OperationalAssetStateArchived {
			continue
		}

		email := stringValue(file["email"])
		metadata := service.ExtractCPAAuthMetadata(file)
		if email == "" {
			email = metadata.Email
		}
		name := stringValue(file["name"])
		sortName := strings.ToLower(strings.TrimSpace(email))
		if sortName == "" {
			sortName = strings.ToLower(name)
		}
		uniqueKey := sortName
		snapshot.uniqueCounts[uniqueKey]++
		isDisabled, _ := file["disabled"].(bool)
		isUnavailable, _ := file["unavailable"].(bool)
		if isDisabled {
			snapshot.disabled++
		} else {
			snapshot.active++
		}
		createdAt, createdOK := cpaTimestampValue(file["created_at"])
		updatedAt, updatedOK := cpaTimestampValue(file["updated_at"])
		snapshot.candidates = append(snapshot.candidates, cpaAccountCandidate{
			file: file, assetKey: assetKey, uniqueKey: uniqueKey, email: email, sortName: sortName, poolType: poolType,
			costMinor: storedAsset.CostMinor, disabled: isDisabled, unavailable: isUnavailable,
			success: cpaNumberValue(file["success"]), failed: cpaNumberValue(file["failed"]),
			createdAt: createdAt, createdOK: createdOK, updatedAt: updatedAt, updatedOK: updatedOK, metadata: metadata,
		})
	}
	return snapshot, nil
}

func GetCPAAccountSelection(c *gin.Context) {
	poolType, ok := normalizeCPAAccountPoolType(c.Query("pool_type"), false)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号池类型无效"})
		return
	}
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	files, err := listCachedCPAAuthFiles(c.Request.Context(), managementKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 返回格式错误"})
		return
	}
	snapshot, err := buildCPAAccountPoolSnapshot(files, poolType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取 CPA 账号失败"})
		return
	}
	sortCPAAccountCandidates(snapshot.candidates, "name", "asc")
	names := make([]string, 0, len(snapshot.candidates))
	for _, candidate := range snapshot.candidates {
		names = append(names, stringValue(candidate.file["name"]))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"names": names, "total": len(names)},
	})
}

func cpaAccountPoolType(name string) string {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	if cpaTemporaryAccountNamePattern.MatchString(normalizedName) {
		return model.CPAAccountPoolTemporary
	}
	if cpaImportedAccountNamePattern.MatchString(normalizedName) {
		return model.CPAAccountPoolImported
	}
	return model.CPAAccountPoolOfficial
}

func normalizeCPAAccountPoolType(poolType string, allowEmpty bool) (string, bool) {
	poolType = strings.TrimSpace(poolType)
	if poolType == "" {
		return "", allowEmpty
	}
	if poolType == model.CPAAccountPoolImported || poolType == model.CPAAccountPoolTemporary || poolType == model.CPAAccountPoolOfficial {
		return poolType, true
	}
	return "", false
}

func resolveCPAAccountPoolType(name string, storedPoolType string) string {
	storedPoolType = strings.TrimSpace(storedPoolType)
	if storedPoolType == model.CPAAccountPoolImported || storedPoolType == model.CPAAccountPoolTemporary || storedPoolType == model.CPAAccountPoolOfficial {
		return storedPoolType
	}
	return cpaAccountPoolType(name)
}

func normalizeCPAImportPoolType(poolType string) (string, bool) {
	poolType = strings.TrimSpace(poolType)
	if poolType == "" {
		return model.CPAAccountPoolImported, true
	}
	if poolType == model.CPAAccountPoolImported || poolType == model.CPAAccountPoolTemporary {
		return poolType, true
	}
	return "", false
}

func normalizeCPAAccountSortBy(sortBy string) (string, bool) {
	sortBy = strings.TrimSpace(sortBy)
	if sortBy == "" {
		return "name", true
	}
	switch sortBy {
	case "name", "cost", "status", "success", "failed", "created_at", "updated_at":
		return sortBy, true
	default:
		return "", false
	}
}

func defaultCPAAccountSortOrder(sortBy string) string {
	switch sortBy {
	case "cost", "success", "failed", "created_at", "updated_at":
		return "desc"
	default:
		return "asc"
	}
}

func normalizeCPAAccountSortOrder(sortOrder string, sortBy string) (string, bool) {
	sortOrder = strings.TrimSpace(sortOrder)
	if sortOrder == "" {
		return defaultCPAAccountSortOrder(sortBy), true
	}
	if sortOrder == "asc" || sortOrder == "desc" {
		return sortOrder, true
	}
	return "", false
}

func sortCPAAccountCandidates(candidates []cpaAccountCandidate, sortBy string, sortOrder string) {
	descending := sortOrder == "desc"
	sort.SliceStable(candidates, func(i int, j int) bool {
		left := candidates[i]
		right := candidates[j]
		comparison := 0
		missingComparison := 0
		switch sortBy {
		case "cost":
			comparison = cmp.Compare(left.costMinor, right.costMinor)
		case "status":
			comparison = cmp.Compare(cpaAccountStatusRank(left), cpaAccountStatusRank(right))
		case "success":
			comparison = cmp.Compare(left.success, right.success)
		case "failed":
			comparison = cmp.Compare(left.failed, right.failed)
		case "created_at":
			missingComparison = compareCPAAccountSortPresence(left.createdOK, right.createdOK)
			comparison = cmp.Compare(left.createdAt, right.createdAt)
		case "updated_at":
			missingComparison = compareCPAAccountSortPresence(left.updatedOK, right.updatedOK)
			comparison = cmp.Compare(left.updatedAt, right.updatedAt)
		default:
			comparison = strings.Compare(left.sortName, right.sortName)
		}
		if missingComparison != 0 {
			return missingComparison < 0
		}
		if descending {
			comparison = -comparison
		}
		if comparison == 0 {
			comparison = strings.Compare(strings.ToLower(stringValue(left.file["name"])), strings.ToLower(stringValue(right.file["name"])))
		}
		return comparison < 0
	})
}

func compareCPAAccountSortPresence(leftPresent bool, rightPresent bool) int {
	if leftPresent == rightPresent {
		return 0
	}
	if leftPresent {
		return -1
	}
	return 1
}

func cpaAccountStatusRank(candidate cpaAccountCandidate) int64 {
	if candidate.disabled {
		return 2
	}
	if candidate.unavailable {
		return 1
	}
	return 0
}

type cpaAccountArchiveRequest struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

func ArchiveCPAAccount(c *gin.Context) {
	var req cpaAccountArchiveRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号名称不能为空"})
		return
	}
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	files, err := listCachedCPAAuthFiles(c.Request.Context(), managementKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "读取 CPA 账号失败"})
		return
	}
	var selected map[string]any
	for _, file := range files {
		if stringValue(file["name"]) == strings.TrimSpace(req.Name) {
			selected = file
			break
		}
	}
	if selected == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "CPA 账号不存在"})
		return
	}
	if err := archiveCPAAccountFile(c.Request.Context(), managementKey, selected, req.Reason, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	invalidateCPAAuthFilesCache()
	common.ApiSuccess(c, nil)
}

type cpaAccountStatusRequest struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}

func SetCPAAccountStatus(c *gin.Context) {
	var req cpaAccountStatusRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号名称不能为空"})
		return
	}
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	if err := setCPAAccountDisabled(c.Request.Context(), managementKey, req.Name, req.Disabled); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "更新 CPA 账号状态失败"})
		return
	}
	invalidateCPAAuthFilesCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "账号状态已更新"})
}

func DeleteCPAAccount(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号名称不能为空"})
		return
	}
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	if err := deleteCPAAccountFile(c.Request.Context(), managementKey, name); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "删除 CPA 账号失败"})
		return
	}
	invalidateCPAAuthFilesCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "账号已删除"})
}

func BatchManageCPAAccounts(c *gin.Context) {
	var req cpaAccountBatchRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "批量管理参数无效"})
		return
	}
	names := make([]string, 0, len(req.Names))
	seen := make(map[string]struct{}, len(req.Names))
	for _, rawName := range req.Names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 || len(names) > cpaBatchMaxAccounts {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请选择 1 到 2000 个 CPA 账号"})
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.Action != cpaBatchActionDisable && req.Action != cpaBatchActionEnable &&
		req.Action != cpaBatchActionMove && req.Action != cpaBatchActionCost &&
		req.Action != cpaBatchActionArchive && req.Action != cpaBatchActionDelete {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "批量管理操作无效"})
		return
	}
	if req.Action == cpaBatchActionMove {
		if strings.TrimSpace(req.PoolType) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "目标账号池无效"})
			return
		}
		poolType, ok := normalizeCPAImportPoolType(req.PoolType)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "目标账号池无效"})
			return
		}
		req.PoolType = poolType
	}
	if req.Action == cpaBatchActionCost && (req.CostMinor == nil || *req.CostMinor < 0) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "批量成本无效"})
		return
	}
	if req.Action == cpaBatchActionDelete && strings.TrimSpace(req.DeleteConfirmation) != expectedCPABatchDeleteConfirmation(len(names)) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "永久删除确认文本无效"})
		return
	}

	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	files, err := listCachedCPAAuthFiles(c.Request.Context(), managementKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "读取 CPA 账号失败"})
		return
	}
	filesByName := make(map[string]map[string]any, len(files))
	for _, file := range files {
		name := stringValue(file["name"])
		if name != "" {
			filesByName[name] = file
		}
	}
	logMessage := fmt.Sprintf(
		"CPA batch action requested: user_id=%d client_ip=%s action=%s count=%d",
		c.GetInt("id"), c.ClientIP(), req.Action, len(names),
	)
	if req.Action == cpaBatchActionDelete {
		logger.LogWarn(c.Request.Context(), logMessage)
	} else {
		logger.LogInfo(c.Request.Context(), logMessage)
	}

	succeeded := make([]string, 0, len(names))
	failed := make([]gin.H, 0)
	for _, name := range names {
		file := filesByName[name]
		if file == nil {
			failed = append(failed, gin.H{"name": name, "error": "CPA 账号不存在"})
			continue
		}
		var actionErr error
		switch req.Action {
		case cpaBatchActionDisable:
			actionErr = setCPAAccountDisabled(c.Request.Context(), managementKey, name, true)
		case cpaBatchActionEnable:
			actionErr = setCPAAccountDisabled(c.Request.Context(), managementKey, name, false)
		case cpaBatchActionMove:
			actionErr = setCPAAccountPool(file, req.PoolType, c.GetInt("id"))
		case cpaBatchActionCost:
			actionErr = setCPAAccountCost(file, *req.CostMinor, req.CostDate, req.CostNote, c.GetInt("id"))
		case cpaBatchActionArchive:
			actionErr = archiveCPAAccountFile(c.Request.Context(), managementKey, file, req.ArchiveReason, c.GetInt("id"))
		case cpaBatchActionDelete:
			actionErr = deleteCPAAccountFile(c.Request.Context(), managementKey, name)
		}
		if actionErr != nil {
			failed = append(failed, gin.H{"name": name, "error": actionErr.Error()})
			continue
		}
		succeeded = append(succeeded, name)
	}
	if req.Action == cpaBatchActionDisable || req.Action == cpaBatchActionEnable ||
		req.Action == cpaBatchActionArchive || req.Action == cpaBatchActionDelete {
		invalidateCPAAuthFilesCache()
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf(
		"CPA batch action completed: user_id=%d action=%s requested=%d succeeded=%d failed=%d",
		c.GetInt("id"), req.Action, len(names), len(succeeded), len(failed),
	))
	c.JSON(http.StatusOK, gin.H{
		"success": len(failed) == 0,
		"message": fmt.Sprintf("批量管理完成：成功 %d，失败 %d", len(succeeded), len(failed)),
		"data": gin.H{
			"total": len(names), "succeeded": succeeded, "failed": failed,
		},
	})
}

func expectedCPABatchDeleteConfirmation(count int) string {
	return fmt.Sprintf("DELETE %d", count)
}

func setCPAAccountDisabled(ctx context.Context, managementKey string, name string, disabled bool) error {
	payload, err := common.Marshal(map[string]any{"name": name, "disabled": disabled})
	if err != nil {
		return fmt.Errorf("账号状态参数无效")
	}
	_, status, err := doCPARequest(ctx, managementKey, http.MethodPatch, "/v0/management/auth-files/status", payload)
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("更新账号状态失败")
	}
	return nil
}

func deleteCPAAccountFile(ctx context.Context, managementKey string, name string) error {
	path := "/v0/management/auth-files?name=" + url.QueryEscape(name)
	_, status, err := doCPARequest(ctx, managementKey, http.MethodDelete, path, nil)
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("删除账号失败")
	}
	return nil
}

func setCPAAccountPool(file map[string]any, poolType string, userID int) error {
	authIndex := stringValue(file["auth_index"])
	if authIndex == "" {
		return fmt.Errorf("CPA 账号缺少 auth_index")
	}
	displayName := stringValue(file["email"])
	if displayName == "" {
		displayName = stringValue(file["name"])
	}
	return model.SetCPAOperationalAssetPool(&model.OperationalAsset{
		SourceKey: model.OperationalCPAAssetKey(authIndex), SourceRef: authIndex, DisplayName: displayName,
		CreatedBy: userID, UpdatedBy: userID,
	}, poolType, userID)
}

func setCPAAccountCost(file map[string]any, costMinor int64, costDate int64, note string, userID int) error {
	authIndex := stringValue(file["auth_index"])
	if authIndex == "" {
		return fmt.Errorf("CPA 账号缺少 auth_index")
	}
	displayName := stringValue(file["email"])
	if displayName == "" {
		displayName = stringValue(file["name"])
	}
	return model.SetOperationalAssetCost(&model.OperationalAsset{
		SourceType: model.OperationalAssetTypeCPAAccount,
		SourceKey:  model.OperationalCPAAssetKey(authIndex), SourceRef: authIndex, DisplayName: displayName,
		CreatedBy: userID, UpdatedBy: userID,
	}, costMinor, costDate, note, userID)
}

func archiveCPAAccountFile(ctx context.Context, managementKey string, file map[string]any, reason string, userID int) error {
	name := stringValue(file["name"])
	authIndex := stringValue(file["auth_index"])
	if name == "" || authIndex == "" {
		return fmt.Errorf("CPA 账号信息不完整")
	}
	metadata, _, resolveErr := service.ResolveAndRepairCPAAuthMetadata(ctx, file)
	if resolveErr != nil {
		metadata = service.ExtractCPAAuthMetadata(file)
	}
	if err := setCPAAccountDisabled(ctx, managementKey, name, true); err != nil {
		return fmt.Errorf("冻结账号失败，未完成归档")
	}
	email := stringValue(file["email"])
	if email == "" {
		email = metadata.Email
	}
	if email == "" {
		email = name
	}
	snapshot, err := common.Marshal(map[string]any{
		"name": name, "email": email, "auth_index": authIndex,
		"account_id": metadata.AccountID, "plan_type": metadata.PlanType,
		"status": file["status"], "status_message": file["status_message"],
	})
	if err != nil {
		return err
	}
	asset := &model.OperationalAsset{
		SourceType: model.OperationalAssetTypeCPAAccount,
		SourceKey:  model.OperationalCPAAssetKey(authIndex), SourceRef: authIndex, DisplayName: email,
	}
	if err := model.ArchiveOperationalAsset(asset, string(snapshot), reason, userID); err != nil {
		return err
	}
	if err := model.DisableActivationSource(model.ActivationSourceCPAAccount, authIndex); err != nil {
		common.SysError("failed to disable archived CPA activation target: " + err.Error())
	}
	return nil
}

func decodeCPAAccounts(content string) ([]map[string]any, error) {
	accounts := make([]map[string]any, 0)
	err := common.DecodeJsonSequence(strings.NewReader(content), func(value any) error {
		switch item := value.(type) {
		case map[string]any:
			accounts = append(accounts, item)
		case []any:
			for _, raw := range item {
				account, ok := raw.(map[string]any)
				if !ok {
					return fmt.Errorf("JSON 数组中只能包含账号对象")
				}
				accounts = append(accounts, account)
			}
		default:
			return fmt.Errorf("请输入账号对象或账号对象数组")
		}
		return nil
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "JSON 数组") || strings.HasPrefix(err.Error(), "请输入") {
			return nil, err
		}
		return nil, fmt.Errorf("JSON 格式错误：%v", err)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("没有找到可上货的账号")
	}
	return accounts, nil
}

func normalizeCPAAccount(account map[string]any, poolType string) (string, map[string]any, error) {
	accessToken := stringValue(account["access_token"])
	if accessToken == "" {
		return "", nil, fmt.Errorf("缺少 access_token")
	}
	claims := decodeJWTClaims(accessToken)

	setDefaultString(account, "type", "codex")
	setDefaultString(account, "client_id", defaultCodexClientID)
	if !service.IsCLIProxyCompatibleCPAToken(stringValue(account["id_token"])) {
		if !service.IsCLIProxyCompatibleCPAToken(accessToken) {
			return "", nil, fmt.Errorf("access_token 无法生成可解析的 id_token")
		}
		account["id_token"] = accessToken
	}
	accessMetadata := service.ExtractCPAAuthMetadata(account)
	if accessMetadata.AccountID == "" {
		return "", nil, fmt.Errorf("access_token 缺少 chatgpt_account_id")
	}
	setDefaultString(account, "refresh_token", "")
	setDefaultString(account, "password", "Takeover_NoPassword")
	account["account_id"] = accessMetadata.AccountID
	account["chatgpt_account_id"] = accessMetadata.AccountID
	if accessMetadata.PlanType != "" {
		account["plan_type"] = accessMetadata.PlanType
		account["chatgpt_plan_type"] = accessMetadata.PlanType
	}
	setDefaultString(account, "email", accessMetadata.Email)
	setDefaultString(account, "last_refresh", time.Now().UTC().Format(time.RFC3339))
	if stringValue(account["expired"]) == "" {
		if exp, ok := claims["exp"].(float64); ok && exp > 0 {
			account["expired"] = time.Unix(int64(exp), 0).UTC().Format(time.RFC3339)
		}
	}
	if _, ok := account["disabled"]; !ok {
		account["disabled"] = false
	}

	email := cpaFileNameCleaner.ReplaceAllString(stringValue(account["email"]), "-")
	if email == "" {
		email = "account"
	}
	sum := sha256.Sum256([]byte(accessToken))
	namePrefix := "codex-"
	if poolType == model.CPAAccountPoolTemporary {
		namePrefix = "codex-temp-"
	}
	name := fmt.Sprintf("%s%s-%s.json", namePrefix, email, hex.EncodeToString(sum[:4]))
	return name, account, nil
}

func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return map[string]any{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return map[string]any{}
	}
	claims := map[string]any{}
	if common.Unmarshal(payload, &claims) != nil {
		return map[string]any{}
	}
	return claims
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func cpaNumberValue(value any) float64 {
	parsed := float64(0)
	switch number := value.(type) {
	case int:
		parsed = float64(number)
	case int64:
		parsed = float64(number)
	case float64:
		parsed = number
	case string:
		parsed, _ = strconv.ParseFloat(strings.TrimSpace(number), 64)
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return parsed
}

func cpaTimestampValue(value any) (int64, bool) {
	switch timestamp := value.(type) {
	case int:
		return int64(timestamp), timestamp > 0
	case int64:
		return timestamp, timestamp > 0
	case float64:
		if math.IsNaN(timestamp) || math.IsInf(timestamp, 0) || timestamp <= 0 {
			return 0, false
		}
		parsed := int64(timestamp)
		if parsed > 1_000_000_000_000 {
			parsed /= 1000
		}
		return parsed, true
	case string:
		timestamp = strings.TrimSpace(timestamp)
		if timestamp == "" {
			return 0, false
		}
		if parsed, err := strconv.ParseInt(timestamp, 10, 64); err == nil && parsed > 0 {
			if parsed > 1_000_000_000_000 {
				parsed /= 1000
			}
			return parsed, true
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			parsed, err := time.Parse(layout, timestamp)
			if err == nil {
				return parsed.Unix(), true
			}
		}
	}
	return 0, false
}

func setDefaultString(values map[string]any, key, value string) {
	if stringValue(values[key]) == "" {
		values[key] = value
	}
}

func readCPAManagementKey() (string, error) {
	path := strings.TrimSpace(os.Getenv("CPA_MANAGEMENT_KEY_FILE"))
	if path == "" {
		path = "/etc/new-api/cpa-management-key"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("empty management key")
	}
	return key, nil
}

func uploadCPAAccount(ctx context.Context, managementKey, name string, account map[string]any) error {
	body, err := common.Marshal(account)
	if err != nil {
		return fmt.Errorf("账号序列化失败")
	}
	_, status, err := doCPARequest(ctx, managementKey, http.MethodPost, "/v0/management/auth-files?name="+url.QueryEscape(name), body)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("CPA 返回 HTTP %d", status)
	}
	return nil
}

func doCPARequest(ctx context.Context, managementKey, method, path string, body []byte) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:8317"+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 CPA 请求失败")
	}
	req.Header.Set("Authorization", "Bearer "+managementKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("CPA 不可达")
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取 CPA 响应失败")
	}
	return responseBody, resp.StatusCode, nil
}

func fetchCPAAccountUsage(ctx context.Context, managementKey, authIndex, accountID string) (map[string]any, error) {
	payload, _ := common.Marshal(map[string]any{
		"auth_index": authIndex,
		"method":     http.MethodGet,
		"url":        "https://chatgpt.com/backend-api/wham/usage",
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"chatgpt-account-id": accountID,
			"Accept":             "application/json",
			"originator":         "codex_cli_rs",
		},
	})
	body, status, err := doCPARequest(ctx, managementKey, http.MethodPost, "/v0/management/api-call", payload)
	if err != nil || status < 200 || status >= 300 {
		return nil, fmt.Errorf("额度查询失败")
	}
	var outer struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
	}
	if common.Unmarshal(body, &outer) != nil || outer.StatusCode < 200 || outer.StatusCode >= 300 {
		return nil, fmt.Errorf("额度查询返回异常")
	}
	usage := map[string]any{}
	if common.UnmarshalJsonStr(outer.Body, &usage) != nil {
		return nil, fmt.Errorf("额度数据格式错误")
	}
	return usage, nil
}
