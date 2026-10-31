package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/clashapi"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
	"gopkg.in/yaml.v3"
)

var ErrApply = errors.New("configuration apply failed")

type Runtime interface {
	Validate(string, string) error
	StartSingBox(string) error
	StartMihomo(string) error
	StopAll() error
}
type Settings struct {
	Kernel          ir.KernelType `json:"kernel"`
	InboundHost     string        `json:"inbound_host"`
	InboundPort     int           `json:"inbound_port"`
	MixedPort       int           `json:"mixed_port"`
	AllowLAN        bool          `json:"allow_lan"`
	LogLevel        string        `json:"log_level"`
	DNSServers      []string      `json:"dns_servers"`
	DNSFakeIP       bool          `json:"dns_fake_ip"`
	EnableTUN       bool          `json:"enable_tun"`
	TUNStack        string        `json:"tun_stack"`
	DefaultOutbound string        `json:"default_outbound"`
}
type ApplyRequest struct {
	Kernel   ir.KernelType `json:"kernel,omitempty"`
	Content  string        `json:"config,omitempty"`
	NodeIDs  []string      `json:"node_ids,omitempty"`
	outbound string
}
type State struct {
	ID         string        `json:"id"`
	Kernel     ir.KernelType `json:"kernel"`
	NodeIDs    []string      `json:"node_ids"`
	AppliedAt  time.Time     `json:"applied_at"`
	Path       string        `json:"path,omitempty"`
	Controller string        `json:"controller,omitempty"`
	Secret     string        `json:"secret,omitempty"`
}

func (s *State) Public() *State {
	if s == nil {
		return nil
	}
	copy := *s
	copy.Path, copy.Controller, copy.Secret = "", "", ""
	copy.NodeIDs = append([]string{}, s.NodeIDs...)
	return &copy
}

type Service struct {
	mu         sync.Mutex
	Store      *store.Store
	Runtime    Runtime
	Directory  string
	Generators map[ir.KernelType]generator.Generator
}

func New(s *store.Store, runtime Runtime, directory string) *Service {
	return &Service{Store: s, Runtime: runtime, Directory: directory, Generators: map[ir.KernelType]generator.Generator{ir.KernelSingBox: singbox.NewGenerator(), ir.KernelMihomo: mihomo.NewGenerator()}}
}
func defaults() Settings {
	return Settings{Kernel: ir.KernelSingBox, InboundHost: "127.0.0.1", InboundPort: 1080, MixedPort: 7890, LogLevel: "info", DNSServers: []string{"223.5.5.5", "8.8.8.8"}, TUNStack: "system", DefaultOutbound: "DIRECT"}
}
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	value := defaults()
	err := s.Store.State(ctx, "settings", &value)
	if errors.Is(err, store.ErrNotFound) {
		err = nil
	}
	return value, err
}
func (s *Service) SaveSettings(ctx context.Context, value Settings) error {
	if value.Kernel != ir.KernelSingBox && value.Kernel != ir.KernelMihomo {
		return fmt.Errorf("%w: invalid kernel", store.ErrInvalid)
	}
	if net.ParseIP(value.InboundHost) == nil || value.InboundPort < 1 || value.InboundPort > 65535 || value.MixedPort < 1 || value.MixedPort > 65535 {
		return fmt.Errorf("%w: invalid inbound address/port", store.ErrInvalid)
	}
	switch value.LogLevel {
	case "trace", "debug", "info", "warn", "error", "fatal", "panic":
	default:
		return fmt.Errorf("%w: invalid log level", store.ErrInvalid)
	}
	switch value.TUNStack {
	case "system", "gvisor", "mixed":
	default:
		return fmt.Errorf("%w: invalid TUN stack", store.ErrInvalid)
	}
	if value.DefaultOutbound == "" {
		return fmt.Errorf("%w: default_outbound is required", store.ErrInvalid)
	}
	return s.Store.SaveState(ctx, "settings", value)
}
func (s *Service) Current(ctx context.Context) (*State, error) {
	var state State
	if err := s.Store.State(ctx, "current_config", &state); err != nil {
		return nil, err
	}
	return &state, nil
}
func (s *Service) start(kernel ir.KernelType, path string) error {
	if kernel == ir.KernelSingBox {
		return s.Runtime.StartSingBox(path)
	}
	return s.Runtime.StartMihomo(path)
}

