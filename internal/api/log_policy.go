package api

import (
	"fmt"
	"net/http"

	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
)

type logMaintenance interface {
	LogPolicy() kernel.LogPolicy
	ConfigureLogPolicy(kernel.LogPolicy) error
	ClearLogs(string) error
}

func (s *Server) logPolicy(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.Kernel.(logMaintenance)
	if !ok {
		fail(w, fmt.Errorf("%w: log maintenance is unavailable", store.ErrInvalid))
		return
	}
	if r.Method == http.MethodGet {
		WriteJSON(w, 200, manager.LogPolicy())
		return
	}
	var policy kernel.LogPolicy
	if err := decode(w, r, &policy, false); err != nil {
		fail(w, err)
		return
	}
	if policy.RetentionDays < 1 || policy.RetentionDays > 365 || policy.CleanupHours < 0 || policy.CleanupHours > 720 {
		fail(w, fmt.Errorf("%w: retention_days must be 1..365 and cleanup_hours must be 0..720", store.ErrInvalid))
		return
	}
	if err := s.Store.SaveState(r.Context(), "kernel_log_policy", policy); err != nil {
		fail(w, err)
		return
	}
	if err := manager.ConfigureLogPolicy(policy); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, manager.LogPolicy())
}

func (s *Server) clearLogs(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.Kernel.(logMaintenance)
	if !ok {
		fail(w, fmt.Errorf("%w: log maintenance is unavailable", store.ErrInvalid))
		return
	}
	query, err := s.parseLogQuery(r)
	if err != nil {
		fail(w, err)
		return
	}
	if err := manager.ClearLogs(query.Kernel); err != nil {
		fail(w, err)
		return
	}
	WriteJSON(w, 200, map[string]any{"cleared": true})
}
