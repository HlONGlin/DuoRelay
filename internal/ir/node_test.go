package ir

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNodeSerialization(t *testing.T) {
	tests := []struct {
		name string
		node *Node
	}{
		{
			name: "Shadowsocks basic",
			node: ExampleShadowsocksNode(),
		},
		{
			name: "Shadowsocks with plugin",
			node: ExampleShadowsocksNodeWithPlugin(),
		},
		{
			name: "VMess with WebSocket",
			node: ExampleVMessNode(),
		},
		{
			name: "VMess with gRPC",
			node: ExampleVMessNodeGRPC(),
		},
		{
			name: "VLESS with Reality",
			node: ExampleVLESSNode(),
		},
		{
			name: "VLESS with WebSocket",
			node: ExampleVLESSNodeWS(),
		},
		{
			name: "Trojan basic",
			node: ExampleTrojanNode(),
		},
		{
			name: "Trojan with WebSocket",
			node: ExampleTrojanNodeWS(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize to JSON
			data, err := json.Marshal(tt.node)
			if err != nil {
				t.Fatalf("Failed to marshal node: %v", err)
			}

			// Deserialize from JSON
			var decoded Node
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal node: %v", err)
			}

			// Verify core fields
			if decoded.ID != tt.node.ID {
				t.Errorf("ID mismatch: got %s, want %s", decoded.ID, tt.node.ID)
			}
			if decoded.Name != tt.node.Name {
				t.Errorf("Name mismatch: got %s, want %s", decoded.Name, tt.node.Name)
			}
			if decoded.Protocol != tt.node.Protocol {
				t.Errorf("Protocol mismatch: got %s, want %s", decoded.Protocol, tt.node.Protocol)
			}
			if decoded.Owner != tt.node.Owner {
				t.Errorf("Owner mismatch: got %v, want %v", decoded.Owner, tt.node.Owner)
			}

			// Verify Server
			if decoded.Server.Host != tt.node.Server.Host {
				t.Errorf("Server.Host mismatch: got %s, want %s", decoded.Server.Host, tt.node.Server.Host)
			}
			if decoded.Server.Port != tt.node.Server.Port {
				t.Errorf("Server.Port mismatch: got %d, want %d", decoded.Server.Port, tt.node.Server.Port)
			}

			// Verify Transport
			if decoded.Transport.Type != tt.node.Transport.Type {
				t.Errorf("Transport.Type mismatch: got %s, want %s", decoded.Transport.Type, tt.node.Transport.Type)
			}

			// Verify Security
			if decoded.Security.Type != tt.node.Security.Type {
				t.Errorf("Security.Type mismatch: got %s, want %s", decoded.Security.Type, tt.node.Security.Type)
			}

			// Verify Proto map has same keys
			if len(decoded.Proto) != len(tt.node.Proto) {
				t.Errorf("Proto map length mismatch: got %d, want %d", len(decoded.Proto), len(tt.node.Proto))
			}
			for key := range tt.node.Proto {
				if _, exists := decoded.Proto[key]; !exists {
					t.Errorf("Proto key missing after deserialization: %s", key)
				}
			}
		})
	}
}

func TestNodeJSON_LosslessRoundTrip(t *testing.T) {
	original := ExampleVMessNode()

	// First round: serialize and deserialize
	data1, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("First marshal failed: %v", err)
	}

	var decoded1 Node
	if err := json.Unmarshal(data1, &decoded1); err != nil {
		t.Fatalf("First unmarshal failed: %v", err)
	}

	// Second round: serialize again
	data2, err := json.Marshal(&decoded1)
	if err != nil {
		t.Fatalf("Second marshal failed: %v", err)
	}

	// Compare JSON output (should be identical)
	var obj1, obj2 map[string]interface{}
	if err := json.Unmarshal(data1, &obj1); err != nil {
		t.Fatalf("Failed to unmarshal first JSON: %v", err)
	}
	if err := json.Unmarshal(data2, &obj2); err != nil {
		t.Fatalf("Failed to unmarshal second JSON: %v", err)
	}

	// Check critical fields are preserved
	if obj1["id"] != obj2["id"] {
		t.Error("ID not preserved in round-trip")
	}
	if obj1["protocol"] != obj2["protocol"] {
		t.Error("Protocol not preserved in round-trip")
	}
}

