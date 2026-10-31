package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/holi/duorelay/internal/clashapi"
	"github.com/holi/duorelay/internal/configuration"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/subscription"
	"github.com/holi/duorelay/internal/task"
)

type KernelReader interface {
	Status() map[string]any
	GetLogs(string, int) []kernel.LogEntry
	StopAll() error
}
type Server struct {
	Store         *store.Store
	Subscriptions *subscription.Manager
	Speedtest     *speedtest.Engine
	Config        *configuration.Service
	Kernel        KernelReader
	Context       context.Context
	mux           *http.ServeMux
}

func NewServer(s *store.Store, subs *subscription.Manager, speed *speedtest.Engine, config *configuration.Service, runtime KernelReader) *Server {
	server := &Server{Store: s, Subscriptions: subs, Speedtest: speed, Config: config, Kernel: runtime, Context: context.Background(), mux: http.NewServeMux()}
	server.routes()
	return server
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/subscriptions", s.listSubscriptions)
	s.mux.HandleFunc("POST /api/subscriptions", s.createSubscription)
	s.mux.HandleFunc("GET /api/subscriptions/{id}", s.getSubscription)
	s.mux.HandleFunc("PUT /api/subscriptions/{id}", s.editSubscription)
	s.mux.HandleFunc("DELETE /api/subscriptions/{id}", s.deleteSubscription)
	s.mux.HandleFunc("POST /api/subscriptions/{id}/update", s.updateSubscription)
	s.mux.HandleFunc("GET /api/subscription-groups", s.groups)
	s.mux.HandleFunc("GET /api/nodes", s.listNodes)
	s.mux.HandleFunc("POST /api/nodes", s.createNode)
	s.mux.HandleFunc("DELETE /api/nodes/{id}", s.deleteNode)
	s.mux.HandleFunc("POST /api/nodes/{id}/test", s.testNode)
	s.mux.HandleFunc("POST /api/nodes/{id}/activate", s.activateNode)
	s.mux.HandleFunc("POST /api/speedtest/batch", s.batchTest)
	s.mux.HandleFunc("GET /api/speedtest/tasks/{id}", s.testTask)
	s.mux.HandleFunc("DELETE /api/speedtest/tasks/{id}", s.cancelTask)
	s.mux.HandleFunc("GET /api/config", s.settings)
	s.mux.HandleFunc("PUT /api/config", s.saveSettings)
	s.mux.HandleFunc("GET /api/config/current", s.currentConfig)
	s.mux.HandleFunc("POST /api/config/apply", s.applyConfig)
	s.mux.HandleFunc("GET /api/kernel/logs", s.logs)
	s.mux.HandleFunc("GET /api/kernel/status", s.kernelStatus)
	s.mux.HandleFunc("POST /api/kernel/start", s.applyConfig)
	s.mux.HandleFunc("POST /api/kernel/stop", s.stopKernel)
	s.mux.HandleFunc("GET /api/kernel/connections", s.connections)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { WriteError(w, 404, ErrNotFound("endpoint")) })
}
func decode(w http.ResponseWriter, r *http.Request, value any, optional bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("%w: invalid JSON request", store.ErrInvalid)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: expected one JSON document", store.ErrInvalid)
	}
	return nil
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalid):
		WriteError(w, 400, ErrInvalidRequest(err.Error()))
	case errors.Is(err, store.ErrNotFound), errors.Is(err, task.ErrNotFound):
		WriteError(w, 404, ErrNotFound("resource"))
	case errors.Is(err, store.ErrConflict):
		WriteError(w, 409, &Error{Code: "CONFLICT", What: "Resource was changed"})
	case errors.Is(err, context.DeadlineExceeded):
		WriteError(w, 504, &Error{Code: "TIMEOUT", What: "Operation timed out"})
	case errors.Is(err, context.Canceled):
		WriteError(w, 408, &Error{Code: "CANCELLED", What: "Operation cancelled"})
	case errors.Is(err, subscription.ErrUpdate):
		WriteError(w, 502, &Error{Code: "SUBSCRIPTION_UPDATE_FAILED", What: "Subscription update failed; previous nodes were retained"})
	case errors.Is(err, configuration.ErrApply):
		WriteError(w, 422, ErrKernelCheckFailed("Configuration validation or startup failed", ""))
	case errors.Is(err, subscription.ErrUnavailable), errors.Is(err, task.ErrClosed):
		WriteError(w, 503, &Error{Code: "UNAVAILABLE", What: "Service is unavailable"})
	default:
		WriteError(w, 500, ErrInternal("Operation failed"))
	}
}
func (s *Server) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	values, err := s.Subscriptions.List(r.Context(), r.URL.Query().Get("group"))
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, values)
}
func (s *Server) getSubscription(w http.ResponseWriter, r *http.Request) {
	value, err := s.Subscriptions.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value)
}
func (s *Server) createSubscription(w http.ResponseWriter, r *http.Request) {
	var value subscription.Subscription
	if err := decode(w, r, &value, false); err != nil {
		fail(w, err)
		return
	}
	if err := s.Subscriptions.Add(r.Context(), &value); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 201, value)
}
func (s *Server) editSubscription(w http.ResponseWriter, r *http.Request) {
	var input subscription.Subscription
	if err := decode(w, r, &input, false); err != nil {
		fail(w, err)
		return
	}
	value, err := s.Subscriptions.Edit(r.Context(), r.PathValue("id"), &input)
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value)
}
func (s *Server) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	if err := s.Subscriptions.Delete(r.Context(), r.PathValue("id")); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) updateSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodes, err := s.Subscriptions.Update(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	sub, err := s.Subscriptions.Get(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, map[string]any{"nodes_added": sub.Diff.Added, "nodes_removed": sub.Diff.Removed, "nodes_changed": sub.Diff.Changed, "nodes_total": len(nodes), "subscription": sub})
}
func (s *Server) groups(w http.ResponseWriter, r *http.Request) {
	values, err := s.Subscriptions.Groups(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, values)
}
func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	values, err := s.Store.Nodes(r.Context(), r.URL.Query().Get("subscription_id"))
	if err != nil {
		fail(w, err)
		return
	}
	result := []*ir.Node{}
	query := r.URL.Query()
	for _, node := range values {
		if protocol := query.Get("protocol"); protocol != "" && node.Protocol != protocol {
			continue
		}
		if search := strings.ToLower(query.Get("search")); search != "" && !strings.Contains(strings.ToLower(node.Name+" "+node.Server.Host), search) {
			continue
		}
		result = append(result, node)
	}
	WriteJSON(w, 200, result)
}
func (s *Server) createNode(w http.ResponseWriter, r *http.Request) {
	var node ir.Node
	if err := decode(w, r, &node, false); err != nil {
		fail(w, err)
		return
	}
	node.ID = ""
	node.Source = ir.Source{Type: "manual"}
	node.Owner = ir.GetProtocolOwner(node.Protocol)
	if err := s.Store.SaveNode(r.Context(), &node); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 201, node)
}
func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.DB().ExecContext(r.Context(), "DELETE FROM nodes WHERE id=?", r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		fail(w, err)
		return
	}
	if count == 0 {
		fail(w, store.ErrNotFound)
		return
	}
	WriteJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) testNode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Type      string `json:"type"`
		URL       string `json:"url"`
		TimeoutMS int    `json:"timeout_ms"`
		Timeout   int    `json:"timeout"`
	}
	if err := decode(w, r, &input, true); err != nil {
		fail(w, err)
		return
	}
	if input.TimeoutMS == 0 {
		input.TimeoutMS = input.Timeout
	}
	result, err := s.Speedtest.Test(r.Context(), r.PathValue("id"), speedtest.Options{Type: input.Type, URL: input.URL, TimeoutMS: input.TimeoutMS})
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, result)
}
func (s *Server) batchTest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		NodeIDs []string `json:"node_ids"`
		speedtest.Options
	}
	if err := decode(w, r, &input, false); err != nil {
		fail(w, err)
		return
	}
	ctx := s.Context
	if ctx == nil {
		ctx = context.Background()
	}
	id, err := s.Speedtest.Batch(ctx, input.NodeIDs, input.Options)
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 202, map[string]string{"task_id": id})
}
func (s *Server) testTask(w http.ResponseWriter, r *http.Request) {
	value, err := s.Speedtest.Task(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value)
}
func (s *Server) cancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Speedtest.Task(id); err != nil {
		fail(w, err)
		return
	}
	if err := s.Speedtest.Queue.Cancel(id); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, map[string]bool{"cancelled": true})
}
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	value, err := s.Config.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value)
}
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var value configuration.Settings
	if err := decode(w, r, &value, false); err != nil {
		fail(w, err)
		return
	}
	if err := s.Config.SaveSettings(r.Context(), value); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value)
}
func (s *Server) currentConfig(w http.ResponseWriter, r *http.Request) {
	value, err := s.Config.Current(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value.Public())
}
func (s *Server) applyConfig(w http.ResponseWriter, r *http.Request) {
	var input configuration.ApplyRequest
	if err := decode(w, r, &input, true); err != nil {
		fail(w, err)
		return
	}
	value, err := s.Config.Apply(r.Context(), input)
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value.Public())
}
func (s *Server) activateNode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Kernel ir.KernelType `json:"kernel"`
	}
	if err := decode(w, r, &input, true); err != nil {
		fail(w, err)
		return
	}
	id := r.PathValue("id")
	if _, err := s.Config.Activate(r.Context(), id, input.Kernel); err != nil {
		fail(w, err)
		return
	}
	node, err := s.Store.Node(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	node.Status = "active"
	WriteJSON(w, 200, node)
}
func (s *Server) activeKernel() string {
	status := s.Kernel.Status()
	for _, name := range []string{"sing-box", "mihomo"} {
		key := name
		if name == "sing-box" {
			key = "singbox"
		}
		if value, ok := status[key].(map[string]any); ok && value["running"] == true {
			return name
		}
	}
	return ""
}
func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("kernel")
	if name == "" {
		name = s.activeKernel()
		if name == "" {
			name = "sing-box"
		}
	}
	if name != "sing-box" && name != "mihomo" {
		fail(w, fmt.Errorf("%w: invalid kernel", store.ErrInvalid))
		return
	}
	limit := 200
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 10000 {
			fail(w, fmt.Errorf("%w: limit must be 1..10000", store.ErrInvalid))
			return
		}
		limit = parsed
	}
	values := []map[string]any{}
	for _, entry := range s.Kernel.GetLogs(name, limit) {
		level := "info"
		if entry.Source == "stderr" {
			level = "error"
		}
		values = append(values, map[string]any{"time": entry.Timestamp, "level": level, "message": entry.Line, "timestamp": entry.Timestamp, "source": entry.Source, "line": entry.Line})
	}
	WriteJSON(w, 200, values)
}
func (s *Server) kernelStatus(w http.ResponseWriter, r *http.Request) {
	raw := s.Kernel.Status()
	name := s.activeKernel()
	value := map[string]any{"running": name != "", "active_kernel": name, "kernels": raw}
	if name != "" {
		key := name
		if name == "sing-box" {
			key = "singbox"
		}
		if state, ok := raw[key].(map[string]any); ok {
			value["pid"] = state["pid"]
			value["started_at"] = state["start_time"]
			if start, ok := state["start_time"].(time.Time); ok {
				value["uptime_seconds"] = int64(time.Since(start).Seconds())
			}
		}
	}
	WriteJSON(w, 200, value)
}
func (s *Server) stopKernel(w http.ResponseWriter, r *http.Request) {
	if err := s.Kernel.StopAll(); err != nil {
		fail(w, err)
		return
	}
	s.kernelStatus(w, r)
}
func (s *Server) connections(w http.ResponseWriter, r *http.Request) {
	state, err := s.Config.Current(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	client := clashapi.NewClient(state.Controller, state.Secret, clashapi.KernelType(state.Kernel))
	defer client.Close()
	values, err := client.GetConnections(r.Context())
	if err != nil {
		WriteError(w, 502, &Error{Code: "KERNEL_UNAVAILABLE", What: "Kernel control API is unavailable"})
		return
	}
	WriteJSON(w, 200, values)
}
