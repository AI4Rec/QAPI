package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const responsesRequestPayloadReleasedKey = "responses_request_payload_released"

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	configuredPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled
	fastPassThrough := canFastPassThroughResponses(c, info, responsesReq)
	if configuredPassThrough || fastPassThrough {
		if err := helper.ModelMappedHelper(c, info, nil); err != nil {
			return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
		}
		if fastPassThrough && len(info.ParamOverride) > 0 {
			if err := relaycommon.ApplyHeaderOnlyParamOverrideWithRelayInfo(info); err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}
		if responsesReq.Reasoning != nil {
			info.ReasoningEffort = responsesReq.Reasoning.Effort
		}
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		info.UpstreamRequestBodySize = storage.Size()
		requestBody = common.ReaderOnly(storage)
		if _, ok := info.Request.(*dto.OpenAIResponsesRequest); ok {
			releaseResponsesRequestPayload(responsesReq)
			c.Set(responsesRequestPayloadReleasedKey, true)
		}
	} else {
		if c.GetBool(responsesRequestPayloadReleasedKey) {
			responsesReq = &dto.OpenAIResponsesRequest{}
			if err := common.UnmarshalBodyReusable(c, responsesReq); err != nil {
				return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
			}
			info.Request = responsesReq
			c.Set(responsesRequestPayloadReleasedKey, false)
		}
		request, err := common.DeepCopy(responsesReq)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		if err := helper.ModelMappedHelper(c, info, request); err != nil {
			return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
		}
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		logger.LogDebug(c, "requestBody: %s", jsonData)
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		jsonData = nil
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
	} else {
		service.PostTextConsumeQuota(c, info, usageDto, nil)
	}
	return nil
}

func canFastPassThroughResponses(c *gin.Context, info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) bool {
	if !appconstant.ResponsesFastPathEnabled || info.RelayMode != relayconstant.RelayModeResponses {
		return false
	}
	if info.ApiType != appconstant.APITypeOpenAI || info.ChannelType != appconstant.ChannelTypeOpenAI {
		return false
	}
	if !relaycommon.IsHeaderOnlyParamOverride(info.ParamOverride) {
		return false
	}
	modelMapping := strings.TrimSpace(c.GetString("model_mapping"))
	if modelMapping != "" && modelMapping != "{}" {
		return false
	}
	if effort, _ := reasoning.ParseOpenAIReasoningEffortFromModelSuffix(request.Model); effort != "" {
		return false
	}
	settings := info.ChannelOtherSettings
	if (!settings.AllowServiceTier && request.ServiceTier != "") ||
		(!settings.AllowInferenceGeo && len(request.InferenceGeo) > 0) ||
		(!settings.AllowSpeed && len(request.Speed) > 0) ||
		(settings.DisableStore && len(request.Store) > 0) ||
		(!settings.AllowSafetyIdentifier && len(request.SafetyIdentifier) > 0) ||
		(!settings.AllowIncludeObfuscation && request.StreamOptions != nil && request.StreamOptions.IncludeObfuscation != nil) {
		return false
	}
	return true
}

func releaseResponsesRequestPayload(request *dto.OpenAIResponsesRequest) {
	request.Input = nil
	request.Include = nil
	request.Conversation = nil
	request.ContextManagement = nil
	request.Instructions = nil
	request.Metadata = nil
	request.ParallelToolCalls = nil
	request.PromptCacheKey = nil
	request.PromptCacheRetention = nil
	request.Text = nil
	request.ToolChoice = nil
	request.Tools = nil
	request.Truncation = nil
	request.User = nil
	request.Prompt = nil
	request.EnableThinking = nil
	request.Preset = nil
}
