package service

import (
	"net/http"
	"time"
)

// debugResponse is the Debug Endpoint Contract (see docs/decisions) - the one deliberate
// hand-written wire type. Nothing in this repository declares a second copy of this shape to drift
// against: its one real consumer is the GCP uptime check (`infrastructure/AGENTS.md`'s "Monitoring & Alerting"), which matches on
// the literal `"status":"ok"` substring rather than through any type this repo controls. Moving it
// into packages/protos would add a message with nothing on either side to keep it honest -
// exactly the case the Contract Layer exists to prevent, not one it needs to cover.
type debugResponse struct {
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	Environment string `json:"environment"`
	Commit      string `json:"commit"`
}

// Debug serves both /health and /debug.
func (s *Service) Debug(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, debugResponse{
		Status:      "ok",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Environment: s.cfg.Environment,
		Commit:      s.cfg.Commit,
	})
}
