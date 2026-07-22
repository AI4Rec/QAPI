package controller

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const accountInspectionMaxJSONSize = 10 << 20

var accountInspectionHTTPStatusPattern = regexp.MustCompile(`(?:HTTP |returned )(\d{3})`)

type accountInspectionRequest struct {
	Provider   string `json:"provider"`
	Operation  string `json:"operation"`
	Identifier string `json:"identifier"`
	Content    string `json:"content"`
}

type accountInspectionResult struct {
	Provider       string `json:"provider"`
	Operation      string `json:"operation"`
	Identifier     string `json:"identifier"`
	Email          string `json:"email,omitempty"`
	Source         string `json:"source"`
	State          string `json:"state"`
	Success        bool   `json:"success"`
	LatencyMS      int64  `json:"latency_ms"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
	Message        string `json:"message,omitempty"`
	Usage          any    `json:"usage,omitempty"`
}

type accountInspectionTarget struct {
	Provider   string `json:"provider"`
	Identifier string `json:"identifier"`
	Email      string `json:"email,omitempty"`
	Source     string `json:"source"`
	State      string `json:"state"`
}

func ListAccountInspectionTargets(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Query("provider")))
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	if provider == "" {
		provider = "all"
	}
	if scope == "" {
		scope = "attention"
	}
	if (provider != "all" && provider != "cpa" && provider != "sub2api") || (scope != "all" && scope != "attention") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验号筛选条件无效"})
		return
	}

	archivedAssets, err := model.ListArchivedOperationalAssets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取归档账号失败"})
		return
	}
	archived := make(map[string]model.OperationalAsset, len(archivedAssets))
	for _, asset := range archivedAssets {
		archived[asset.SourceType+":"+asset.SourceKey] = asset
	}

	targets := make([]accountInspectionTarget, 0)
	if provider == "all" || provider == "cpa" {
		managementKey, keyErr := readCPAManagementKey()
		if keyErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
			return
		}
		files, listErr := listCachedCPAAuthFiles(c.Request.Context(), managementKey)
		if listErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "读取 CPA 账号失败"})
			return
		}
		for _, file := range files {
			authIndex := stringValue(file["auth_index"])
			name := stringValue(file["name"])
			asset, isArchived := archived[model.OperationalAssetTypeCPAAccount+":"+model.OperationalCPAAssetKey(authIndex)]
			disabled, _ := file["disabled"].(bool)
			unavailable, _ := file["unavailable"].(bool)
			if scope == "attention" && !disabled && !unavailable && !isArchived {
				continue
			}
			state := "available"
			if isArchived {
				state = "archived"
			} else if disabled {
				state = "frozen"
			} else if unavailable {
				state = "unavailable"
			}
			email := stringValue(file["email"])
			if email == "" && isArchived {
				email = asset.DisplayName
			}
			targets = append(targets, accountInspectionTarget{Provider: "cpa", Identifier: name, Email: email, Source: "pool", State: state})
		}
	}

	if provider == "all" || provider == "sub2api" {
		accounts, listErr := listAllSub2APIAccounts(c.Request.Context())
		if listErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": listErr.Error()})
			return
		}
		for _, account := range accounts {
			asset, isArchived := archived[model.OperationalAssetTypeSub2APIAccount+":"+model.OperationalSub2APIAssetKey(account.ID)]
			if scope == "attention" && account.Schedulable && account.Status == "active" && !isArchived {
				continue
			}
			state := "available"
			if isArchived {
				state = "archived"
			} else if !account.Schedulable {
				state = "frozen"
			} else if account.Status != "active" {
				state = "unavailable"
			}
			email := strings.TrimSpace(account.Credentials.Email)
			if email == "" && isArchived {
				email = asset.DisplayName
			}
			targets = append(targets, accountInspectionTarget{Provider: "sub2api", Identifier: strconv.FormatInt(account.ID, 10), Email: email, Source: "pool", State: state})
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": targets}})
}

func InspectAccount(c *gin.Context) {
	var req accountInspectionRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验号参数无效"})
		return
	}
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.Operation = strings.ToLower(strings.TrimSpace(req.Operation))
	req.Identifier = strings.TrimSpace(req.Identifier)
	req.Content = strings.TrimSpace(req.Content)
	if req.Operation != "verify" && req.Operation != "ping" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "验号操作无效"})
		return
	}
	if len(req.Content) > accountInspectionMaxJSONSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "内容不能超过 10MB"})
		return
	}
	if req.Content != "" {
		result, err := inspectJSONAccount(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
		return
	}
	if req.Identifier == "" || (req.Provider != "cpa" && req.Provider != "sub2api") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请选择要测试的账号"})
		return
	}
	result := inspectStoredAccount(c.Request.Context(), req.Provider, req.Operation, req.Identifier)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func inspectStoredAccount(ctx context.Context, provider string, operation string, identifier string) (result accountInspectionResult) {
	startedAt := time.Now()
	result = accountInspectionResult{Provider: provider, Operation: operation, Identifier: identifier, Source: "pool", State: "unavailable"}
	defer func() { result.LatencyMS = time.Since(startedAt).Milliseconds() }()
	if provider == "cpa" {
		managementKey, err := readCPAManagementKey()
		if err != nil {
			result.Message = "CPA 管理密钥不可用"
			return result
		}
		files, err := listCachedCPAAuthFiles(ctx, managementKey)
		if err != nil {
			result.Message = "读取 CPA 账号失败"
			return result
		}
		var selected map[string]any
		for _, file := range files {
			if stringValue(file["name"]) == identifier || stringValue(file["auth_index"]) == identifier {
				selected = file
				break
			}
		}
		if selected == nil {
			result.Message = "CPA 账号不存在"
			return result
		}
		metadata, _, err := service.ResolveAndRepairCPAAuthMetadata(ctx, selected)
		if err != nil {
			metadata = service.ExtractCPAAuthMetadata(selected)
		}
		result.Email = metadata.Email
		authIndex := stringValue(selected["auth_index"])
		if operation == "ping" {
			err = service.PingCPAAccount(ctx, authIndex)
			if err != nil {
				result.Message = err.Error()
				result.UpstreamStatus = accountInspectionStatusFromError(err.Error())
				return result
			}
			result.Success = true
			result.State = "available"
			result.UpstreamStatus = http.StatusOK
			return result
		}
		usage, err := fetchCPAAccountUsage(ctx, managementKey, authIndex, metadata.AccountID)
		if err != nil {
			result.Message = err.Error()
			return result
		}
		result.Success = true
		result.State = accountInspectionUsageState(usage)
		result.UpstreamStatus = http.StatusOK
		result.Usage = usage
		return result
	}

	accountID, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil || accountID <= 0 {
		result.Message = "Sub2API 账号 ID 无效"
		return result
	}
	account, err := getSub2APIAccount(ctx, accountID)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	result.Email = strings.TrimSpace(account.Credentials.Email)
	if operation == "verify" {
		loadSub2APIAccountUsage(ctx, account)
		if account.Usage == nil || account.Usage.Error != "" {
			if account.Usage != nil {
				result.Message = account.Usage.Error
			}
			return result
		}
		result.Success = true
		result.State = "available"
		if account.Usage.RateLimit.LimitReached {
			result.State = "quota_exhausted"
		}
		result.UpstreamStatus = http.StatusOK
		result.Usage = account.Usage
		return result
	}
	body, _ := common.Marshal(map[string]string{"model_id": "gpt-5.6-sol", "prompt": "Reply with exactly OK."})
	responseBody, status, requestErr := doSub2APIRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/test", accountID), body)
	result.UpstreamStatus = status
	if requestErr != nil {
		result.Message = requestErr.Error()
		return result
	}
	success, message, upstreamStatus := parseSub2APIAccountTest(responseBody)
	result.Success = success
	result.Message = message
	if upstreamStatus > 0 {
		result.UpstreamStatus = upstreamStatus
	}
	if success {
		result.State = "available"
	}
	return result
}

func inspectJSONAccount(ctx context.Context, req accountInspectionRequest) (accountInspectionResult, error) {
	provider := req.Provider
	if provider == "" || provider == "auto" {
		provider = detectAccountJSONProvider(req.Content)
	}
	if provider != "cpa" && provider != "sub2api" {
		return accountInspectionResult{}, fmt.Errorf("无法识别 JSON 类型，请手动选择 CPA 或 Sub2API")
	}
	if provider == "cpa" {
		accounts, err := decodeCPAAccounts(req.Content)
		if err != nil {
			return accountInspectionResult{}, err
		}
		if len(accounts) != 1 {
			return accountInspectionResult{}, fmt.Errorf("JSON 单独测试一次只支持一个 CPA 账号")
		}
		managementKey, err := readCPAManagementKey()
		if err != nil {
			return accountInspectionResult{}, fmt.Errorf("CPA 管理密钥不可用")
		}
		_, normalized, err := normalizeCPAAccount(accounts[0], model.CPAAccountPoolTemporary)
		if err != nil {
			return accountInspectionResult{}, err
		}
		normalized["disabled"] = false
		name := fmt.Sprintf("codex-probe-%d.json", time.Now().UnixNano())
		if err := uploadCPAAccount(ctx, managementKey, name, normalized); err != nil {
			return accountInspectionResult{}, err
		}
		invalidateCPAAuthFilesCache()
		defer func() {
			cleanupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = deleteCPAAccountFile(cleanupContext, managementKey, name)
			invalidateCPAAuthFilesCache()
		}()
		result := inspectStoredAccount(ctx, "cpa", req.Operation, name)
		result.Source = "json"
		return result, nil
	}

	account, err := firstSub2APIJSONAccount(req.Content)
	if err != nil {
		return accountInspectionResult{}, err
	}
	probeName := fmt.Sprintf("qapi-probe-%d", time.Now().UnixNano())
	account["name"] = probeName
	payload := newSub2APIDataImport([]any{account})
	payload["skip_default_group_bind"] = true
	body, _ := common.Marshal(payload)
	responseBody, status, err := doSub2APIRequest(ctx, http.MethodPost, "/api/v1/admin/accounts/data", body)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		return accountInspectionResult{}, fmt.Errorf("Sub2API 临时挂载失败")
	}
	var upstream sub2APIResponse
	if common.Unmarshal(responseBody, &upstream) != nil || upstream.Code != 0 {
		return accountInspectionResult{}, fmt.Errorf("Sub2API 临时挂载失败：%s", strings.TrimSpace(upstream.Message))
	}
	accounts, err := listAllSub2APIAccounts(ctx)
	if err != nil {
		return accountInspectionResult{}, err
	}
	var accountID int64
	for _, candidate := range accounts {
		if candidate.Name == probeName {
			accountID = candidate.ID
			break
		}
	}
	if accountID == 0 {
		return accountInspectionResult{}, fmt.Errorf("Sub2API 未返回临时测试账号")
	}
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _, _ = doSub2APIRequest(cleanupContext, http.MethodDelete, fmt.Sprintf("/api/v1/admin/accounts/%d", accountID), nil)
	}()
	result := inspectStoredAccount(ctx, "sub2api", req.Operation, strconv.FormatInt(accountID, 10))
	result.Identifier = probeName
	result.Source = "json"
	return result, nil
}

func detectAccountJSONProvider(content string) string {
	var value any
	if common.UnmarshalJsonStr(content, &value) != nil {
		return ""
	}
	object, _ := value.(map[string]any)
	if object == nil {
		return ""
	}
	if object["type"] == "sub2api-data" || object["accounts"] != nil || (object["platform"] != nil && object["credentials"] != nil) {
		return "sub2api"
	}
	if object["access_token"] != nil || object["refresh_token"] != nil {
		return "cpa"
	}
	return ""
}

func firstSub2APIJSONAccount(content string) (map[string]any, error) {
	var value any
	if common.UnmarshalJsonStr(content, &value) != nil {
		return nil, fmt.Errorf("账号 JSON 格式无效")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON 中没有找到 Sub2API 账号")
	}
	if data, ok := object["data"].(map[string]any); ok {
		object = data
	}
	if accounts, ok := object["accounts"].([]any); ok && len(accounts) > 0 {
		account, ok := accounts[0].(map[string]any)
		if ok {
			return account, nil
		}
	}
	if object["platform"] != nil && object["credentials"] != nil {
		return object, nil
	}
	return nil, fmt.Errorf("JSON 中没有找到 Sub2API 账号")
}

func accountInspectionUsageState(usage map[string]any) string {
	rateLimit, _ := usage["rate_limit"].(map[string]any)
	limitReached, _ := rateLimit["limit_reached"].(bool)
	if limitReached {
		return "quota_exhausted"
	}
	return "available"
}

func accountInspectionStatusFromError(message string) int {
	match := accountInspectionHTTPStatusPattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return 0
	}
	status, _ := strconv.Atoi(match[1])
	return status
}

func parseSub2APIAccountTest(body []byte) (bool, string, int) {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Success bool   `json:"success"`
			Error   string `json:"error"`
			Text    string `json:"text"`
		}
		if common.UnmarshalJsonStr(strings.TrimSpace(strings.TrimPrefix(line, "data:")), &event) != nil {
			continue
		}
		if event.Type == "error" {
			return false, event.Error, accountInspectionStatusFromError(event.Error)
		}
		if event.Type == "test_complete" {
			if event.Success {
				return true, strings.TrimSpace(event.Text), http.StatusOK
			}
			return false, event.Error, accountInspectionStatusFromError(event.Error)
		}
	}
	return false, "Sub2API 测试未返回完成状态", 0
}
