package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	defaultSub2APIBaseURL = "http://127.0.0.1:18080"
	maxSub2APIImportSize  = 10 << 20
)

type sub2APIImportRequest struct {
	Content string `json:"content"`
}

type sub2APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type sub2APIAccount struct {
	ID                  int64                      `json:"id"`
	Name                string                     `json:"name"`
	Email               string                     `json:"email,omitempty"`
	Platform            string                     `json:"platform"`
	Type                string                     `json:"type"`
	Concurrency         int                        `json:"concurrency"`
	CurrentConcurrency  int                        `json:"current_concurrency"`
	Priority            int                        `json:"priority"`
	Status              string                     `json:"status"`
	Schedulable         bool                       `json:"schedulable"`
	ErrorMessage        string                     `json:"error_message,omitempty"`
	ExpiresAt           *int64                     `json:"expires_at,omitempty"`
	Credentials         sub2APIAccountCredentials  `json:"credentials"`
	PlanType            string                     `json:"plan_type,omitempty"`
	Usage               *sub2APIAccountUsage       `json:"usage,omitempty"`
	TodayStats          *sub2APIAccountWindowStats `json:"today_stats,omitempty"`
	AssetKey            string                     `json:"asset_key"`
	CostMinor           int64                      `json:"cost_minor"`
	CostDate            int64                      `json:"cost_date"`
	CostNote            string                     `json:"cost_note"`
	CumulativeOutputUSD float64                    `json:"cumulative_output_usd"`
}

type sub2APIAccountCredentials struct {
	Email     string `json:"email,omitempty"`
	PlanType  string `json:"plan_type,omitempty"`
	ExpiresAt string `json:"subscription_expires_at,omitempty"`
}

