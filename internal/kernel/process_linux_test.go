//go:build linux

package kernel

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureParentDeath(t *testing.T) {
	cmd := exec.Command("true")
	configureParentDeath(cmd)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Fatalf("parent death signal = %#v, want SIGTERM", cmd.SysProcAttr)
	}
}
