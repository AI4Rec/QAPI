package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type componentUpdateTaskPayload struct {
	Action        string `json:"action"`
	Component     string `json:"component"`
	TargetVersion string `json:"target_version,omitempty"`
}

type componentUpdateTaskState struct {
	Component string `json:"component"`
	Stage     string `json:"stage"`
	Progress  int    `json:"progress"`
}

type componentUpdateTaskResult struct {
	Action        string `json:"action"`
	Component     string `json:"component"`
	TargetVersion string `json:"target_version,omitempty"`
	Output        string `json:"output,omitempty"`
}

func GetComponentUpdateStatus(c *gin.Context) {
	component := c.Param("component")
	if !service.IsManagedComponent(component) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported component"})
		return
	}

	status, err := service.GetComponentReleaseStatus(c.Request.Context(), component, c.Query("refresh") == "true")
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var activeTask any
	task, err := model.GetActiveSystemTask(model.SystemTaskTypeComponentUpdate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if task != nil {
		payload := componentUpdateTaskPayload{}
		if task.DecodePayload(&payload) == nil && payload.Component == component {
			activeTask = task.ToResponse()
		}
	}

	common.ApiSuccess(c, gin.H{
		"status":      status,
		"active_task": activeTask,
	})
}

func StartComponentUpdate(c *gin.Context) {
	component := c.Param("component")
	if !service.IsManagedComponent(component) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported component"})
		return
	}

	status, err := service.GetComponentReleaseStatus(c.Request.Context(), component, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !status.UpdaterAvailable {
		common.ApiErrorMsg(c, "component updater is not installed")
		return
	}
	if !status.UpdateAvailable {
		common.ApiErrorMsg(c, "component is already up to date")
		return
	}

	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeComponentUpdate, componentUpdateTaskPayload{
		Action:        service.ComponentUpdateActionApply,
		Component:     component,
		TargetVersion: status.LatestVersion,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !created && !componentUpdateTaskMatches(task, component) {
		common.ApiErrorMsg(c, "another component update is already running")
		return
	}
	common.ApiSuccess(c, gin.H{"task": task.ToResponse(), "created": created})
}

func StartComponentRollback(c *gin.Context) {
	component := c.Param("component")
	if !service.IsManagedComponent(component) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported component"})
		return
	}

	status, err := service.GetComponentReleaseStatus(c.Request.Context(), component, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !status.UpdaterAvailable {
		common.ApiErrorMsg(c, "component updater is not installed")
		return
	}
	if !status.RollbackAvailable {
		common.ApiErrorMsg(c, "no rollback version is available")
		return
	}

	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeComponentUpdate, componentUpdateTaskPayload{
		Action:    service.ComponentUpdateActionRollback,
		Component: component,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !created && !componentUpdateTaskMatches(task, component) {
		common.ApiErrorMsg(c, "another component update is already running")
		return
	}
	common.ApiSuccess(c, gin.H{"task": task.ToResponse(), "created": created})
}

func componentUpdateTaskMatches(task *model.SystemTask, component string) bool {
	payload := componentUpdateTaskPayload{}
	return task != nil && task.DecodePayload(&payload) == nil && payload.Component == component
}

type componentUpdateHandler struct{}

func (componentUpdateHandler) Type() string { return model.SystemTaskTypeComponentUpdate }

func (componentUpdateHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	payload := componentUpdateTaskPayload{}
	if err := task.DecodePayload(&payload); err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}
	if !service.IsManagedComponent(payload.Component) {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, errors.New("unsupported component"))
		return
	}

	state := componentUpdateTaskState{Component: payload.Component, Stage: "preparing", Progress: 10}
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		return
	}
	state.Stage = "installing"
	state.Progress = 30
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		return
	}

	output, err := service.RunComponentUpdate(ctx, payload.Action, payload.Component, payload.TargetVersion)
	if err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}

	state.Stage = "completed"
	state.Progress = 100
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		return
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, componentUpdateTaskResult{
		Action:        payload.Action,
		Component:     payload.Component,
		TargetVersion: payload.TargetVersion,
		Output:        output,
	}, nil)
}