func (s *Service) Apply(ctx context.Context, request ApplyRequest) (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	if request.Kernel == "" {
		request.Kernel = settings.Kernel
	}
	if request.Kernel != ir.KernelSingBox && request.Kernel != ir.KernelMihomo {
		return nil, fmt.Errorf("%w: invalid kernel", store.ErrInvalid)
	}
	if len(request.NodeIDs) > 10000 {
		return nil, fmt.Errorf("%w: too many nodes", store.ErrInvalid)
	}
	if request.Content != "" && len(request.NodeIDs) > 0 {
		return nil, fmt.Errorf("%w: provide config or node_ids", store.ErrInvalid)
	}
	nodeIDs := []string{}
	content := []byte(request.Content)
	if len(content) == 0 {
		nodes := []*ir.Node{}
		if len(request.NodeIDs) == 0 {
			all, err := s.Store.Nodes(ctx, "")
			if err != nil {
				return nil, err
			}
			for _, node := range all {
				if node.CanRunOn(request.Kernel) {
					nodes = append(nodes, node)
					nodeIDs = append(nodeIDs, node.ID)
				}
			}
		} else {
			seen := map[string]bool{}
			for _, id := range request.NodeIDs {
				if seen[id] {
					continue
				}
				seen[id] = true
				node, err := s.Store.Node(ctx, id)
				if err != nil {
					return nil, err
				}
				if !node.CanRunOn(request.Kernel) {
					return nil, fmt.Errorf("%w: node cannot run on selected kernel", store.ErrInvalid)
				}
				nodes = append(nodes, node)
				nodeIDs = append(nodeIDs, id)
			}
		}
		options := &generator.GenerateOptions{InboundHost: settings.InboundHost, InboundPort: settings.InboundPort, MixedPort: settings.MixedPort, DNSServers: settings.DNSServers, DNSFakeIP: settings.DNSFakeIP, EnableTUN: settings.EnableTUN, TUNStack: settings.TUNStack, LogLevel: settings.LogLevel, DefaultOutbound: settings.DefaultOutbound}
		if request.outbound != "" {
			options.DefaultOutbound = request.outbound
		}
		if settings.AllowLAN {
			options.InboundHost = "0.0.0.0"
		}
		gen := s.Generators[request.Kernel]
		if gen == nil {
			return nil, fmt.Errorf("generator is not configured")
		}
		content, err = gen.Generate(nodes, options)
		if err != nil {
			return nil, err
		}
	}
	content, controller, secret, err := withController(content, request.Kernel, settings.AllowLAN)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory := s.Directory
	if directory == "" {
		directory = filepath.Join("data", "configs")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	path, err := filepath.Abs(filepath.Join(directory, id+".json"))
	if err != nil {
		return nil, err
	}
	if request.Kernel == ir.KernelMihomo {
		path = strings.TrimSuffix(path, ".json") + ".yaml"
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		return nil, err
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.Remove(path)
		}
	}()
	if s.Runtime == nil {
		return nil, fmt.Errorf("kernel runtime is not configured")
	}
	if err := s.Runtime.Validate(string(request.Kernel), path); err != nil {
		return nil, fmt.Errorf("%w: validation: %v", ErrApply, err)
	}
	previous, err := s.Current(ctx)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.Runtime.StopAll(); err != nil {
		return nil, fmt.Errorf("%w: stop: %v", ErrApply, err)
	}
	rollback := func(cause error) error {
		stopErr := s.Runtime.StopAll()
		var restoreErr error
		if previous != nil {
			restoreErr = s.start(previous.Kernel, previous.Path)
		}
		return errors.Join(fmt.Errorf("%w: %v", ErrApply, cause), stopErr, restoreErr)
	}
	if err := s.start(request.Kernel, path); err != nil {
		return nil, rollback(err)
	}
	state := &State{ID: id, Kernel: request.Kernel, NodeIDs: nodeIDs, AppliedAt: time.Now().UTC(), Path: path, Controller: controller, Secret: secret}
	// Finish the durable state update even if the HTTP client disconnected.
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Store.SaveState(persistCtx, "current_config", state); err != nil {
		return nil, rollback(err)
	}
	retained = true
	return state, nil
}

func (s *Service) Activate(ctx context.Context, id string, kernel ir.KernelType) (*State, error) {
	node, err := s.Store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	if kernel == "" {
		kernel = node.PreferredKernel()
	}
	if !node.CanRunOn(kernel) {
		return nil, fmt.Errorf("%w: incompatible kernel", store.ErrInvalid)
	}
	return s.Apply(ctx, ApplyRequest{Kernel: kernel, NodeIDs: []string{id}, outbound: node.Name})
}

func withController(content []byte, kernel ir.KernelType, allowLAN bool) ([]byte, string, string, error) {
	root := map[string]any{}
	var err error
	if kernel == ir.KernelSingBox {
		err = json.Unmarshal(content, &root)
	} else {
		err = yaml.Unmarshal(content, &root)
	}
	if err != nil || root == nil {
		return nil, "", "", fmt.Errorf("%w: malformed config", store.ErrInvalid)
	}
	container := root
	addressKey := "external-controller"
	defaultAddress := "127.0.0.1:19093"
	if kernel == ir.KernelSingBox {
		defaultAddress = "127.0.0.1:19094"
		addressKey = "external_controller"
		experimental, _ := root["experimental"].(map[string]any)
		if experimental == nil {
			experimental = map[string]any{}
			root["experimental"] = experimental
		}
		container, _ = experimental["clash_api"].(map[string]any)
		if container == nil {
			container = map[string]any{}
			experimental["clash_api"] = container
		}
	} else {
		root["allow-lan"] = allowLAN
	}
	address, _ := container[addressKey].(string)
	if address == "" {
		address = defaultAddress
		container[addressKey] = address
	}
	secret, _ := container["secret"].(string)
	if secret == "" {
		secret = uuid.NewString()
		container["secret"] = secret
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" || port == "0" {
		return nil, "", "", fmt.Errorf("%w: invalid controller address", store.ErrInvalid)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	var output []byte
	if kernel == ir.KernelSingBox {
		output, err = json.MarshalIndent(root, "", "  ")
	} else {
		output, err = yaml.Marshal(root)
	}
	return output, "http://" + net.JoinHostPort(host, port), secret, err
}

func (s *Service) ProbeHTTP(ctx context.Context, node *ir.Node, target string, timeout time.Duration) (int64, error) {
	state, err := s.Current(ctx)
	if err != nil {
		return 0, err
	}
	found := false
	for _, id := range state.NodeIDs {
		if id == node.ID {
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("node is not in the active generated configuration")
	}
	client := clashapi.NewClient(state.Controller, state.Secret, clashapi.KernelType(state.Kernel))
	defer client.Close()
	response, err := client.TestDelay(ctx, node.Name, target, int(timeout/time.Millisecond))
	if err != nil {
		return 0, err
	}
	return int64(response.Delay), nil
}
