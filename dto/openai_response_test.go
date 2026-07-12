package dto_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestInputTokenDetailsCacheWriteTokenCount(t *testing.T) {
	tests := []struct {
		name string
		json string
		want int
	}{
		{
			name: "uses cache write token name",
			json: `{"cached_tokens":5888,"cache_write_tokens":1024}`,
			want: 1024,
		},
		{
			name: "supports legacy cache creation token name",
			json: `{"cached_creation_tokens":2048}`,
			want: 2048,
		},
		{
			name: "prefers cache write token name",
			json: `{"cached_creation_tokens":2048,"cache_write_tokens":1024}`,
			want: 1024,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var details dto.InputTokenDetails
			require.NoError(t, common.UnmarshalJsonStr(test.json, &details))
			require.Equal(t, test.want, details.CacheWriteTokenCount())
		})
	}
}
