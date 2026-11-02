package ir

import "time"

// ExampleShadowsocksNode returns a sample Shadowsocks node
func ExampleShadowsocksNode() *Node {
	now := time.Now()
	return &Node{
		ID:       "ss-example-001",
		Name:     "HK-SS-01",
		Protocol: ProtocolShadowsocks,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "hk.example.com",
			Port: 8388,
		},
		Transport: Transport{
			Type: "tcp",
		},
		Security: Security{
			Type: "none",
		},
		Proto: map[string]any{
			"method":   "aes-256-gcm",
			"password": "example-password-123",
			"udp":      true,
		},
		Extra: map[string]any{},
		Source: Source{
			Type:           "subscription",
			SubscriptionID: "sub-001",
			URL:            "https://example.com/sub",
		},
		Tags:        []string{"hongkong", "premium"},
		Status:      "active",
		LastDelayMs: 85,
		LastTestAt:  now.Add(-5 * time.Minute),
		UpdatedAt:   now,
		CreatedAt:   now.Add(-24 * time.Hour),
	}
}

// ExampleShadowsocksNodeWithPlugin returns a Shadowsocks node with plugin
func ExampleShadowsocksNodeWithPlugin() *Node {
	now := time.Now()
	return &Node{
		ID:       "ss-example-002",
		Name:     "US-SS-Obfs",
		Protocol: ProtocolShadowsocks,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "us.example.com",
			Port: 443,
		},
		Transport: Transport{
			Type: "tcp",
		},
		Security: Security{
			Type: "none",
		},
		Proto: map[string]any{
			"method":      "chacha20-ietf-poly1305",
			"password":    "secure-pass-456",
			"plugin":      "obfs-local",
			"plugin_opts": "obfs=http;obfs-host=www.bing.com",
			"udp":         true,
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "manual"},
		Tags:      []string{"usa", "obfs"},
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now,
	}
}

// ExampleVMessNode returns a sample VMess node
func ExampleVMessNode() *Node {
	now := time.Now()
	return &Node{
		ID:       "vmess-example-001",
		Name:     "SG-VMess-WS",
		Protocol: ProtocolVMess,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "sg.example.com",
			Port: 443,
		},
		Transport: Transport{
			Type: "ws",
			Path: "/vmess",
			Host: "www.example.com",
			Opts: map[string]any{
				"max_early_data":         2048,
				"early_data_header_name": "Sec-WebSocket-Protocol",
			},
		},
		Security: Security{
			Type: "tls",
			SNI:  "www.example.com",
			ALPN: []string{"h2", "http/1.1"},
		},
		Proto: map[string]any{
			"uuid":     "12345678-1234-1234-1234-123456789abc",
			"alter_id": 0,
			"security": "auto",
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "subscription", SubscriptionID: "sub-002"},
		Tags:      []string{"singapore", "ws", "tls"},
		Favorite:  true,
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now.Add(-48 * time.Hour),
	}
}

// ExampleVMessNodeGRPC returns a VMess node with gRPC transport
func ExampleVMessNodeGRPC() *Node {
	now := time.Now()
	return &Node{
		ID:       "vmess-example-002",
		Name:     "JP-VMess-gRPC",
		Protocol: ProtocolVMess,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "jp.example.com",
			Port: 443,
		},
		Transport: Transport{
			Type: "grpc",
			Path: "vmess-grpc",
			Opts: map[string]any{
				"idle_timeout":          60,
				"health_check":          false,
				"permit_without_stream": false,
			},
		},
		Security: Security{
			Type:        "tls",
			SNI:         "jp.example.com",
			ALPN:        []string{"h2"},
			Fingerprint: "chrome",
		},
		Proto: map[string]any{
			"uuid":            "87654321-4321-4321-4321-cba987654321",
			"alter_id":        0,
			"security":        "aes-128-gcm",
			"packet_encoding": "xudp",
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "import"},
		Tags:      []string{"japan", "grpc"},
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now.Add(-7 * 24 * time.Hour),
	}
}

