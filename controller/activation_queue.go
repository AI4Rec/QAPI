package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type activationQueuePauseRequest struct {
	Paused bool `json:"paused"`
}

type activationTargetStatusRequest struct {
	Enabled bool `json:"enabled"`
}

type activationJobSnoozeRequest struct {
	Seconds int64 `json:"seconds"`
}

func GetActivationQueueOverview(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	overview, err := service.GetActivationQueueOverview(limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": overview})
}

func ReconcileActivationQueue(c *gin.Context) {
	summary, err := service.ReconcileActivationQueue(c.Request.Context())
	message := ""
	if err != nil {
		message = err.Error()
	}
	c.JSON(http.StatusOK, gin.H{
		"success": err == nil,
		"message": message,
		"data":    summary,
	})
}

func SetActivationQueuePaused(c *gin.Context) {
	request := activationQueuePauseRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	if err := service.SetActivationQueuePaused(request.Paused); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "queue status updated"})
}

func SetActivationTargetEnabled(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid target id"})
		return
	}
	request := activationTargetStatusRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	if err := service.SetActivationTargetEnabled(id, request.Enabled); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "target status updated"})
}

func RunActivationJobNow(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid job id"})
		return
	}
	if err := service.RunActivationJobNow(id); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "job queued for immediate execution"})
}

func SnoozeActivationJob(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid job id"})
		return
	}
	request := activationJobSnoozeRequest{}
	if err := c.ShouldBindJSON(&request); err != nil || request.Seconds <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid snooze duration"})
		return
	}
	if err := service.SnoozeActivationJob(id, time.Duration(request.Seconds)*time.Second); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "job snoozed"})
}

func SkipActivationJob(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid job id"})
		return
	}
	if err := service.SkipActivationJob(id); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "job skipped"})
}