type sub2APIAccountListData struct {
	Items    []sub2APIAccount `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Pages    int              `json:"pages"`
}

type sub2APIAccountListResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    sub2APIAccountListData `json:"data"`
}

type sub2APIAccountResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    sub2APIAccount `json:"data"`
}

type sub2APIAccountWindowStats struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	StandardCost float64 `json:"standard_cost"`
	UserCost     float64 `json:"user_cost"`
}

type sub2APIUsageWindow struct {
	UsedPercent float64                    `json:"used_percent"`
	ResetAt     int64                      `json:"reset_at"`
	WindowStats *sub2APIAccountWindowStats `json:"window_stats,omitempty"`
}

type sub2APIAccountUsage struct {
	RateLimit struct {
		LimitReached    bool                `json:"limit_reached"`
		PrimaryWindow   *sub2APIUsageWindow `json:"primary_window,omitempty"`
		SecondaryWindow *sub2APIUsageWindow `json:"secondary_window,omitempty"`
	} `json:"rate_limit"`
	Error string `json:"error,omitempty"`
}

type sub2APIUpstreamUsageResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		FiveHour         *sub2APIUpstreamUsageProgress `json:"five_hour"`
		SevenDay         *sub2APIUpstreamUsageProgress `json:"seven_day"`
		SubscriptionTier string                        `json:"subscription_tier"`
		Error            string                        `json:"error"`
	} `json:"data"`
}

type sub2APIUpstreamUsageProgress struct {
	Utilization float64                    `json:"utilization"`
	ResetsAt    *time.Time                 `json:"resets_at"`
	WindowStats *sub2APIAccountWindowStats `json:"window_stats"`
}

type sub2APIOpenAIQuotaResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		PlanType  string `json:"plan_type"`
		RateLimit *struct {
			LimitReached    bool                      `json:"limit_reached"`
			PrimaryWindow   *sub2APIOpenAIQuotaWindow `json:"primary_window"`
			SecondaryWindow *sub2APIOpenAIQuotaWindow `json:"secondary_window"`
		} `json:"rate_limit"`
	} `json:"data"`
}

type sub2APIOpenAIQuotaWindow struct {
	UsedPercent       float64 `json:"used_percent"`
	LimitWindowSecond int64   `json:"limit_window_seconds"`
	ResetAfterSecond  int64   `json:"reset_after_seconds"`
	ResetAt           int64   `json:"reset_at"`
}

func ImportSub2APIAccounts(c *gin.Context) {
	var req sub2APIImportRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请粘贴账号 JSON"})
		return
	}
	if len(req.Content) > maxSub2APIImportSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "内容不能超过 10MB"})
		return
	}

	path, payload, err := buildSub2APIImportPayload(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	body, err := common.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "账号数据序列化失败"})
		return
	}

	responseBody, status, err := doSub2APIRequest(c.Request.Context(), http.MethodPost, path, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	writeSub2APIResponse(c, status, responseBody, "Sub2API 上货完成")
}

func GetSub2APIAccounts(c *gin.Context) {
	query := url.Values{}
	for _, key := range []string{"page", "page_size", "platform", "type", "status", "search", "sort_by", "sort_order"} {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			query.Set(key, value)
		}
	}
	if query.Get("page") == "" {
		query.Set("page", "1")
	}
	if query.Get("page_size") == "" {
		query.Set("page_size", "20")
	}

	responseBody, status, err := doSub2APIRequest(
		c.Request.Context(),
		http.MethodGet,
		"/api/v1/admin/accounts?"+query.Encode(),
		nil,
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	var upstream sub2APIAccountListResponse
	if err := common.Unmarshal(responseBody, &upstream); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Sub2API 响应格式错误"})
		return
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
		writeSub2APIResponse(c, status, responseBody, "success")
		return
	}

	accountIDs := make([]int64, 0, len(upstream.Data.Items))
	assetKeys := make([]string, 0, len(upstream.Data.Items))
	for index := range upstream.Data.Items {
		account := &upstream.Data.Items[index]
		account.Email = strings.TrimSpace(account.Credentials.Email)
		account.PlanType = strings.TrimSpace(account.Credentials.PlanType)
		account.AssetKey = model.OperationalSub2APIAssetKey(account.ID)
		displayName := account.Email
		if displayName == "" {
			displayName = account.Name
		}
		if err := model.UpsertOperationalAssetSeen(&model.OperationalAsset{
			SourceType: model.OperationalAssetTypeSub2APIAccount, SourceKey: account.AssetKey,
			SourceID: account.ID, SourceRef: strconv.FormatInt(account.ID, 10), DisplayName: displayName,
			CreatedBy: c.GetInt("id"), UpdatedBy: c.GetInt("id"),
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取 Sub2API 成本失败"})
			return
		}
		accountIDs = append(accountIDs, account.ID)
		assetKeys = append(assetKeys, account.AssetKey)
	}
	assets, err := model.GetOperationalAssetsByKeys(model.OperationalAssetTypeSub2APIAccount, assetKeys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取 Sub2API 成本失败"})
		return
	}
	visibleItems := upstream.Data.Items[:0]
	for _, account := range upstream.Data.Items {
		asset, ok := assets[account.AssetKey]
		if ok && asset.State == model.OperationalAssetStateArchived {
			upstream.Data.Total--
			continue
		}
		visibleItems = append(visibleItems, account)
	}
	upstream.Data.Items = visibleItems
	accountIDs = accountIDs[:0]
	for _, account := range upstream.Data.Items {
		accountIDs = append(accountIDs, account.ID)
	}

	statsChannel := make(chan map[string]sub2APIAccountWindowStats, 1)
	go func() {
		statsChannel <- getSub2APITodayStats(c.Request.Context(), accountIDs)
	}()
	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, 6)
	for index := range upstream.Data.Items {
		waitGroup.Add(1)
		go func(account *sub2APIAccount) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			loadSub2APIAccountUsage(c.Request.Context(), account)
			loadSub2APIAccountCumulativeOutput(c.Request.Context(), account)
		}(&upstream.Data.Items[index])
	}
	waitGroup.Wait()
	stats := <-statsChannel
	for index := range upstream.Data.Items {
		account := &upstream.Data.Items[index]
		if asset, ok := assets[account.AssetKey]; ok {
			account.CostMinor = asset.CostMinor
			account.CostDate = asset.CostDate
			account.CostNote = asset.CostNote
		}
		if today, ok := stats[strconv.FormatInt(account.ID, 10)]; ok {
			account.TodayStats = &today
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success", "data": upstream.Data})
}

func loadSub2APIAccountCumulativeOutput(ctx context.Context, account *sub2APIAccount) {
	query := url.Values{}
	query.Set("account_id", strconv.FormatInt(account.ID, 10))
	query.Set("start_date", "2000-01-01")
	query.Set("end_date", "2099-12-31")
	query.Set("timezone", "UTC")
	responseBody, status, err := doSub2APIRequest(
		ctx,
		http.MethodGet,
		"/api/v1/admin/usage/stats?"+query.Encode(),
		nil,
	)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		return
	}
	var upstream struct {
		Code int `json:"code"`
		Data struct {
			TotalActualCost float64 `json:"total_actual_cost"`
		} `json:"data"`
	}
	if common.Unmarshal(responseBody, &upstream) != nil || upstream.Code != 0 || upstream.Data.TotalActualCost <= 0 {
		return
	}
	account.CumulativeOutputUSD = upstream.Data.TotalActualCost
}

func SetSub2APIAccountSchedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号 ID 无效"})
		return
	}
	var req struct {
		Schedulable bool `json:"schedulable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号状态无效"})
		return
	}
	body, err := common.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "账号状态序列化失败"})
		return
	}
	responseBody, status, err := doSub2APIRequest(
		c.Request.Context(),
		http.MethodPost,
		fmt.Sprintf("/api/v1/admin/accounts/%d/schedulable", accountID),
		body,
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	writeSub2APIResponse(c, status, responseBody, "账号状态已更新")
}

