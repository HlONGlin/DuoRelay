package transparent

type Mode string

const (
	ModeOff      Mode = "off"
	ModeRedirect Mode = "redirect"
	ModeTProxy   Mode = "tproxy"
	ModeTUN      Mode = "tun"
)

type ModeInfo struct {
	Mode           Mode   `json:"mode"`
	Supported      bool   `json:"supported"`
	Reason         string `json:"reason,omitempty"`
	Prerequisites  string `json:"prerequisites"`
	DNSDescription string `json:"dns_description"`
}

type Manager struct {
	backend backend
}

type backend interface {
	Capabilities() []ModeInfo
	Preflight(Mode, int, int) error
	Prepare(Mode, int, int) error
	Cleanup() error
}

func New() *Manager { return &Manager{backend: newBackend()} }

func (m *Manager) Capabilities() []ModeInfo {
	if m == nil || m.backend == nil {
		return []ModeInfo{{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限", DNSDescription: "不接管 DNS。"}}
	}
	return m.backend.Capabilities()
}

func (m *Manager) Prepare(mode Mode, redirectPort, tproxyPort int) error {
	if mode == "" || mode == ModeOff {
		return m.Cleanup()
	}
	return m.backend.Prepare(mode, redirectPort, tproxyPort)
}

func (m *Manager) Preflight(mode Mode, redirectPort, tproxyPort int) error {
	if m == nil || m.backend == nil || mode == "" || mode == ModeOff {
		return nil
	}
	return m.backend.Preflight(mode, redirectPort, tproxyPort)
}

func (m *Manager) Cleanup() error {
	if m == nil || m.backend == nil {
		return nil
	}
	return m.backend.Cleanup()
}
