package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
)

func TestApplyUsagePostProcessingNormalizesCacheWriteTokens(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens: 1000,
		PromptTokensDetails: dto.InputTokenDetails{
			CacheWriteTokens: 250,
		},
	}

	applyUsagePostProcessing(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
	}, usage, nil)

	assert.Equal(t, 250, usage.PromptTokensDetails.CachedCreationTokens)
}