// ExampleVLESSNode returns a sample VLESS node
func ExampleVLESSNode() *Node {
	now := time.Now()
	return &Node{
		ID:       "vless-example-001",
		Name:     "DE-VLESS-Reality",
		Protocol: ProtocolVLESS,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "de.example.com",
			Port: 443,
		},
		Transport: Transport{
			Type: "tcp",
		},
		Security: Security{
			Type:        "reality",
			SNI:         "www.microsoft.com",
			Fingerprint: "chrome",
			Opts: map[string]any{
				"public_key": "example-public-key-base64",
				"short_id":   "0123456789abcdef",
			},
		},
		Proto: map[string]any{
			"uuid":       "abcdef12-3456-7890-abcd-ef1234567890",
			"flow":       "xtls-rprx-vision",
			"encryption": "none",
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "manual"},
		Tags:      []string{"germany", "reality", "xtls"},
		Favorite:  true,
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now.Add(-3 * 24 * time.Hour),
	}
}

// ExampleVLESSNodeWS returns a VLESS node with WebSocket transport
func ExampleVLESSNodeWS() *Node {
	now := time.Now()
	return &Node{
		ID:       "vless-example-002",
		Name:     "UK-VLESS-WS-TLS",
		Protocol: ProtocolVLESS,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "uk.example.com",
			Port: 443,
		},
		Transport: Transport{
			Type: "ws",
			Path: "/vless-ws",
			Host: "cdn.example.com",
		},
		Security: Security{
			Type: "tls",
			SNI:  "cdn.example.com",
			ALPN: []string{"http/1.1"},
		},
		Proto: map[string]any{
			"uuid":       "fedcba98-7654-3210-fedc-ba9876543210",
			"encryption": "none",
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "subscription", SubscriptionID: "sub-003"},
		Tags:      []string{"uk", "ws"},
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now.Add(-10 * 24 * time.Hour),
	}
}

// ExampleTrojanNode returns a sample Trojan node
func ExampleTrojanNode() *Node {
	now := time.Now()
	return &Node{
		ID:       "trojan-example-001",
		Name:     "CA-Trojan-TLS",
		Protocol: ProtocolTrojan,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "ca.example.com",
			Port: 443,
		},
		Transport: Transport{
			Type: "tcp",
		},
		Security: Security{
			Type:        "tls",
			SNI:         "ca.example.com",
			ALPN:        []string{"h2", "http/1.1"},
			Fingerprint: "firefox",
		},
		Proto: map[string]any{
			"password": "trojan-password-abc123",
			"udp":      true,
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "subscription", SubscriptionID: "sub-001"},
		Tags:      []string{"canada", "trojan"},
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now.Add(-14 * 24 * time.Hour),
	}
}

// ExampleTrojanNodeWS returns a Trojan node with WebSocket transport
func ExampleTrojanNodeWS() *Node {
	now := time.Now()
	return &Node{
		ID:       "trojan-example-002",
		Name:     "FR-Trojan-WS",
		Protocol: ProtocolTrojan,
		Owner:    KernelSetBoth,
		Server: Server{
			Host: "fr.example.com",
			Port: 8443,
		},
		Transport: Transport{
			Type: "ws",
			Path: "/trojan",
			Host: "fr.example.com",
		},
		Security: Security{
			Type: "tls",
			SNI:  "fr.example.com",
			ALPN: []string{"http/1.1"},
		},
		Proto: map[string]any{
			"password": "secure-trojan-pass-xyz",
			"udp":      true,
		},
		Extra:     map[string]any{},
		Source:    Source{Type: "manual"},
		Tags:      []string{"france", "ws"},
		Status:    "active",
		UpdatedAt: now,
		CreatedAt: now.Add(-5 * 24 * time.Hour),
	}
}

// GetAllExamples returns all example nodes
func GetAllExamples() []*Node {
	return []*Node{
		ExampleShadowsocksNode(),
		ExampleShadowsocksNodeWithPlugin(),
		ExampleVMessNode(),
		ExampleVMessNodeGRPC(),
		ExampleVLESSNode(),
		ExampleVLESSNodeWS(),
		ExampleTrojanNode(),
		ExampleTrojanNodeWS(),
	}
}
