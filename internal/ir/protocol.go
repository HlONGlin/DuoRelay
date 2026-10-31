package ir

// Protocol constants - 28 protocols total
const (
	// Shared by both kernels (16)
	ProtocolShadowsocks = "shadowsocks"
	ProtocolVMess       = "vmess"
	ProtocolVLESS       = "vless"
	ProtocolTrojan      = "trojan"
	ProtocolHysteria    = "hysteria"
	ProtocolHysteria2   = "hysteria2"
	ProtocolTUIC        = "tuic"
	ProtocolAnyTLS      = "anytls"
	ProtocolSnell       = "snell"
	ProtocolSOCKS5      = "socks5"
	ProtocolHTTP        = "http"
	ProtocolSSH         = "ssh"
	ProtocolWireGuard   = "wireguard"
	ProtocolOpenVPN     = "openvpn"
	ProtocolMASQUE      = "masque"
	ProtocolTailscale   = "tailscale"

	// sing-box exclusive (4)
	ProtocolShadowTLS   = "shadowtls"
	ProtocolNaive       = "naive"
	ProtocolTor         = "tor"
	ProtocolOpenConnect = "openconnect"

	// mihomo exclusive (8)
	ProtocolShadowsocksR = "shadowsocksr"
	ProtocolMieru        = "mieru"
	ProtocolSudoku       = "sudoku"
	ProtocolShadowQUIC   = "shadowquic"
	ProtocolTrustTunnel  = "trusttunnel"
	ProtocolRematch      = "rematch"
	ProtocolEasyTier     = "easytier"
	ProtocolZeroTier     = "zerotier"
)

// ProtocolOwnership maps protocols to their supporting kernels
var ProtocolOwnership = map[string]KernelSet{
	// Shared
	ProtocolShadowsocks: KernelSetBoth,
	ProtocolVMess:       KernelSetBoth,
	ProtocolVLESS:       KernelSetBoth,
	ProtocolTrojan:      KernelSetBoth,
	ProtocolHysteria:    KernelSetBoth,
	ProtocolHysteria2:   KernelSetBoth,
	ProtocolTUIC:        KernelSetBoth,
	ProtocolAnyTLS:      KernelSetBoth,
	ProtocolSnell:       KernelSetBoth,
	ProtocolSOCKS5:      KernelSetBoth,
	ProtocolHTTP:        KernelSetBoth,
	ProtocolSSH:         KernelSetBoth,
	ProtocolWireGuard:   KernelSetBoth,
	ProtocolOpenVPN:     KernelSetBoth,
	ProtocolMASQUE:      KernelSetBoth,
	ProtocolTailscale:   KernelSetBoth,

	// sing-box exclusive
	ProtocolShadowTLS:   KernelSetSingBox,
	ProtocolNaive:       KernelSetSingBox,
	ProtocolTor:         KernelSetSingBox,
	ProtocolOpenConnect: KernelSetSingBox,

	// mihomo exclusive
	ProtocolShadowsocksR: KernelSetMihomo,
	ProtocolMieru:        KernelSetMihomo,
	ProtocolSudoku:       KernelSetMihomo,
	ProtocolShadowQUIC:   KernelSetMihomo,
	ProtocolTrustTunnel:  KernelSetMihomo,
	ProtocolRematch:      KernelSetMihomo,
	ProtocolEasyTier:     KernelSetMihomo,
	ProtocolZeroTier:     KernelSetMihomo,
}

// GetProtocolOwner returns the kernel set that can run this protocol
func GetProtocolOwner(protocol string) KernelSet {
	if owner, ok := ProtocolOwnership[protocol]; ok {
		return owner
	}
	return 0 // Unknown protocol
}

// IsValidProtocol checks if the protocol is supported
func IsValidProtocol(protocol string) bool {
	_, ok := ProtocolOwnership[protocol]
	return ok
}

// GetAllProtocols returns all supported protocols
func GetAllProtocols() []string {
	protocols := make([]string, 0, len(ProtocolOwnership))
	for p := range ProtocolOwnership {
		protocols = append(protocols, p)
	}
	return protocols
}
