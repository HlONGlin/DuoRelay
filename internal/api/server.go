package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holi/duorelay/internal/auth"
	"github.com/holi/duorelay/internal/clashapi"
	"github.com/holi/duorelay/internal/configuration"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/routing"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/subscription"
	"github.com/holi/duorelay/internal/task"
	embeddedweb "github.com/holi/duorelay/web"
)

var (
	logUUIDPattern   = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)
	logNumberPattern = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
)

type KernelReader interface {
	Status() map[string]any
	GetLogs(string, int) []kernel.LogEntry
	StopAll() error
}

type logSubscriber interface {
	SubscribeLogs(string) (<-chan kernel.LogEntry, func(), error)
}
type Server struct {
	Store         *store.Store
	Subscriptions *subscription.Manager
	Speedtest     *speedtest.Engine
	Config        *configuration.Service
	Kernel        KernelReader
	Auth          *auth.Service
	CookieSecure  bool
	loginLimiter  *loginLimiter
	Context       context.Context
	mux           *http.ServeMux
}

func NewServer(s *store.Store, subs *subscription.Manager, speed *speedtest.Engine, config *configuration.Service, runtime KernelReader) *Server {
	server := &Server{Store: s, Subscriptions: subs, Speedtest: speed, Config: config, Kernel: runtime, Auth: auth.New(s), Context: context.Background(), mux: http.NewServeMux(), loginLimiter: newLoginLimiter()}
	server.routes()
	return server
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	protected := strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/auth/")
	protected = protected || r.URL.Path == "/ws/logs"
	if protected && s.Auth != nil {
		status, err := s.Auth.Status(r.Context(), r)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, ErrInternal("authentication status unavailable"))
			return
		}
		if !status.Configured {
			WriteError(w, http.StatusPreconditionRequired, &Error{Code: "AUTH_SETUP_REQUIRED", What: "请先创建管理员账号"})
			return
		}
		if !status.Authenticated {
			WriteError(w, http.StatusUnauthorized, &Error{Code: "AUTH_REQUIRED", What: "请先登录"})
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws/logs" {
		s.mux.ServeHTTP(w, r)
		return
	}
	s.frontend(w, r)
}

func (s *Server) frontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		WriteError(w, http.StatusMethodNotAllowed, &Error{Code: "METHOD_NOT_ALLOWED", What: "请求方法不支持"})
		return
	}
	frontend, err := fs.Sub(embeddedweb.Dist, "dist")
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrNotFound("frontend"))
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" || strings.HasPrefix(name, "..") {
		name = "index.html"
	}
	data, err := fs.ReadFile(frontend, name)
	if err != nil {
		data, err = fs.ReadFile(frontend, "index.html")
		name = "index.html"
	}
	if err != nil {
		WriteError(w, http.StatusNotFound, ErrNotFound("frontend"))
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(data)
}

type authCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.Auth.Status(r.Context(), r)
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, status)
}

