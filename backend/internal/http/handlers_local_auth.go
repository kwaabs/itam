package http

import (
	"net/http"
	"strings"
)

// handleLogin authenticates against iam.user_profiles.password_hash and
// issues an access/refresh token pair. Public route - no auth middleware.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(in.Email) == "" || in.Password == "" {
		writeErr(w, http.StatusBadRequest, "email and password are required")
		return
	}
	pair, err := s.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// handleRefresh rotates a refresh token for a new access/refresh pair.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.RefreshToken) == "" {
		writeErr(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	pair, err := s.auth.Refresh(r.Context(), in.RefreshToken)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// handleLogout revokes a refresh token. Always 204 - logout isn't an error
// path even if the token was already invalid.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = decode(r, &in)
	if strings.TrimSpace(in.RefreshToken) != "" {
		_ = s.auth.Logout(r.Context(), in.RefreshToken)
	}
	w.WriteHeader(http.StatusNoContent)
}
