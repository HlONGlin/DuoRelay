package configuration

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
)

// Two real encrypted SS servers plus an echo destination exercise native UDP
// through both bridge directions. No network outside loopback is contacted.
func TestRealChainUDP(t *testing.T) {
	if testing.Short() || os.Getenv("DUORELAY_CHAIN_INTEGRATION") != "1" {
		t.Skip("set DUORELAY_CHAIN_INTEGRATION=1")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "kernels"))
	if err != nil {
		t.Fatal(err)
	}
	port := func() int {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	serverPorts := []int{port(), port()}
	var inbounds []map[string]any
	for i, p := range serverPorts {
		inbounds = append(inbounds, map[string]any{"type": "shadowsocks", "tag": fmt.Sprintf("ss-%d", i), "listen": "127.0.0.1", "listen_port": p, "method": "aes-128-gcm", "password": "chain-udp-test"})
	}
	data, _ := json.Marshal(map[string]any{"log": map[string]any{"level": "debug"}, "inbounds": inbounds, "outbounds": []map[string]any{{"type": "direct", "tag": "DIRECT"}}, "route": map[string]any{"final": "DIRECT"}})
	data, _, _, err = withController(data, ir.KernelSingBox, false)
	if err != nil {
		t.Fatal(err)
	}
	// Use an ephemeral controller instead of the default user instance port.
	root := map[string]any{}
	json.Unmarshal(data, &root)
	root["experimental"].(map[string]any)["clash_api"].(map[string]any)["external_controller"] = fmt.Sprintf("127.0.0.1:%d", port())
	data, _ = json.Marshal(root)
	path := filepath.Join(t.TempDir(), "servers.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	servers := kernel.NewManager(dir)
	defer servers.StopAll()
	if err := servers.StartSingBox(path); err != nil {
		t.Fatal(err)
	}
	echo, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		buffer := make([]byte, 2048)
		for {
			n, peer, err := echo.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = echo.WriteTo(buffer[:n], peer)
		}
	}()
	s, _ := testService(t)
	runtime := kernel.NewManager(dir)
	s.Runtime = runtime
	defer s.Stop(context.Background())
	settings := defaults()
	settings.InboundPort = port()
	settings.MixedPort = port()
	settings.SOCKSPort = port()
	settings.SOCKSOnly = true
	settings.RoutingMode = "global"
	if err := s.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	for _, owners := range [][]ir.KernelSet{{1, 1}, {2, 2}, {1, 2}, {2, 1}} {
		t.Run(fmt.Sprint(owners), func(t *testing.T) {
			startLog := len(servers.GetLogs("sing-box", 0))
			var ids []string
			for i, owner := range owners {
				n := &ir.Node{Name: fmt.Sprintf("udp-%d", i), Protocol: ir.ProtocolShadowsocks, Owner: owner, Server: ir.Server{Host: "127.0.0.1", Port: serverPorts[i]}, Proto: map[string]any{"method": "aes-128-gcm", "password": "chain-udp-test", "udp": true}}
				if err := s.Store.SaveNode(context.Background(), n); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, n.ID)
			}
			state, err := s.Apply(context.Background(), ApplyRequest{Mode: "chain", NodeIDs: ids})
			if err != nil {
				t.Fatal(err)
			}
			if !state.Chain.UDP {
				t.Fatal("UDP was unnecessarily disabled")
			}
			control, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", settings.SOCKSPort), 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer control.Close()
			_ = control.SetDeadline(time.Now().Add(5 * time.Second))
			_, _ = control.Write([]byte{5, 1, 0})
			var greeting [2]byte
			if _, err := io.ReadFull(control, greeting[:]); err != nil || greeting != [2]byte{5, 0} {
				t.Fatalf("SOCKS greeting: %v", err)
			}
			_, _ = control.Write([]byte{5, 3, 0, 1, 0, 0, 0, 0, 0, 0})
			var response [4]byte
			if _, err := io.ReadFull(control, response[:]); err != nil || response[1] != 0 {
				t.Fatalf("UDP associate: %v %v", response, err)
			}
			length := 4
			if response[3] == 4 {
				length = 16
			}
			address := make([]byte, length+2)
			if _, err := io.ReadFull(control, address); err != nil {
				t.Fatal(err)
			}
			udpPort := int(binary.BigEndian.Uint16(address[len(address)-2:]))
			packet, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: udpPort})
			if err != nil {
				t.Fatal(err)
			}
			defer packet.Close()
			_ = packet.SetDeadline(time.Now().Add(5 * time.Second))
			target := echo.LocalAddr().(*net.UDPAddr)
			payload := []byte{0, 0, 0, 1, 127, 0, 0, 1, 0, 0}
			binary.BigEndian.PutUint16(payload[8:], uint16(target.Port))
			payload = append(payload, []byte("chain-udp-ok")...)
			if _, err := packet.Write(payload); err != nil {
				t.Fatal(err)
			}
			buffer := make([]byte, 2048)
			n, err := packet.Read(buffer)
			if err != nil {
				t.Fatal(err)
			}
			if n < len("chain-udp-ok") || string(buffer[n-len("chain-udp-ok"):n]) != "chain-udp-ok" {
				t.Fatal("unexpected UDP echo")
			}
			for hop := 0; hop < 2; hop++ {
				found := false
				for _, entry := range servers.GetLogs("sing-box", 0)[startLog:] {
					if strings.Contains(entry.Line, fmt.Sprintf("inbound/shadowsocks[ss-%d]", hop)) && strings.Contains(entry.Line, "packet") {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("UDP did not traverse encrypted server %d", hop)
				}
			}
		})
	}
}
