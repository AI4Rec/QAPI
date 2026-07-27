package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAccountPoolFilter(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		wantFilter string
		wantOK     bool
	}{
		{name: "empty defaults to all", filter: "", wantFilter: accountPoolFilterAll, wantOK: true},
		{name: "all", filter: accountPoolFilterAll, wantFilter: accountPoolFilterAll, wantOK: true},
		{name: "insufficient quota", filter: accountPoolFilterInsufficientQuota, wantFilter: accountPoolFilterInsufficientQuota, wantOK: true},
		{name: "forbidden", filter: accountPoolFilterForbidden, wantFilter: accountPoolFilterForbidden, wantOK: true},
		{name: "paused", filter: accountPoolFilterPaused, wantFilter: accountPoolFilterPaused, wantOK: true},
		{name: "unknown", filter: "disabled", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotFilter, gotOK := normalizeAccountPoolFilter(test.filter)
			require.Equal(t, test.wantOK, gotOK)
			assert.Equal(t, test.wantFilter, gotFilter)
		})
	}
}

func TestCPAAccountCandidateMatchesPoolFilter(t *testing.T) {
	candidate := cpaAccountCandidate{
		file: map[string]any{
			"status":         "active",
			"status_message": "API returned 403: forbidden",
		},
	}
	usage := map[string]any{
		"rate_limit": map[string]any{"limit_reached": true},
	}

	assert.True(t, cpaAccountCandidateMatchesPoolFilter(candidate, usage, accountPoolFilterInsufficientQuota))
	assert.True(t, cpaAccountCandidateMatchesPoolFilter(candidate, nil, accountPoolFilterForbidden))
	assert.False(t, cpaAccountCandidateMatchesPoolFilter(candidate, nil, accountPoolFilterPaused))

	candidate.disabled = true
	assert.True(t, cpaAccountCandidateMatchesPoolFilter(candidate, nil, accountPoolFilterPaused))
}

func TestSub2APIAccountMatchesPoolFilter(t *testing.T) {
	account := sub2APIAccount{
		Status:       "active",
		Schedulable:  true,
		ErrorMessage: "API returned 403: forbidden",
		Usage:        &sub2APIAccountUsage{},
	}
	account.Usage.RateLimit.LimitReached = true

	assert.True(t, sub2APIAccountMatchesPoolFilter(account, accountPoolFilterInsufficientQuota))
	assert.True(t, sub2APIAccountMatchesPoolFilter(account, accountPoolFilterForbidden))
	assert.False(t, sub2APIAccountMatchesPoolFilter(account, accountPoolFilterPaused))

	account.Schedulable = false
	assert.True(t, sub2APIAccountMatchesPoolFilter(account, accountPoolFilterPaused))
}

func TestSub2APIAccountPageCount(t *testing.T) {
	assert.Equal(t, 0, sub2APIAccountPageCount(0, 20))
	assert.Equal(t, 1, sub2APIAccountPageCount(1, 20))
	assert.Equal(t, 4, sub2APIAccountPageCount(78, 20))
}
