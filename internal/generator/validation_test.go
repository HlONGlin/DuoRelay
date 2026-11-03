//go:build !lite

package generator_test

import (
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
)

func TestGeneratedConfigValidatorsAcceptGeneratedConfigs(t *testing.T) {
	for _, tc := range []struct {
		name string
		gen  generator.Generator
	}{
		{name: "sing-box", gen: singbox.NewGenerator()},
		{name: "mihomo", gen: mihomo.NewGenerator()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := tc.gen.Generate(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.gen.Validate(config); err != nil {
				t.Fatalf("generated config rejected: %v", err)
			}
		})
	}
}

func TestGeneratedConfigValidatorsRejectMalformedOrWrongShape(t *testing.T) {
	for _, tc := range []struct {
		name  string
		gen   generator.Generator
		bad   []byte
		wrong []byte
	}{
		{name: "sing-box", gen: singbox.NewGenerator(), bad: []byte(`{"log":`), wrong: []byte(`{"log":{},"dns":{},"inbounds":[],"outbounds":[]}`)},
		{name: "mihomo", gen: mihomo.NewGenerator(), bad: []byte("mode: ["), wrong: []byte("mode: rule\nlog-level: info\ndns: {}\nproxies: []\nproxy-groups: []\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.gen.Validate(tc.bad); err == nil {
				t.Fatal("malformed config was accepted")
			}
			if err := tc.gen.Validate(tc.wrong); err == nil {
				t.Fatal("config with missing required fields was accepted")
			}
		})
	}
}