func (s *Server) authRegister(w http.ResponseWriter, r *http.Request) {
	var credentials authCredentials
	if err := decode(w, r, &credentials, false); err != nil {
		fail(w, err)
		return
	}
	if err := s.Auth.Register(r.Context(), credentials.Username, credentials.Password); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"registered": true})
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	var credentials authCredentials
	if err := decode(w, r, &credentials, false); err != nil {
		fail(w, err)
		return
	}
	clientIP := requestClientIP(r)
	if allowed, retryAfter := s.loginLimiter.allow(clientIP, time.Now()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		WriteError(w, http.StatusTooManyRequests, &Error{Code: "RATE_LIMITED", What: "登录失败次数过多，请稍后重试"})
		return
	}
	token, err := s.Auth.Login(r.Context(), credentials.Username, credentials.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalid) {
			s.loginLimiter.fail(clientIP, time.Now())
			WriteError(w, http.StatusUnauthorized, &Error{Code: "AUTH_INVALID", What: "用户名或密码错误"})
			return
		}
		fail(w, err)
		return
	}
	s.loginLimiter.success(clientIP)
	secureCookie := s.CookieSecure || r.TLS != nil
	auth.SetSessionCookie(w, token, secureCookie)
	WriteJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": strings.TrimSpace(credentials.Username)})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.Logout(r.Context(), r); err != nil {
		fail(w, err)
		return
	}
	auth.ClearSessionCookie(w, s.CookieSecure || r.TLS != nil)
	WriteJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/auth/status", s.authStatus)
	s.mux.HandleFunc("POST /api/auth/register", s.authRegister)
	s.mux.HandleFunc("POST /api/auth/login", s.authLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.authLogout)
	s.mux.HandleFunc("GET /api/subscriptions", s.listSubscriptions)
	s.mux.HandleFunc("POST /api/subscriptions", s.createSubscription)
	s.mux.HandleFunc("GET /api/subscriptions/{id}", s.getSubscription)
	s.mux.HandleFunc("PUT /api/subscriptions/{id}", s.editSubscription)
	s.mux.HandleFunc("DELETE /api/subscriptions/{id}", s.deleteSubscription)
	s.mux.HandleFunc("POST /api/subscriptions/{id}/update", s.updateSubscription)
	s.mux.HandleFunc("GET /api/subscription-groups", s.groups)
	s.mux.HandleFunc("GET /api/nodes", s.listNodes)
	s.mux.HandleFunc("POST /api/nodes", s.createNode)
	s.mux.HandleFunc("POST /api/nodes/import", s.importNodes)
	s.mux.HandleFunc("DELETE /api/nodes/{id}", s.deleteNode)
	s.mux.HandleFunc("POST /api/nodes/{id}/test", s.testNode)
	s.mux.HandleFunc("POST /api/nodes/{id}/activate", s.activateNode)
	s.mux.HandleFunc("POST /api/speedtest/batch", s.batchTest)
	s.mux.HandleFunc("GET /api/speedtest/tasks/{id}", s.testTask)
	s.mux.HandleFunc("DELETE /api/speedtest/tasks/{id}", s.cancelTask)
	s.mux.HandleFunc("GET /api/config", s.settings)
	s.mux.HandleFunc("PUT /api/config", s.saveSettings)
	s.mux.HandleFunc("GET /api/routing/export", s.exportRouting)
	s.mux.HandleFunc("POST /api/routing/validate", s.validateRoutingText)
	s.mux.HandleFunc("POST /api/routing/import", s.importRouting)
	s.mux.HandleFunc("GET /api/config/current", s.currentConfig)
	s.mux.HandleFunc("GET /api/config/proxy-group", s.proxyGroupStatus)
	s.mux.HandleFunc("POST /api/config/apply", s.applyConfig)
	s.mux.HandleFunc("POST /api/config/chain/preview", s.previewChain)
	s.mux.HandleFunc("GET /api/kernel/logs", s.logs)
	s.mux.HandleFunc("GET /api/kernel/logs/summary", s.logSummary)
	s.mux.HandleFunc("GET /ws/logs", s.logStream)
	s.mux.HandleFunc("GET /api/kernel/status", s.kernelStatus)
	s.mux.HandleFunc("POST /api/kernel/start", s.applyConfig)
	s.mux.HandleFunc("POST /api/kernel/stop", s.stopKernel)
	s.mux.HandleFunc("GET /api/kernel/connections", s.connections)
	s.mux.HandleFunc("GET /api/kernel/traffic", s.traffic)
	s.mux.HandleFunc("GET /api/kernel/traffic/history", s.trafficHistory)
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
		why := strings.Join(strings.Fields(strings.TrimSpace(err.Error())), " ")
		if len(why) > 500 {
			why = why[:500] + "..."
		}
		WriteError(w, 422, ErrKernelCheckFailed(why, ""))
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
	query := r.URL.Query()
	filter := store.NodeListQuery{
		SubscriptionID: query.Get("subscription_id"),
		Protocol:       query.Get("protocol"),
		Search:         query.Get("search"),
	}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 5000 {
			fail(w, fmt.Errorf("%w: limit must be 1..5000", store.ErrInvalid))
			return
		}
		filter.Limit = parsed
	}
	if value := query.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 1000000 {
			fail(w, fmt.Errorf("%w: offset must be 0..1000000", store.ErrInvalid))
			return
		}
		filter.Offset = parsed
	}
	values, err := s.Store.NodesQuery(r.Context(), filter)
	if err != nil {
		fail(w, err)
		return
	}
	// Node status is runtime state, not durable node metadata. Older databases
	// may have persisted "active" on several nodes, so derive it from the
	// currently applied configuration before returning the list.
	activeIDs := map[string]struct{}{}
	if s.Config != nil {
		if current, currentErr := s.Config.Current(r.Context()); currentErr == nil && current != nil {
			for _, id := range current.NodeIDs {
				activeIDs[id] = struct{}{}
			}
		}
	}
	for _, node := range values {
		if _, ok := activeIDs[node.ID]; ok {
			node.Status = "active"
		} else {
			node.Status = "idle"
		}
	}
	WriteJSON(w, 200, values)
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