func TestNodeProtoValidation_Shadowsocks(t *testing.T) {
	tests := []struct {
		name      string
		proto     map[string]any
		wantError bool
	}{
		{
			name: "valid basic config",
			proto: map[string]any{
				"method":   "aes-256-gcm",
				"password": "test-password",
			},
			wantError: false,
		},
		{
			name: "valid with plugin",
			proto: map[string]any{
				"method":      "chacha20-ietf-poly1305",
				"password":    "test-pass",
				"plugin":      "obfs-local",
				"plugin_opts": "obfs=http",
			},
			wantError: false,
		},
		{
			name: "missing required field",
			proto: map[string]any{
				"method": "aes-256-gcm",
				// missing password
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{
				ID:       "test-ss",
				Protocol: ProtocolShadowsocks,
				Proto:    tt.proto,
			}

			// Validate by marshaling/unmarshaling
			data, err := json.Marshal(node)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded Node
			err = json.Unmarshal(data, &decoded)
			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}

			// Check required fields exist
			if !tt.wantError {
				if _, ok := tt.proto["method"]; !ok {
					t.Error("method field required but not present")
				}
				if _, ok := tt.proto["password"]; !ok {
					t.Error("password field required but not present")
				}
			}
		})
	}
}

func TestNodeProtoValidation_VMess(t *testing.T) {
	tests := []struct {
		name  string
		proto map[string]any
		valid bool
	}{
		{
			name: "valid modern config",
			proto: map[string]any{
				"uuid":     "12345678-1234-1234-1234-123456789abc",
				"alter_id": 0,
				"security": "auto",
			},
			valid: true,
		},
		{
			name: "valid legacy config",
			proto: map[string]any{
				"uuid":     "87654321-4321-4321-4321-cba987654321",
				"alter_id": 64,
				"security": "aes-128-gcm",
			},
			valid: true,
		},
		{
			name: "missing uuid",
			proto: map[string]any{
				"alter_id": 0,
				"security": "auto",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if _, ok := tt.proto["uuid"]; !ok {
					t.Error("uuid is required for valid VMess config")
				}
			}
		})
	}
}

func TestNodeProtoValidation_VLESS(t *testing.T) {
	tests := []struct {
		name  string
		proto map[string]any
		valid bool
	}{
		{
			name: "valid with flow",
			proto: map[string]any{
				"uuid":       "abcdef12-3456-7890-abcd-ef1234567890",
				"flow":       "xtls-rprx-vision",
				"encryption": "none",
			},
			valid: true,
		},
		{
			name: "valid without flow",
			proto: map[string]any{
				"uuid":       "fedcba98-7654-3210-fedc-ba9876543210",
				"encryption": "none",
			},
			valid: true,
		},
		{
			name: "missing uuid",
			proto: map[string]any{
				"encryption": "none",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if _, ok := tt.proto["uuid"]; !ok {
					t.Error("uuid is required for valid VLESS config")
				}
			}
		})
	}
}

func TestNodeProtoValidation_Trojan(t *testing.T) {
	tests := []struct {
		name  string
		proto map[string]any
		valid bool
	}{
		{
			name: "valid basic config",
			proto: map[string]any{
				"password": "trojan-password-123",
			},
			valid: true,
		},
		{
			name: "valid with udp",
			proto: map[string]any{
				"password": "secure-password",
				"udp":      true,
			},
			valid: true,
		},
		{
			name: "missing password",
			proto: map[string]any{
				"udp": true,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if _, ok := tt.proto["password"]; !ok {
					t.Error("password is required for valid Trojan config")
				}
			}
		})
	}
}

func TestNodeTimeFields(t *testing.T) {
	node := ExampleShadowsocksNode()

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Node
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify time fields are preserved (within reasonable delta)
	if !decoded.CreatedAt.IsZero() && decoded.CreatedAt.Unix() != node.CreatedAt.Unix() {
		t.Errorf("CreatedAt not preserved: got %v, want %v", decoded.CreatedAt, node.CreatedAt)
	}
}

