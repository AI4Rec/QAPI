package controller

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	cpaOAuthProvider          = "codex"
	cpaOAuthSessionStateKey   = "cpa_oauth_state"
	cpaOAuthSessionUserKey    = "cpa_oauth_user_id"
	cpaOAuthSessionExpiryKey  = "cpa_oauth_expires_at"
	cpaOAuthSessionTTL        = 5 * time.Minute
	cpaOAuthMaxCallbackLength = 16 << 10
)

type cpaOAuthStartResponse struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

type cpaOAuthCallbackRequest struct {
	RedirectURL string `json:"redirect_url"`
}

type cpaOAuthStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func StartCPACodexOAuth(c *gin.Context) {
	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}

	body, status, err := doCPARequest(c.Request.Context(), managementKey, http.MethodGet, "/v0/management/codex-auth-url?is_webui=true", nil)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "无法启动 CPA 官方登录"})
		return
	}

	var response cpaOAuthStartResponse
	if err := common.Unmarshal(body, &response); err != nil || response.Status != "ok" {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 登录响应格式错误"})
		return
	}
	response.State = strings.TrimSpace(response.State)
	response.URL = strings.TrimSpace(response.URL)
	if !validCPAOAuthState(response.State) || !validCPAOAuthAuthorizationURL(response.URL) {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 返回了无效的登录地址"})
		return
	}

	expiresAt := time.Now().Add(cpaOAuthSessionTTL).Unix()
	session := sessions.Default(c)
	session.Set(cpaOAuthSessionStateKey, response.State)
	session.Set(cpaOAuthSessionUserKey, c.GetInt("id"))
	session.Set(cpaOAuthSessionExpiryKey, expiresAt)
	if err := session.Save(); err != nil {
		cancelCPAOAuthUpstream(c.Request.Context(), managementKey, response.State)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存登录会话失败"})
		return
	}
	if err := startCPARemoteBrowser(response.State, c.GetInt("id"), response.URL); err != nil {
		cancelCPAOAuthUpstream(c.Request.Context(), managementKey, response.State)
		clearCPAOAuthSession(c)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"provider":   cpaOAuthProvider,
			"url":        response.URL,
			"state":      response.State,
			"expires_at": expiresAt,
		},
	})
}

func SubmitCPAOAuthCallback(c *gin.Context) {
	var request cpaOAuthCallbackRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请提交登录后的完整回调地址"})
		return
	}
	request.RedirectURL = strings.TrimSpace(request.RedirectURL)
	if len(request.RedirectURL) == 0 || len(request.RedirectURL) > cpaOAuthMaxCallbackLength {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "回调地址无效"})
		return
	}

	state, err := parseCPAOAuthCallbackURL(request.RedirectURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := validateCPAOAuthSession(c, state); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		return
	}

	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	payload, err := common.Marshal(map[string]string{
		"provider":     cpaOAuthProvider,
		"redirect_url": request.RedirectURL,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	body, status, err := doCPARequest(c.Request.Context(), managementKey, http.MethodPost, "/v0/management/oauth-callback", payload)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 未能接收登录回调"})
		return
	}

	var response cpaOAuthStatusResponse
	if err := common.Unmarshal(body, &response); err != nil || response.Status != "ok" {
		message := strings.TrimSpace(response.Error)
		if message == "" {
			message = "CPA 拒绝了登录回调"
		}
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": message})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "登录回调已提交"})
}

func GetCPAOAuthStatus(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	if err := validateCPAOAuthSession(c, state); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		return
	}

	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	path := "/v0/management/get-auth-status?state=" + url.QueryEscape(state)
	body, status, err := doCPARequest(c.Request.Context(), managementKey, http.MethodGet, path, nil)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "读取 CPA 登录状态失败"})
		return
	}

	var response cpaOAuthStatusResponse
	if err := common.Unmarshal(body, &response); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 登录状态格式错误"})
		return
	}
	if response.Status != "wait" && response.Status != "ok" && response.Status != "error" {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "CPA 返回了未知登录状态"})
		return
	}
	if response.Status == "ok" || response.Status == "error" {
		stopCPARemoteBrowser(state, c.GetInt("id"))
		clearCPAOAuthSession(c)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"status": response.Status, "error": response.Error},
	})
}

func CancelCPAOAuth(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	if err := validateCPAOAuthSession(c, state); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": err.Error()})
		return
	}

	managementKey, err := readCPAManagementKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "CPA 管理密钥不可用"})
		return
	}
	stopCPARemoteBrowser(state, c.GetInt("id"))
	clearCPAOAuthSession(c)

	status, err := cancelCPAOAuthUpstream(c.Request.Context(), managementKey, state)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "取消 CPA 登录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "登录已取消"})
}

func cancelCPAOAuthUpstream(ctx context.Context, managementKey, state string) (int, error) {
	path := "/v0/management/oauth-session?state=" + url.QueryEscape(state)
	_, status, err := doCPARequest(ctx, managementKey, http.MethodDelete, path, nil)
	return status, err
}

func validateCPAOAuthSession(c *gin.Context, state string) error {
	if !validCPAOAuthState(state) {
		return errors.New("登录状态无效")
	}
	session := sessions.Default(c)
	storedState, _ := session.Get(cpaOAuthSessionStateKey).(string)
	storedUserID, _ := session.Get(cpaOAuthSessionUserKey).(int)
	expiresAt, _ := session.Get(cpaOAuthSessionExpiryKey).(int64)
	if storedState == "" || storedState != state || storedUserID != c.GetInt("id") {
		return errors.New("登录会话不匹配，请重新发起登录")
	}
	if expiresAt <= time.Now().Unix() {
		stopCPARemoteBrowser(storedState, storedUserID)
		clearCPAOAuthSession(c)
		return errors.New("登录会话已过期，请重新发起登录")
	}
	return nil
}

func clearCPAOAuthSession(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(cpaOAuthSessionStateKey)
	session.Delete(cpaOAuthSessionUserKey)
	session.Delete(cpaOAuthSessionExpiryKey)
	_ = session.Save()
}

func validCPAOAuthState(state string) bool {
	if len(state) < 16 || len(state) > 128 {
		return false
	}
	for _, char := range state {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-', char == '_', char == '.', char == '~':
		default:
			return false
		}
	}
	return true
}

func validCPAOAuthAuthorizationURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme == "https" && strings.EqualFold(parsed.Hostname(), "auth.openai.com") && parsed.User == nil
}

func parseCPAOAuthCallbackURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil {
		return "", errors.New("请粘贴浏览器地址栏中的完整 localhost 回调地址")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname != "localhost" && hostname != "127.0.0.1" && hostname != "::1" {
		return "", errors.New("回调地址必须指向 localhost")
	}
	state := strings.TrimSpace(parsed.Query().Get("state"))
	if !validCPAOAuthState(state) {
		return "", errors.New("回调地址缺少有效的 state")
	}
	if strings.TrimSpace(parsed.Query().Get("code")) == "" && strings.TrimSpace(parsed.Query().Get("error")) == "" {
		return "", errors.New("回调地址缺少 code 或 error")
	}
	return state, nil
}
