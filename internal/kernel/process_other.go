//go:build !linux

package kernel

import "os/exec"

func configureParentDeath(*exec.Cmd) {}
