//go:build linux

package transparent

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRootDoesNotBypassEffectiveCapabilities(t *testing.T) {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	var caps uint64
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "CapEff:") {
			caps, err = strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "CapEff:")), 16, 64)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if hasNetAdmin() != (caps&(1<<12) != 0) || hasNetRaw() != (caps&(1<<13) != 0) {
		t.Fatal("effective capability check ignored process restrictions")
	}
	if caps&(1<<12) == 0 && New().Preflight(ModeTUN, 1080, 1081) == nil {
		t.Fatal("TUN permitted without NET_ADMIN")
	}
	if err := New().Cleanup(); err != nil {
		t.Fatal("inactive cleanup attempted privileged firewall operations", err)
	}
}