func DeleteSub2APIAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号 ID 无效"})
		return
	}
	responseBody, status, err := doSub2APIRequest(
		c.Request.Context(),
		http.MethodDelete,
		fmt.Sprintf("/api/v1/admin/accounts/%d", accountID),
		nil,
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	writeSub2APIResponse(c, status, responseBody, "账号已删除")
}

func ArchiveSub2APIAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号 ID 无效"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "归档参数无效"})
		return
	}
	if err := archiveSub2APIAccount(c.Request.Context(), accountID, req.Reason, c.GetInt("id")); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "账号已归档"})
}

func GetSub2APIAccountSelection(c *gin.Context) {
	responseBody, status, err := doSub2APIRequest(
		c.Request.Context(),
		http.MethodGet,
		"/api/v1/admin/accounts?page=1&page_size=1000&sort_by=name&sort_order=asc",
		nil,
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	var upstream sub2APIAccountListResponse
	if err := common.Unmarshal(responseBody, &upstream); err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "读取 Sub2API 账号失败"})
		return
	}
	assetKeys := make([]string, 0, len(upstream.Data.Items))
	for _, account := range upstream.Data.Items {
		assetKeys = append(assetKeys, model.OperationalSub2APIAssetKey(account.ID))
	}
	assets, err := model.GetOperationalAssetsByKeys(model.OperationalAssetTypeSub2APIAccount, assetKeys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "读取 Sub2API 账号失败"})
		return
	}
	ids := make([]int64, 0, len(upstream.Data.Items))
	for _, account := range upstream.Data.Items {
		if asset, ok := assets[model.OperationalSub2APIAssetKey(account.ID)]; ok && asset.State == model.OperationalAssetStateArchived {
			continue
		}
		ids = append(ids, account.ID)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"ids": ids, "total": len(ids)}})
}