func (s *Server) importNodes(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content string `json:"content"`
	}
	if err := decode(w, r, &input, false); err != nil {
		fail(w, err)
		return
	}
	if strings.TrimSpace(input.Content) == "" {
		fail(w, fmt.Errorf("%w: content is required", store.ErrInvalid))
		return
	}
	if s.Subscriptions == nil {
		fail(w, fmt.Errorf("%w: subscription parser is not configured", store.ErrInvalid))
		return
	}
	nodes, err := s.Subscriptions.Parse([]byte(input.Content))
	if err != nil {
		fail(w, fmt.Errorf("%w: %v", store.ErrInvalid, err))
		return
	}
	if len(nodes) == 0 {
		fail(w, fmt.Errorf("%w: content contains no usable nodes", store.ErrInvalid))
		return
	}
	for _, node := range nodes {
		node.ID = ""
		node.Source = ir.Source{Type: "import"}
		node.Owner = ir.GetProtocolOwner(node.Protocol)
		if err := s.Store.SaveNode(r.Context(), node); err != nil {
			fail(w, err)
			return
		}
	}
	WriteJSON(w, 201, map[string]any{"nodes": nodes, "nodes_total": len(nodes)})
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

func (s *Server) exportRouting(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Config.Settings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"text": routing.Format(settings.RoutingRules)})
}

func (s *Server) validateRoutingText(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Text string `json:"text"`
	}
	if err := decode(w, r, &input, false); err != nil {
		fail(w, err)
		return
	}
	result := routing.Parse(input.Text)
	WriteJSON(w, http.StatusOK, result)
}

func (s *Server) importRouting(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Text string `json:"text"`
	}
	if err := decode(w, r, &input, false); err != nil {
		fail(w, err)
		return
	}
	result := routing.Parse(input.Text)
	// Import is parse-only. Existing settings are not changed until the user
	// submits the returned valid rules through PUT /api/config.
	WriteJSON(w, http.StatusOK, result)
}
func (s *Server) currentConfig(w http.ResponseWriter, r *http.Request) {
	value, err := s.Config.Current(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, value.Public())
}

func (s *Server) proxyGroupStatus(w http.ResponseWriter, r *http.Request) {
	state, err := s.Config.Current(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	if state.Mode != "load_balance" || state.Kernel != ir.KernelMihomo {
		WriteJSON(w, http.StatusOK, map[string]any{"active": false})
		return
	}
	client := clashapi.NewClient(state.Controller, state.Secret, clashapi.KernelMihomo)
	defer client.Close()
	proxies, err := client.GetProxies(r.Context())
	if err != nil {
		WriteError(w, http.StatusBadGateway, &Error{Code: "KERNEL_UNAVAILABLE", What: "无法读取自动选优和故障切换状态"})
		return
	}
	group, ok := proxies.Proxies["PROXY"]
	if !ok {
		WriteError(w, http.StatusBadGateway, &Error{Code: "PROXY_GROUP_UNAVAILABLE", What: "当前内核未提供代理组状态"})
		return
	}
	members := make([]map[string]any, 0, len(group.All))
	for _, name := range group.All {
		proxy, exists := proxies.Proxies[name]
		member := map[string]any{"name": name, "alive": exists && proxy.Alive}
		if exists && len(proxy.History) > 0 {
			member["delay_ms"] = proxy.History[len(proxy.History)-1].Delay
		}
		members = append(members, member)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"active": true, "now": group.Now, "members": members})
}

func (s *Server) applyConfig(w http.ResponseWriter, r *http.Request) {
	var input configuration.ApplyRequest
	if err := decode(w, r, &input, true); err != nil {
		fail(w, err)
		return
	}
	if input.Content != "" {
		fail(w, fmt.Errorf("%w: raw kernel configuration is not accepted through this API; use nodes and routing settings", store.ErrInvalid))
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
	if s.Config != nil {
		if state, err := s.Config.Current(context.Background()); err == nil && state != nil {
			name := string(state.Kernel)
			key := name
			if name == "sing-box" {
				key = "singbox"
			}
			if value, ok := status[key].(map[string]any); ok && value["running"] == true {
				return name
			}
			if state.Chain != nil {
				return ""
			}
		}
	}
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

func (s *Server) previewChain(w http.ResponseWriter, r *http.Request) {
	var request configuration.ApplyRequest
	if err := decode(w, r, &request, false); err != nil {
		fail(w, err)
		return
	}
	plan, err := s.Config.PreviewChain(r.Context(), request)
	if err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, plan)
}
func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	query, err := s.parseLogQuery(r)
	if err != nil {
		fail(w, err)
		return
	}
	values := s.filteredLogs(query)
	WriteJSON(w, 200, values)
}

