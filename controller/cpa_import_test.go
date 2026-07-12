package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCPAAccountPoolType(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "imported account hash suffix",
			fileName: "codex-user-example.com-141d741e.json",
			want:     "cpa_import",
		},
		{
			name:     "official login plan suffix",
			fileName: "codex-user@example.com-plus.json",
			want:     "official_login",
		},
		{
			name:     "official login generic name",
			fileName: "codex-user@example.com.json",
			want:     "official_login",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, cpaAccountPoolType(test.fileName))
		})
	}
}