func BatchManageSub2APIAccounts(c *gin.Context) {
	var req struct {
		AccountIDs    []int64 `json:"account_ids"`
		Action        string  `json:"action"`
		CostMinor     int64   `json:"cost_minor"`
		CostDate      int64   `json:"cost_date"`
		CostNote      string  `json:"cost_note"`
		ArchiveReason string  `json:"archive_reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AccountIDs) == 0 || len(req.AccountIDs) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "账号选择无效"})
		return
	}
	if req.Action != "enable" && req.Action != "disable" && req.Action != "cost" && req.Action != "archive" && req.Action != "delete" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "批量操作无效"})
		return
	}
	if req.Action == "cost" && req.CostMinor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "成本不能为负数"})
		return
	}

	type result struct {
		ID    int64
		Error string
	}
	results := make([]result, len(req.AccountIDs))
	semaphore := make(chan struct{}, 6)
	var waitGroup sync.WaitGroup
	requestContext := c.Request.Context()
	userID := c.GetInt("id")
	for index, accountID := range req.AccountIDs {
		if accountID <= 0 {
			results[index] = result{ID: accountID, Error: "账号 ID 无效"}
			continue
		}
		waitGroup.Add(1)
		go func(resultIndex int, id int64) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			if req.Action == "cost" {
				asset := &model.OperationalAsset{
					SourceType: model.OperationalAssetTypeSub2APIAccount,
					SourceKey:  model.OperationalSub2APIAssetKey(id), SourceID: id,
					SourceRef: strconv.FormatInt(id, 10), DisplayName: strconv.FormatInt(id, 10),
					CreatedBy: userID, UpdatedBy: userID,
				}
				if err := model.SetOperationalAssetCost(asset, req.CostMinor, req.CostDate, req.CostNote, userID); err != nil {
					results[resultIndex] = result{ID: id, Error: "成本保存失败"}
					return
				}
				results[resultIndex] = result{ID: id}
				return
			}
			if req.Action == "archive" {
				if err := archiveSub2APIAccount(requestContext, id, req.ArchiveReason, userID); err != nil {
					results[resultIndex] = result{ID: id, Error: err.Error()}
					return
				}
				results[resultIndex] = result{ID: id}
				return
			}

			method := http.MethodPost
			path := fmt.Sprintf("/api/v1/admin/accounts/%d/schedulable", id)
			var body []byte
			if req.Action == "delete" {
				method = http.MethodDelete
				path = fmt.Sprintf("/api/v1/admin/accounts/%d", id)
			} else {
				payload, marshalErr := common.Marshal(map[string]bool{"schedulable": req.Action == "enable"})
				if marshalErr != nil {
					results[resultIndex] = result{ID: id, Error: "账号状态序列化失败"}
					return
				}
				body = payload
			}
			responseBody, status, requestErr := doSub2APIRequest(requestContext, method, path, body)
			if requestErr != nil {
				results[resultIndex] = result{ID: id, Error: requestErr.Error()}
				return
			}
			var upstream sub2APIResponse
			if common.Unmarshal(responseBody, &upstream) != nil || status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
				message := strings.TrimSpace(upstream.Message)
				if message == "" {
					message = "Sub2API 操作失败"
				}
				results[resultIndex] = result{ID: id, Error: message}
				return
			}
			results[resultIndex] = result{ID: id}
		}(index, accountID)
	}
	waitGroup.Wait()

	succeeded := make([]int64, 0, len(results))
	failed := make([]gin.H, 0)
	for _, item := range results {
		if item.Error == "" {
			succeeded = append(succeeded, item.ID)
		} else {
			failed = append(failed, gin.H{"id": item.ID, "error": item.Error})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"succeeded": succeeded, "failed": failed},
	})
}

func getSub2APITodayStats(ctx context.Context, accountIDs []int64) map[string]sub2APIAccountWindowStats {
	stats := make(map[string]sub2APIAccountWindowStats)
	if len(accountIDs) == 0 {
		return stats
	}
	body, err := common.Marshal(map[string]any{"account_ids": accountIDs})
	if err != nil {
		return stats
	}
	responseBody, status, err := doSub2APIRequest(ctx, http.MethodPost, "/api/v1/admin/accounts/today-stats/batch", body)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		return stats
	}
	var upstream struct {
		Code int `json:"code"`
		Data struct {
			Stats map[string]sub2APIAccountWindowStats `json:"stats"`
		} `json:"data"`
	}
	if common.Unmarshal(responseBody, &upstream) != nil || upstream.Code != 0 {
		return stats
	}
	return upstream.Data.Stats
}

func listAllSub2APIAccounts(ctx context.Context) ([]sub2APIAccount, error) {
	responseBody, status, err := doSub2APIRequest(ctx, http.MethodGet, "/api/v1/admin/accounts?page=1&page_size=1000&sort_by=created_at&sort_order=desc", nil)
	if err != nil {
		return nil, err
	}
	var upstream sub2APIAccountListResponse
	if common.Unmarshal(responseBody, &upstream) != nil || status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
		return nil, fmt.Errorf("读取 Sub2API 账号失败")
	}
	return upstream.Data.Items, nil
}

func getSub2APIAccount(ctx context.Context, accountID int64) (*sub2APIAccount, error) {
	responseBody, status, err := doSub2APIRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts/%d", accountID), nil)
	if err != nil {
		return nil, err
	}
	var upstream sub2APIAccountResponse
	if common.Unmarshal(responseBody, &upstream) != nil || status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
		return nil, fmt.Errorf("Sub2API 账号不存在")
	}
	return &upstream.Data, nil
}

func archiveSub2APIAccount(ctx context.Context, accountID int64, reason string, userID int) error {
	account, err := getSub2APIAccount(ctx, accountID)
	if err != nil {
		return err
	}
	loadSub2APIAccountCumulativeOutput(ctx, account)
	body, err := common.Marshal(map[string]bool{"schedulable": false})
	if err != nil {
		return fmt.Errorf("账号状态序列化失败")
	}
	responseBody, status, err := doSub2APIRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/admin/accounts/%d/schedulable", accountID), body)
	if err != nil {
		return err
	}
	var upstream sub2APIResponse
	if common.Unmarshal(responseBody, &upstream) != nil || status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
		return fmt.Errorf("冻结账号失败，未完成归档")
	}
	email := strings.TrimSpace(account.Credentials.Email)
	if email == "" {
		email = account.Name
	}
	snapshot, err := common.Marshal(map[string]any{
		"id": account.ID, "name": account.Name, "email": email,
		"platform": account.Platform, "type": account.Type, "status": account.Status,
		"plan_type": account.Credentials.PlanType, "cumulative_output_usd": account.CumulativeOutputUSD,
	})
	if err != nil {
		return err
	}
	return model.ArchiveOperationalAsset(&model.OperationalAsset{
		SourceType: model.OperationalAssetTypeSub2APIAccount,
		SourceKey:  model.OperationalSub2APIAssetKey(account.ID),
		SourceID:   account.ID, SourceRef: strconv.FormatInt(account.ID, 10), DisplayName: email,
		CreatedBy: userID, UpdatedBy: userID,
	}, string(snapshot), reason, userID)
}

func loadSub2APIAccountUsage(ctx context.Context, account *sub2APIAccount) {
	account.Usage = &sub2APIAccountUsage{}
	if account.Platform == "openai" && account.Type == "oauth" {
		responseBody, status, err := doSub2APIRequest(
			ctx,
			http.MethodGet,
			fmt.Sprintf("/api/v1/admin/openai/accounts/%d/quota", account.ID),
			nil,
		)
		if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
			account.Usage.Error = "额度查询失败"
			return
		}
		var upstream sub2APIOpenAIQuotaResponse
		if common.Unmarshal(responseBody, &upstream) != nil || upstream.Code != 0 {
			account.Usage.Error = "额度查询失败"
			return
		}
		if upstream.Data.PlanType != "" {
			account.PlanType = upstream.Data.PlanType
		}
		if upstream.Data.RateLimit == nil {
			return
		}
		account.Usage.RateLimit.LimitReached = upstream.Data.RateLimit.LimitReached
		setSub2APIQuotaWindow(account.Usage, upstream.Data.RateLimit.PrimaryWindow, true)
		setSub2APIQuotaWindow(account.Usage, upstream.Data.RateLimit.SecondaryWindow, false)
		return
	}

	responseBody, status, err := doSub2APIRequest(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/api/v1/admin/accounts/%d/usage", account.ID),
		nil,
	)
	if err != nil || status < http.StatusOK || status >= http.StatusMultipleChoices {
		account.Usage.Error = "额度查询失败"
		return
	}
	var upstream sub2APIUpstreamUsageResponse
	if common.Unmarshal(responseBody, &upstream) != nil || upstream.Code != 0 {
		account.Usage.Error = "额度查询失败"
		return
	}
	if upstream.Data.SubscriptionTier != "" {
		account.PlanType = upstream.Data.SubscriptionTier
	}
	account.Usage.Error = upstream.Data.Error
	account.Usage.RateLimit.PrimaryWindow = makeSub2APIUsageWindow(upstream.Data.FiveHour)
	account.Usage.RateLimit.SecondaryWindow = makeSub2APIUsageWindow(upstream.Data.SevenDay)
	account.Usage.RateLimit.LimitReached = usageWindowReached(account.Usage.RateLimit.PrimaryWindow) || usageWindowReached(account.Usage.RateLimit.SecondaryWindow)
}

func setSub2APIQuotaWindow(usage *sub2APIAccountUsage, window *sub2APIOpenAIQuotaWindow, primary bool) {
	if window == nil {
		return
	}
	resetAt := window.ResetAt
	if resetAt == 0 && window.ResetAfterSecond > 0 {
		resetAt = time.Now().Add(time.Duration(window.ResetAfterSecond) * time.Second).Unix()
	}
	normalized := &sub2APIUsageWindow{UsedPercent: window.UsedPercent, ResetAt: resetAt}
	if window.LimitWindowSecond >= int64(6*24*time.Hour/time.Second) {
		usage.RateLimit.SecondaryWindow = normalized
	} else if window.LimitWindowSecond > 0 {
		usage.RateLimit.PrimaryWindow = normalized
	} else if primary {
		usage.RateLimit.PrimaryWindow = normalized
	} else {
		usage.RateLimit.SecondaryWindow = normalized
	}
	if window.UsedPercent >= 100 {
		usage.RateLimit.LimitReached = true
	}
}

func makeSub2APIUsageWindow(progress *sub2APIUpstreamUsageProgress) *sub2APIUsageWindow {
	if progress == nil {
		return nil
	}
	window := &sub2APIUsageWindow{UsedPercent: progress.Utilization, WindowStats: progress.WindowStats}
	if progress.ResetsAt != nil {
		window.ResetAt = progress.ResetsAt.Unix()
	}
	return window
}

func usageWindowReached(window *sub2APIUsageWindow) bool {
	return window != nil && window.UsedPercent >= 100
}

func buildSub2APIImportPayload(content string) (string, any, error) {
	var value any
	if err := common.UnmarshalJsonStr(content, &value); err != nil {
		return "", nil, fmt.Errorf("账号 JSON 格式无效")
	}

	if document, ok := value.(map[string]any); ok {
		if data, ok := document["data"].(map[string]any); ok && data["accounts"] != nil {
			return "/api/v1/admin/accounts/data", document, nil
		}
		if document["accounts"] != nil {
			return "/api/v1/admin/accounts/data", map[string]any{
				"data": document, "skip_default_group_bind": false,
			}, nil
		}
		if document["platform"] != nil && document["credentials"] != nil {
			return "/api/v1/admin/accounts/data", newSub2APIDataImport([]any{document}), nil
		}
	}

	if accounts, ok := value.([]any); ok && len(accounts) > 0 {
		nativeAccounts := true
		for _, item := range accounts {
			account, ok := item.(map[string]any)
			if !ok || account["platform"] == nil || account["credentials"] == nil {
				nativeAccounts = false
				break
			}
		}
		if nativeAccounts {
			return "/api/v1/admin/accounts/data", newSub2APIDataImport(accounts), nil
		}
	}

	return "/api/v1/admin/accounts/import/codex-session", map[string]any{
		"content": content, "update_existing": true, "skip_default_group_bind": false,
	}, nil
}

func newSub2APIDataImport(accounts []any) map[string]any {
	return map[string]any{
		"data": map[string]any{
			"type": "sub2api-data", "version": 1,
			"exported_at": time.Now().UTC().Format(time.RFC3339),
			"proxies":     []any{}, "accounts": accounts,
		},
		"skip_default_group_bind": false,
	}
}

func writeSub2APIResponse(c *gin.Context, status int, body []byte, successMessage string) {
	var upstream sub2APIResponse
	if err := common.Unmarshal(body, &upstream); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Sub2API 响应格式错误"})
		return
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices || upstream.Code != 0 {
		message := strings.TrimSpace(upstream.Message)
		if message == "" {
			message = fmt.Sprintf("Sub2API 返回 HTTP %d", status)
		}
		if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
			status = http.StatusBadGateway
		}
		c.JSON(status, gin.H{"success": false, "message": message, "data": upstream.Data})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": successMessage, "data": upstream.Data})
}

func doSub2APIRequest(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	adminKey, err := readSub2APIAdminKey()
	if err != nil {
		return nil, 0, fmt.Errorf("Sub2API 管理密钥不可用")
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SUB2API_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultSub2APIBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 Sub2API 请求失败")
	}
	req.Header.Set("x-api-key", adminKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("Sub2API 不可达")
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("读取 Sub2API 响应失败")
	}
	return responseBody, resp.StatusCode, nil
}

func readSub2APIAdminKey() (string, error) {
	path := strings.TrimSpace(os.Getenv("SUB2API_ADMIN_KEY_FILE"))
	if path == "" {
		path = "/etc/new-api/sub2api-admin-key"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("empty admin key")
	}
	return key, nil
}