func TestNodeExtraFields(t *testing.T) {
	node := &Node{
		ID:       "test-extra",
		Protocol: ProtocolShadowsocks,
		Extra: map[string]any{
			"custom_field":  "custom_value",
			"numeric_field": 12345,
			"bool_field":    true,
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Node
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify Extra fields are preserved
	if len(decoded.Extra) != len(node.Extra) {
		t.Errorf("Extra map size mismatch: got %d, want %d", len(decoded.Extra), len(node.Extra))
	}

	for key := range node.Extra {
		if _, exists := decoded.Extra[key]; !exists {
			t.Errorf("Extra field %s missing after deserialization", key)
		}
	}
}

func TestGetAllExamples(t *testing.T) {
	examples := GetAllExamples()

	if len(examples) != 8 {
		t.Errorf("Expected 8 example nodes, got %d", len(examples))
	}

	// Count protocols
	protocolCount := make(map[string]int)
	for _, node := range examples {
		protocolCount[node.Protocol]++
	}

	expectedCounts := map[string]int{
		ProtocolShadowsocks: 2,
		ProtocolVMess:       2,
		ProtocolVLESS:       2,
		ProtocolTrojan:      2,
	}

	for protocol, expected := range expectedCounts {
		if got := protocolCount[protocol]; got != expected {
			t.Errorf("Protocol %s: expected %d examples, got %d", protocol, expected, got)
		}
	}
}

func TestNodeCanRunOnKernel(t *testing.T) {
	node := ExampleShadowsocksNode()

	if !node.CanRunOn(KernelSingBox) {
		t.Error("Shadowsocks should run on sing-box")
	}
	if !node.CanRunOn(KernelMihomo) {
		t.Error("Shadowsocks should run on mihomo")
	}
}

func TestNodePreferredKernel(t *testing.T) {
	node := ExampleVMessNode()

	preferred := node.PreferredKernel()
	if preferred != KernelSingBox {
		t.Errorf("Expected preferred kernel to be sing-box, got %s", preferred)
	}
}

func BenchmarkNodeSerialization(b *testing.B) {
	node := ExampleVMessNode()

	b.Run("Marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(node)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	data, _ := json.Marshal(node)
	b.Run("Unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var decoded Node
			err := json.Unmarshal(data, &decoded)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestNodeValidation_AllExamples(t *testing.T) {
	examples := GetAllExamples()

	for _, node := range examples {
		t.Run(node.Name, func(t *testing.T) {
			// Basic validation
			if node.ID == "" {
				t.Error("Node ID should not be empty")
			}
			if node.Name == "" {
				t.Error("Node Name should not be empty")
			}
			if !IsValidProtocol(node.Protocol) {
				t.Errorf("Invalid protocol: %s", node.Protocol)
			}
			if node.Server.Host == "" {
				t.Error("Server host should not be empty")
			}
			if node.Server.Port <= 0 || node.Server.Port > 65535 {
				t.Errorf("Invalid port: %d", node.Server.Port)
			}
			if node.Owner == 0 {
				t.Error("Node owner should be set")
			}

			// Proto validation
			if len(node.Proto) == 0 {
				t.Error("Proto map should not be empty")
			}

			// Protocol-specific validation
			switch node.Protocol {
			case ProtocolShadowsocks:
				if _, ok := node.Proto["method"]; !ok {
					t.Error("Shadowsocks requires 'method' field")
				}
				if _, ok := node.Proto["password"]; !ok {
					t.Error("Shadowsocks requires 'password' field")
				}
			case ProtocolVMess:
				if _, ok := node.Proto["uuid"]; !ok {
					t.Error("VMess requires 'uuid' field")
				}
				if _, ok := node.Proto["security"]; !ok {
					t.Error("VMess requires 'security' field")
				}
			case ProtocolVLESS:
				if _, ok := node.Proto["uuid"]; !ok {
					t.Error("VLESS requires 'uuid' field")
				}
			case ProtocolTrojan:
				if _, ok := node.Proto["password"]; !ok {
					t.Error("Trojan requires 'password' field")
				}
			}
		})
	}
}

func TestNodeMarshalIndent(t *testing.T) {
	node := ExampleShadowsocksNode()

	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	// Verify it's valid JSON
	var decoded Node
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal indented JSON: %v", err)
	}
}

func TestNodeWithZeroValues(t *testing.T) {
	node := &Node{
		ID:       "test-zero",
		Protocol: ProtocolShadowsocks,
		Server: Server{
			Host: "example.com",
			Port: 8388,
		},
		Proto: map[string]any{
			"method":   "aes-256-gcm",
			"password": "test",
		},
		// Other fields are zero values
		LastDelayMs: 0,
		SortOrder:   0,
		Favorite:    false,
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Node
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Zero values should be preserved
	if decoded.LastDelayMs != 0 {
		t.Error("LastDelayMs should be 0")
	}
	if decoded.Favorite != false {
		t.Error("Favorite should be false")
	}
}
