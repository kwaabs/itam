package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// handleSSOStatus exposes whether GoTrue has Azure AD enabled. Clients use this
// to show the Microsoft button; OAuth itself always goes through GoTrue.
func (s *Server) handleSSOStatus(w http.ResponseWriter, r *http.Request) {
	enabled := s.gotrueAzureEnabled(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled})
}

func (s *Server) gotrueAzureEnabled(ctx context.Context) bool {
	if s.cfg.AzureEnabled {
		return true
	}
	url := strings.TrimRight(s.cfg.GoTrueURL, "/") + "/settings"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	var settings struct {
		External struct {
			Azure bool `json:"azure"`
		} `json:"external"`
	}
	if err := json.Unmarshal(body, &settings); err != nil {
		return false
	}
	return settings.External.Azure
}
