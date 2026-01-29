package pkg

import (
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/probe-lab/go-commons/ptr"
)

func ParseCacheStatus(header network.Headers) *string {
	if v, found := getHeaderValue[string](header, "cf-cache-status"); found && v != "" {
		status := strings.ToUpper(strings.TrimSpace(v))
		switch status {
		case "HIT", "STALE", "UPDATING":
			return ptr.From("HIT")

		case "MISS", "EXPIRED", "REVALIDATED", "DYNAMIC", "BYPASS":
			return ptr.From("MISS")

		default:
			return &v
		}
	} else if v, found := getHeaderValue[string](header, "x-filebase-edge-cache"); found && v != "" {
		return &v
	} else if v, found := getHeaderValue[string](header, "x-cache"); found && v != "" {
		return ptr.From(strings.ToUpper(strings.Split(v, " ")[0]))
	}

	return nil
}

func getHeaderValue[T any](header network.Headers, key string) (T, bool) {
	value, found := header[key]
	if !found {
		return *new(T), false
	}

	typed, ok := value.(T)
	if !ok {
		return *new(T), false
	}

	return typed, true
}
