package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type operationalAssetCostRequest struct {
	SourceType  string `json:"source_type"`
	SourceKey   string `json:"source_key"`
	SourceID    int64  `json:"source_id"`
	SourceRef   string `json:"source_ref"`
	DisplayName string `json:"display_name"`
	CostMinor   int64  `json:"cost_minor"`
	CostDate    int64  `json:"cost_date"`
	Note        string `json:"note"`
}

type operationalCostEntryRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	AmountMinor int64  `json:"amount_minor"`
	OccurredAt  int64  `json:"occurred_at"`
	Note        string `json:"note"`
}

type archiveAssetRequest struct {
	Reason string `json:"reason"`
}

type operationalArchiveItem struct {
	ID                  int64    `json:"id"`
	SourceType          string   `json:"source_type"`
	SourceKey           string   `json:"source_key"`
	SourceID            int64    `json:"source_id"`
	DisplayName         string   `json:"display_name"`
	State               string   `json:"state"`
	CostMinor           int64    `json:"cost_minor"`
	Currency            string   `json:"currency"`
	CostDate            int64    `json:"cost_date"`
	CostNote            string   `json:"cost_note"`
	ArchivedAt          int64    `json:"archived_at"`
	ArchiveReason       string   `json:"archive_reason"`
	CumulativeOutputUSD *float64 `json:"cumulative_output_usd"`
}

func makeOperationalArchiveItems(archives []model.OperationalAsset) []operationalArchiveItem {
	cpaOutputs := readCPAAccountOutputs()
	items := make([]operationalArchiveItem, 0, len(archives))
	for _, asset := range archives {
		var outputUSD *float64
		switch asset.SourceType {
		case model.OperationalAssetTypeCPAAccount:
			if value, ok := cpaOutputs[asset.SourceKey]; ok {
				outputUSD = &value
			}
		case model.OperationalAssetTypeSub2APIAccount:
			var snapshot struct {
				CumulativeOutputUSD *float64 `json:"cumulative_output_usd"`
			}
			if common.UnmarshalJsonStr(asset.Snapshot, &snapshot) == nil {
				outputUSD = snapshot.CumulativeOutputUSD
			}
		}
		if outputUSD != nil && (*outputUSD < 0 || math.IsNaN(*outputUSD) || math.IsInf(*outputUSD, 0)) {
			outputUSD = nil
		}
		items = append(items, operationalArchiveItem{
			ID: asset.ID, SourceType: asset.SourceType, SourceKey: asset.SourceKey,
			SourceID: asset.SourceID, DisplayName: asset.DisplayName, State: asset.State,
			CostMinor: asset.CostMinor, Currency: asset.Currency, CostDate: asset.CostDate,
			CostNote: asset.CostNote, ArchivedAt: asset.ArchivedAt, ArchiveReason: asset.ArchiveReason,
			CumulativeOutputUSD: outputUSD,
		})
	}
	return items
}

func GetOperationalCostOverview(c *gin.Context) {
	summary, err := model.GetOperationalCostSummary()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	entries, err := model.ListOperationalCostEntries()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	archives, err := model.ListArchivedOperationalAssets()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"summary":  summary,
		"entries":  entries,
		"archives": makeOperationalArchiveItems(archives),
		"currency": "CNY",
	})
}

func GetOperationalCostSummary(c *gin.Context) {
	summary, err := model.GetOperationalCostSummary()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"summary": summary, "currency": "CNY"})
}

func ListOperationalCostEntries(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	entries, total, err := model.ListOperationalCostEntriesPage(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(entries)
	common.ApiSuccess(c, pageInfo)
}

func ListOperationalCostArchives(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	sourceType := strings.TrimSpace(c.Query("source_type"))
	if sourceType != "" && sourceType != model.OperationalAssetTypeCPAAccount && sourceType != model.OperationalAssetTypeSub2APIAccount && sourceType != model.OperationalAssetTypeChannel {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid archive source type"})
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "archive search is too long"})
		return
	}
	archives, total, err := model.ListArchivedOperationalAssetsPage(pageInfo, sourceType, search)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(makeOperationalArchiveItems(archives))
	common.ApiSuccess(c, pageInfo)
}

