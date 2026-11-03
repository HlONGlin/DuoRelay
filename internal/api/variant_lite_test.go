//go:build lite

package api

import (
	"strings"
	"testing"
)

func TestLiteCapabilityAndRawConfigBoundary(t *testing.T) {
	s := apiForTest(t)
	result := call(t, s, "GET", "/api/auth/variant", "", 200)
	if !strings.Contains(string(result), `"variant":"lite"`) || strings.Contains(string(result), "sing-box") {
		t.Fatal(string(result))
	}
	call(t, s, "POST", "/api/config/apply", `{"kernel":"mihomo","config":"proxies: []"}`, 400)
	call(t, s, "POST", "/api/config/apply", `{"kernel":"sing-box"}`, 400)
}