func (s *Server) logStream(w http.ResponseWriter, r *http.Request) {
	query, err := s.parseLogQuery(r)
	if err != nil {
		fail(w, err)
		return
	}
	subscriber, ok := s.Kernel.(logSubscriber)
	if !ok {
		WriteError(w, http.StatusNotImplemented, &Error{Code: "LOG_STREAM_UNAVAILABLE", What: "实时日志流不可用"})
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 8192,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	entries, cancel, err := subscriber.SubscribeLogs(query.Kernel)
	if err != nil {
		_ = conn.WriteJSON(map[string]any{"type": "error", "message": err.Error()})
		return
	}
	defer cancel()

	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return
	}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	write := func(value any) error {
		if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
		return conn.WriteJSON(value)
	}
	initial := s.filteredLogs(query)
	if err := write(map[string]any{"type": "snapshot", "logs": initial}); err != nil {
		return
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-readDone:
			return
		case entry, open := <-entries:
			if !open {
				return
			}
			if !matchesLog(query, entry) {
				continue
			}
			if err := write(map[string]any{"type": "log", "log": logEntryJSON(entry, classifyLogLevel(entry))}); err != nil {
				return
			}
		case <-ticker.C:
			// A client that only receives server heartbeats is still alive. Refresh
			// the read deadline before sending the next ping so idle dashboards do
			// not get disconnected after the initial 60-second deadline.
			if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
				return
			}
			if err := write(map[string]any{"type": "ping", "time": time.Now().UTC()}); err != nil {
				return
			}
		}
	}
}

type logQuery struct {
	Kernel string
	Limit  int
	Level  string
	Source string
	Search string
}

func (s *Server) parseLogQuery(r *http.Request) (logQuery, error) {
	query := r.URL.Query()
	name := query.Get("kernel")
	if name == "" {
		name = s.activeKernel()
		if name == "" {
			name = "sing-box"
		}
	}
	if name != "sing-box" && name != "mihomo" {
		return logQuery{}, fmt.Errorf("%w: invalid kernel", store.ErrInvalid)
	}
	result := logQuery{Kernel: name, Limit: 200, Level: strings.ToLower(strings.TrimSpace(query.Get("level"))), Source: strings.ToLower(strings.TrimSpace(query.Get("source"))), Search: strings.TrimSpace(query.Get("search"))}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 10000 {
			return logQuery{}, fmt.Errorf("%w: limit must be 1..10000", store.ErrInvalid)
		}
		result.Limit = parsed
	}
	if result.Level != "" && result.Level != "all" && !validLogLevel(result.Level) {
		return logQuery{}, fmt.Errorf("%w: invalid log level", store.ErrInvalid)
	}
	if result.Level == "all" {
		result.Level = ""
	}
	if result.Source != "" && result.Source != "stdout" && result.Source != "stderr" {
		return logQuery{}, fmt.Errorf("%w: invalid log source", store.ErrInvalid)
	}
	return result, nil
}

func (s *Server) filteredLogs(query logQuery) []map[string]any {
	entries := s.Kernel.GetLogs(query.Kernel, 0)
	if query.Search != "" {
		query.Search = strings.ToLower(query.Search)
	}
	filtered := make([]map[string]any, 0, min(query.Limit, len(entries)))
	for _, entry := range entries {
		if !matchesLog(query, entry) {
			continue
		}
		filtered = append(filtered, logEntryJSON(entry, classifyLogLevel(entry)))
	}
	if len(filtered) > query.Limit {
		filtered = filtered[len(filtered)-query.Limit:]
	}
	return filtered
}

func matchesLog(query logQuery, entry kernel.LogEntry) bool {
	level := classifyLogLevel(entry)
	if query.Level != "" && query.Level != level {
		return false
	}
	if query.Source != "" && query.Source != strings.ToLower(entry.Source) {
		return false
	}
	return query.Search == "" || strings.Contains(strings.ToLower(entry.Line), strings.ToLower(query.Search))
}

