package http

import (
	"context"
	"net/http"
)

// handleSSOStatus exposes whether Azure AD SSO is configured. Clients use
// this to show/hide the Microsoft sign-in button.
func (s *Server) handleSSOStatus(w http.ResponseWriter, r *http.Request) {
	enabled := s.azureSSOEnabled(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled})
}

func (s *Server) azureSSOEnabled(ctx context.Context) bool {
	return s.set.GetOr(ctx, "azure_enabled", "false") == "true"
}
