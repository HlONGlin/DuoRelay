package ir

import (
	"testing"
)

func TestProtocolOwnership(t *testing.T) {
	tests := []struct {
		protocol string
		expected KernelSet
	}{
		// Shared protocols
		{ProtocolShadowsocks, KernelSetBoth},
		{ProtocolVMess, KernelSetBoth},
		{ProtocolVLESS, KernelSetBoth},
		{ProtocolTrojan, KernelSetBoth},

		// sing-box exclusive
		{ProtocolShadowTLS, KernelSetSingBox},
		{ProtocolNaive, KernelSetSingBox},
		{ProtocolTor, KernelSetSingBox},
		{ProtocolOpenConnect, KernelSetSingBox},

		// mihomo exclusive
		{ProtocolShadowsocksR, KernelSetMihomo},
		{ProtocolMieru, KernelSetMihomo},
		{ProtocolSudoku, KernelSetMihomo},
		{ProtocolShadowQUIC, KernelSetMihomo},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			got := GetProtocolOwner(tt.protocol)
			if got != tt.expected {
				t.Errorf("GetProtocolOwner(%s) = %v, want %v", tt.protocol, got, tt.expected)
			}
		})
	}
}

func TestShadowTLSStandaloneKernelEligibility(t *testing.T) {
	// sing-box v1.14.4 registers a standalone ShadowTLS outbound. In mihomo
	// v1.19.33, shadow-tls is nested under Shadowsocks/Snell options only.
	// See docs/protocol-verification-report.md for the pinned source evidence.
	node := &Node{
		Protocol: ProtocolShadowTLS,
		Owner:    GetProtocolOwner(ProtocolShadowTLS),
	}

	for _, tt := range []struct {
		kernel KernelType
		want   bool
	}{
		{KernelSingBox, true},
		{KernelMihomo, false},
	} {
		t.Run(string(tt.kernel), func(t *testing.T) {
			if got := node.CanRunOn(tt.kernel); got != tt.want {
				t.Errorf("standalone ShadowTLS CanRunOn(%s) = %v, want %v", tt.kernel, got, tt.want)
			}
		})
	}
}

func TestNodeCanRunOn(t *testing.T) {
	tests := []struct {
		name     string
		owner    KernelSet
		kernel   KernelType
		expected bool
	}{
		{"both kernels on singbox", KernelSetBoth, KernelSingBox, true},
		{"both kernels on mihomo", KernelSetBoth, KernelMihomo, true},
		{"singbox only on singbox", KernelSetSingBox, KernelSingBox, true},
		{"singbox only on mihomo", KernelSetSingBox, KernelMihomo, false},
		{"mihomo only on mihomo", KernelSetMihomo, KernelMihomo, true},
		{"mihomo only on singbox", KernelSetMihomo, KernelSingBox, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{Owner: tt.owner}
			got := node.CanRunOn(tt.kernel)
			if got != tt.expected {
				t.Errorf("Node.CanRunOn(%v) = %v, want %v", tt.kernel, got, tt.expected)
			}
		})
	}
}

func TestGetAllProtocols(t *testing.T) {
	protocols := GetAllProtocols()
	if len(protocols) != 28 {
		t.Errorf("GetAllProtocols() returned %d protocols, want 28", len(protocols))
	}
}

func TestIsValidProtocol(t *testing.T) {
	tests := []struct {
		protocol string
		valid    bool
	}{
		{ProtocolShadowsocks, true},
		{ProtocolVMess, true},
		{"invalid-protocol", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			got := IsValidProtocol(tt.protocol)
			if got != tt.valid {
				t.Errorf("IsValidProtocol(%s) = %v, want %v", tt.protocol, got, tt.valid)
			}
		})
	}
}
