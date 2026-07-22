package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanFastPassThroughResponses(t *testing.T) {
	originalEnabled := appconstant.ResponsesFastPathEnabled
	t.Cleanup(func() { appconstant.ResponsesFastPathEnabled = originalEnabled })
	appconstant.ResponsesFastPathEnabled = true

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5", Input: json.RawMessage(`"hello"`)}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:     appconstant.APITypeOpenAI,
			ChannelType: appconstant.ChannelTypeOpenAI,
		},
	}

	require.True(t, canFastPassThroughResponses(c, info, request))
	request.SafetyIdentifier = json.RawMessage(`"user-1"`)
	assert.False(t, canFastPassThroughResponses(c, info, request))
	request.SafetyIdentifier = nil
	c.Set("model_mapping", `{"gpt-5":"gpt-5.1"}`)
	assert.False(t, canFastPassThroughResponses(c, info, request))
	c.Set("model_mapping", "")
	info.ParamOverride = map[string]any{
		"operations": []any{map[string]any{"mode": "pass_headers", "value": []any{"Originator"}}},
	}
	assert.True(t, canFastPassThroughResponses(c, info, request))
}

func TestReleaseResponsesRequestPayloadKeepsRoutingAndSafetyFields(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Model:            "gpt-5",
		Input:            json.RawMessage(`"hello"`),
		Tools:            json.RawMessage(`[{"type":"web_search_preview"}]`),
		ServiceTier:      "default",
		SafetyIdentifier: json.RawMessage(`"user-1"`),
	}
	releaseResponsesRequestPayload(request)
	assert.Equal(t, "gpt-5", request.Model)
	assert.Equal(t, "default", request.ServiceTier)
	assert.NotEmpty(t, request.SafetyIdentifier)
	assert.Nil(t, request.Input)
	assert.Nil(t, request.Tools)
}
