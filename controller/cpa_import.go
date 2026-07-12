package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultCodexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

var cpaFileNameCleaner = regexp.MustCompile(`[^a-zA-Z0-9._+-]+`)

type cpaImportRequest struct {
	Content string `json:"content"`
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
		name, normalized, normalizeErr := normalizeCPAAccount(account)
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

	success := len(failed) == 0
	message := fmt.Sprintf("CPA 上货完成：成功 %d，失败 %d", len(uploaded), len(failed))
	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": message,
		"data":    gin.H{"total": len(accounts), "uploaded": len(uploaded), "failed": failed, "files": uploaded},
	})
}

func GetCPAAccounts(c *gin.Context) {
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	body, status, err := doCPARequest(c.Request.Context(), managementKey, http.MethodGet, "/v0/management/auth-files", nil)
	if err != nil || status < 200 || status >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "读取 CPA 账号失败"})
		return
	}
	var list struct {
		Files []map[string]any `json:"files"`
	}
	if json.Unmarshal(body, &list) != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 返回格式错误"})
		return
	}

	accounts := make([]map[string]any, 0, len(list.Files))
	uniqueCounts := map[string]int{}
	usageCandidates := map[string]map[string]string{}
	active, disabled := 0, 0
	for _, file := range list.Files {
		if !strings.EqualFold(stringValue(file["type"]), "codex") {
			continue
		}
		email := stringValue(file["email"])
		uniqueKey := strings.ToLower(email)
		if uniqueKey == "" {
			uniqueKey = stringValue(file["name"])
		}
		uniqueCounts[uniqueKey]++
		isDisabled, _ := file["disabled"].(bool)
		if isDisabled {
			disabled++
		} else {
			active++
		}
		idToken := mapValue(file["id_token"])
		accountID := stringValue(idToken["chatgpt_account_id"])
		planType := stringValue(idToken["plan_type"])
		account := map[string]any{
			"name":             file["name"],
			"email":            email,
			"status":           file["status"],
			"status_message":   file["status_message"],
			"disabled":         isDisabled,
			"unavailable":      file["unavailable"],
			"success":          file["success"],
			"failed":           file["failed"],
			"recent_requests":  file["recent_requests"],
			"created_at":       file["created_at"],
			"updated_at":       file["updated_at"],
			"last_refresh":     file["last_refresh"],
			"next_retry_after": file["next_retry_after"],
			"plan_type":        planType,
			"account_id":       accountID,
			"unique_key":       uniqueKey,
		}
		accounts = append(accounts, account)
		if accountID != "" && usageCandidates[uniqueKey] == nil {
			usageCandidates[uniqueKey] = map[string]string{
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
		account["duplicate"] = uniqueCounts[uniqueKey] > 1
		account["duplicate_count"] = uniqueCounts[uniqueKey]
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
			"summary": gin.H{
				"total":      len(accounts),
				"unique":     len(uniqueCounts),
				"active":     active,
				"disabled":   disabled,
				"duplicates": len(accounts) - len(uniqueCounts),
			},
		},
	})
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
	payload, _ := json.Marshal(map[string]any{"name": req.Name, "disabled": req.Disabled})
	_, status, err := doCPARequest(c.Request.Context(), managementKey, http.MethodPatch, "/v0/management/auth-files/status", payload)
	if err != nil || status < 200 || status >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "更新 CPA 账号状态失败"})
		return
	}
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
	path := "/v0/management/auth-files?name=" + url.QueryEscape(name)
	_, status, err := doCPARequest(c.Request.Context(), managementKey, http.MethodDelete, path, nil)
	if err != nil || status < 200 || status >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "删除 CPA 账号失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "账号已删除"})
}

func decodeCPAAccounts(content string) ([]map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(content))
	accounts := make([]map[string]any, 0)
	for {
		var value any
		if err := decoder.Decode(&value); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("JSON 格式错误：%v", err)
		}
		switch item := value.(type) {
		case map[string]any:
			accounts = append(accounts, item)
		case []any:
			for _, raw := range item {
				account, ok := raw.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("JSON 数组中只能包含账号对象")
				}
				accounts = append(accounts, account)
			}
		default:
			return nil, fmt.Errorf("请输入账号对象或账号对象数组")
		}
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("没有找到可上货的账号")
	}
	return accounts, nil
}

func normalizeCPAAccount(account map[string]any) (string, map[string]any, error) {
	accessToken := stringValue(account["access_token"])
	if accessToken == "" {
		return "", nil, fmt.Errorf("缺少 access_token")
	}
	claims := decodeJWTClaims(accessToken)
	authClaims := mapValue(claims["https://api.openai.com/auth"])
	profileClaims := mapValue(claims["https://api.openai.com/profile"])

	setDefaultString(account, "type", "codex")
	setDefaultString(account, "client_id", defaultCodexClientID)
	setDefaultString(account, "id_token", accessToken)
	setDefaultString(account, "refresh_token", "")
	setDefaultString(account, "password", "Takeover_NoPassword")
	setDefaultString(account, "account_id", stringValue(authClaims["chatgpt_account_id"]))
	setDefaultString(account, "email", stringValue(profileClaims["email"]))
	setDefaultString(account, "plan_type", stringValue(authClaims["chatgpt_plan_type"]))
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
	name := fmt.Sprintf("codex-%s-%s.json", email, hex.EncodeToString(sum[:4]))
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
	if json.Unmarshal(payload, &claims) != nil {
		return map[string]any{}
	}
	return claims
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
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
	body, err := json.Marshal(account)
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
	payload, _ := json.Marshal(map[string]any{
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
	if json.Unmarshal(body, &outer) != nil || outer.StatusCode < 200 || outer.StatusCode >= 300 {
		return nil, fmt.Errorf("额度查询返回异常")
	}
	usage := map[string]any{}
	if json.Unmarshal([]byte(outer.Body), &usage) != nil {
		return nil, fmt.Errorf("额度数据格式错误")
	}
	return usage, nil
}
