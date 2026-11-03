//go:build !linux

package kernel

func cleanupMihomoTUNNetwork() error { return nil }
