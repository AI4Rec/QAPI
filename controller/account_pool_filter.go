package controller

import "strings"

const (
	accountPoolFilterAll               = "all"
	accountPoolFilterInsufficientQuota = "insufficient_quota"
	accountPoolFilterForbidden         = "forbidden"
	accountPoolFilterPaused            = "paused"
)

var accountPoolInsufficientQuotaMarkers = []string{
	"insufficient quota",
	"quota insufficient",
	"limit reached",
	"额度不足",
}

var accountPoolForbiddenMarkers = []string{
	"forbidden",
	"403",
	"禁止访问",
}

var accountPoolPausedMarkers = []string{
	"paused",
	"暂停",
	"已暂停",
}

func normalizeAccountPoolFilter(filter string) (string, bool) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return accountPoolFilterAll, true
	}
	switch filter {
	case accountPoolFilterAll, accountPoolFilterInsufficientQuota, accountPoolFilterForbidden, accountPoolFilterPaused:
		return filter, true
	default:
		return "", false
	}
}

func accountPoolFilterNeedsUsage(filter string) bool {
	return filter == accountPoolFilterInsufficientQuota || filter == accountPoolFilterForbidden
}

func accountPoolTextIncludesAnyMarker(values []string, markers []string) bool {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		for _, marker := range markers {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	}
	return false
}

func accountPoolUsageLimitReached(usage map[string]any) bool {
	rateLimit, ok := usage["rate_limit"].(map[string]any)
	if !ok {
		return false
	}
	reached, _ := rateLimit["limit_reached"].(bool)
	return reached
}
