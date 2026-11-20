package configuration

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/transparent"
	"gopkg.in/yaml.v3"
)

// sing-box's default auto_redirect_output_mark. Generated Linux TUN configs
// enable auto_redirect, whose nftables rules exempt sockets with this mark.
const singBoxOutputMark = 0x2024

// ProbeTCP measures the node's TCP endpoint using the route selected before TUN starts.
func (s *Service) ProbeTCP(ctx context.Context, address string) (int64, error) {
	s.probeMu.RLock()
	defer s.probeMu.RUnlock()
	if s.probeRoute != nil {
		return s.probeRoute.TCPDelay(ctx, address)
	}
	return speedtest.TCPDelay(ctx, address)
}

// prepareProbeRoute runs after the old kernel has stopped and before the new
// one installs routes. Runtime transitions hold probeMu for the entire change.
func (s *Service) prepareProbeRoute(mode transparent.Mode, raw bool, kernel ir.KernelType, path, dns string) {
	s.probeRoute = nil
	tun := mode == transparent.ModeTUN
	var mark uint32
	if raw {
		var err error
		tun, mark, err = importedTUN(kernel, path)
		if err != nil {
			// Keep the imported config usable, but do not guess its probe route.
			s.probeRoute = &speedtest.DirectRoute{}
			return
		}
	} else if tun && kernel == ir.KernelSingBox && runtime.GOOS == "linux" {
		mark = singBoxOutputMark
	}
	if !tun {
		return
	}
	if dns == "" {
		var stored Settings
		_ = s.Store.State(context.Background(), "settings", &stored)
		dns = stored.DNSBootstrap
		if dns == "" {
			dns = defaults().DNSBootstrap
		}
	}
	s.probeRoute = speedtest.CaptureDirectRoute(dns, mark)
}

func importedTUN(kernel ir.KernelType, path string) (bool, uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}
	if kernel == ir.KernelMihomo {
		var config struct {
			TUN struct {
				Enable bool `yaml:"enable"`
			} `yaml:"tun"`
		}
		err := yaml.Unmarshal(data, &config)
		return config.TUN.Enable, 0, err
	}
	var config struct {
		Inbounds []struct {
			Type         string          `json:"type"`
			AutoRedirect bool            `json:"auto_redirect"`
			OutputMark   json.RawMessage `json:"auto_redirect_output_mark"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return false, 0, err
	}
	for _, inbound := range config.Inbounds {
		if inbound.Type != "tun" {
			continue
		}
		if runtime.GOOS != "linux" || !inbound.AutoRedirect {
			return true, 0, nil
		}
		if len(inbound.OutputMark) == 0 || string(inbound.OutputMark) == "null" || string(inbound.OutputMark) == "0" {
			return true, singBoxOutputMark, nil
		}
		mark, err := strconv.ParseUint(strings.Trim(string(inbound.OutputMark), `"`), 0, 32)
		if mark == 0 {
			mark = singBoxOutputMark
		}
		return true, uint32(mark), err
	}
	return false, 0, nil
}