func (s *Server) logSummary(w http.ResponseWriter, r *http.Request) {
	query, err := s.parseLogQuery(r)
	if err != nil {
		fail(w, err)
		return
	}
	entries := s.Kernel.GetLogs(query.Kernel, 0)
	levels := map[string]int{"debug": 0, "info": 0, "warn": 0, "error": 0}
	sources := map[string]int{"stdout": 0, "stderr": 0}
	type cluster struct {
		Message  string    `json:"message"`
		Level    string    `json:"level"`
		Count    int       `json:"count"`
		LastSeen time.Time `json:"last_seen"`
	}
	clusters := make(map[string]*cluster)
	for _, entry := range entries {
		level := classifyLogLevel(entry)
		if query.Level != "" && query.Level != level {
			continue
		}
		if query.Source != "" && query.Source != strings.ToLower(entry.Source) {
			continue
		}
		if query.Search != "" && !strings.Contains(strings.ToLower(entry.Line), strings.ToLower(query.Search)) {
			continue
		}
		levels[level]++
		source := strings.ToLower(entry.Source)
		if _, ok := sources[source]; !ok {
			sources[source] = 0
		}
		sources[source]++
		key := normalizeLogMessage(entry.Line)
		item := clusters[key]
		if item == nil {
			item = &cluster{Message: entry.Line, Level: level, LastSeen: entry.Timestamp}
			clusters[key] = item
		}
		item.Count++
		if entry.Timestamp.After(item.LastSeen) {
			item.LastSeen = entry.Timestamp
		}
	}
	result := make([]cluster, 0, len(clusters))
	for _, item := range clusters {
		result = append(result, *item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].LastSeen.After(result[j].LastSeen)
	})
	if len(result) > 20 {
		result = result[:20]
	}
	WriteJSON(w, 200, map[string]any{"kernel": query.Kernel, "total": sumLogCounts(levels), "levels": levels, "sources": sources, "clusters": result})
}

func logEntryJSON(entry kernel.LogEntry, level string) map[string]any {
	return map[string]any{"time": entry.Timestamp, "level": level, "message": entry.Line, "timestamp": entry.Timestamp, "source": entry.Source, "line": entry.Line}
}

func classifyLogLevel(entry kernel.LogEntry) string {
	line := strings.ToLower(strings.TrimSpace(entry.Line))
	if strings.Contains(line, "debug") || strings.Contains(line, "trace") {
		return "debug"
	}
	if strings.Contains(line, "warn") || strings.Contains(line, "warning") {
		return "warn"
	}
	if strings.Contains(line, "error") || strings.Contains(line, "fatal") || strings.Contains(line, "panic") || strings.Contains(line, "failed") {
		return "error"
	}
	if strings.Contains(line, "info") {
		return "info"
	}
	if entry.Source == "stderr" {
		return "error"
	}
	return "info"
}

func validLogLevel(level string) bool {
	return level == "debug" || level == "info" || level == "warn" || level == "error"
}

func normalizeLogMessage(message string) string {
	message = strings.ToLower(strings.TrimSpace(message))
	message = logUUIDPattern.ReplaceAllString(message, "<id>")
	message = logNumberPattern.ReplaceAllString(message, "<n>")
	return strings.Join(strings.Fields(message), " ")
}

func sumLogCounts(values map[string]int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
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
	if s.Config != nil {
		if err := s.Config.Stop(r.Context()); err != nil {
			fail(w, err)
			return
		}
	} else if err := s.Kernel.StopAll(); err != nil {
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

func (s *Server) traffic(w http.ResponseWriter, r *http.Request) {
	state, err := s.Config.Current(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, 503, &Error{Code: "KERNEL_UNAVAILABLE", What: "Kernel control API is unavailable"})
			return
		}
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	client := clashapi.NewClient(state.Controller, state.Secret, clashapi.KernelType(state.Kernel))
	defer client.Close()
	value, err := client.ReadTraffic(ctx)
	if err != nil {
		WriteError(w, 502, &Error{Code: "KERNEL_UNAVAILABLE", What: "Kernel control API is unavailable"})
		return
	}
	sampledAt := time.Now().UTC()
	if err := s.Store.SaveTrafficSample(r.Context(), store.TrafficSample{Kernel: string(state.Kernel), Up: value.Up, Down: value.Down, SampledAt: sampledAt}); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, clashapi.TrafficSnapshot{TrafficData: *value, SampledAt: sampledAt})
}

func (s *Server) trafficHistory(w http.ResponseWriter, r *http.Request) {
	state, err := s.Config.Current(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteJSON(w, http.StatusOK, []clashapi.TrafficSnapshot{})
			return
		}
		fail(w, err)
		return
	}
	limit := 60
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1 || parsed > 20000 {
			fail(w, fmt.Errorf("%w: limit must be 1..20000", store.ErrInvalid))
			return
		}
		limit = parsed
	}
	samples, err := s.Store.TrafficSamples(r.Context(), string(state.Kernel), limit)
	if err != nil {
		fail(w, err)
		return
	}
	result := make([]clashapi.TrafficSnapshot, 0, len(samples))
	for _, sample := range samples {
		result = append(result, clashapi.TrafficSnapshot{TrafficData: clashapi.TrafficData{Up: sample.Up, Down: sample.Down}, SampledAt: sample.SampledAt})
	}
	WriteJSON(w, http.StatusOK, result)
}
