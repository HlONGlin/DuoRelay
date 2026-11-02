package systemproxy

// Snapshot contains the per-user proxy values changed by DuoRelay.
// Platform-specific fields are kept optional so state written on one platform
// remains readable after a migration to another platform.
type Snapshot struct {
	ProxyEnablePresent   bool              `json:"proxy_enable_present"`
	ProxyEnable          uint32            `json:"proxy_enable"`
	ProxyServerPresent   bool              `json:"proxy_server_present"`
	ProxyServer          string            `json:"proxy_server"`
	ProxyOverridePresent bool              `json:"proxy_override_present"`
	ProxyOverride        string            `json:"proxy_override"`
	LinuxBackend         string            `json:"linux_backend,omitempty"`
	LinuxValues          map[string]string `json:"linux_values,omitempty"`
}