func ReconcileOperationalCostAssets(c *gin.Context) {
	if err := model.ReconcileOperationalAssetsFromLocal(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func SetOperationalAssetCost(c *gin.Context) {
	var req operationalAssetCostRequest
	if c.ShouldBindJSON(&req) != nil || req.CostMinor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset cost"})
		return
	}
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceKey = strings.TrimSpace(req.SourceKey)
	if req.SourceKey == "" || (req.SourceType != model.OperationalAssetTypeCPAAccount && req.SourceType != model.OperationalAssetTypeSub2APIAccount && req.SourceType != model.OperationalAssetTypeChannel) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset source"})
		return
	}
	if req.SourceType == model.OperationalAssetTypeChannel {
		channel, err := model.GetChannelById(int(req.SourceID), false)
		if err != nil || channel == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "channel not found"})
			return
		}
		req.SourceKey = model.OperationalChannelAssetKey(channel.Id)
		req.DisplayName = channel.Name
	}
	asset := &model.OperationalAsset{
		SourceType:  req.SourceType,
		SourceKey:   req.SourceKey,
		SourceID:    req.SourceID,
		SourceRef:   strings.TrimSpace(req.SourceRef),
		DisplayName: strings.TrimSpace(req.DisplayName),
		CreatedBy:   c.GetInt("id"),
		UpdatedBy:   c.GetInt("id"),
	}
	if err := model.SetOperationalAssetCost(asset, req.CostMinor, req.CostDate, req.Note, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func CreateOperationalCostEntry(c *gin.Context) {
	var req operationalCostEntryRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "title is required"})
		return
	}
	entry := &model.OperationalCostEntry{
		Title:       strings.TrimSpace(req.Title),
		Category:    strings.TrimSpace(req.Category),
		AmountMinor: req.AmountMinor,
		OccurredAt:  req.OccurredAt,
		Note:        strings.TrimSpace(req.Note),
		CreatedBy:   c.GetInt("id"),
		UpdatedBy:   c.GetInt("id"),
	}
	if err := model.CreateOperationalCostEntry(entry); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, entry)
}

func UpdateOperationalCostEntry(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid entry id"})
		return
	}
	var req operationalCostEntryRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "title is required"})
		return
	}
	entry := &model.OperationalCostEntry{
		ID:          id,
		Title:       req.Title,
		Category:    req.Category,
		AmountMinor: req.AmountMinor,
		OccurredAt:  req.OccurredAt,
		Note:        req.Note,
	}
	if err := model.UpdateOperationalCostEntry(entry, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeleteOperationalCostEntry(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid entry id"})
		return
	}
	if err := model.VoidOperationalCostEntry(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ArchiveChannelAsset(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid channel id"})
		return
	}
	var req archiveAssetRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil || channel == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "channel not found"})
		return
	}
	snapshot, err := common.Marshal(map[string]any{
		"id": channel.Id, "name": channel.Name, "type": channel.Type,
		"base_url": channel.BaseURL, "models": channel.Models, "group": channel.Group,
		"tag": channel.Tag, "remark": channel.Remark, "status": channel.Status,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "operational archive")
	asset := &model.OperationalAsset{
		SourceType: model.OperationalAssetTypeChannel, SourceKey: model.OperationalChannelAssetKey(channel.Id),
		SourceID: int64(channel.Id), DisplayName: channel.Name,
	}
	if err := model.ArchiveOperationalAsset(asset, string(snapshot), req.Reason, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DisableActivationSource(model.ActivationSourceCodexChannel, strconv.Itoa(channel.Id)); err != nil {
		common.SysError("failed to disable archived channel activation target: " + err.Error())
	}
	common.ApiSuccess(c, nil)
}

func RestoreOperationalAsset(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset id"})
		return
	}
	asset, err := model.RestoreOperationalAsset(id, c.GetInt("id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "archive not found"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if asset.SourceType == model.OperationalAssetTypeChannel && asset.SourceID > 0 {
		model.UpdateChannelStatus(int(asset.SourceID), "", common.ChannelStatusManuallyDisabled, "restored from operational archive")
	}
	common.ApiSuccess(c, asset)
}
