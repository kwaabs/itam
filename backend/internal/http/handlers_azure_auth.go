package http

import (
	"errors"
	"net/http"
	"net/url"

	"itam/internal/auth"
)

// handleAzureStart redirects the browser (or native system browser) to
// Entra ID's authorization endpoint. redirect_to is where the callback sends
// the user back afterward - a web origin, or the itam:// deep link scheme.
func (s *Server) handleAzureStart(w http.ResponseWriter, r *http.Request) {
	redirectTo := r.URL.Query().Get("redirect_to")
	if redirectTo == "" {
		writeErr(w, http.StatusBadRequest, "redirect_to is required")
		return
	}
	authorizeURL, err := s.auth.AzureStartURL(r.Context(), redirectTo)
	if err != nil {
		if errors.Is(err, auth.ErrAzureNotConfigured) {
			writeErr(w, http.StatusServiceUnavailable, "azure sso is not configured")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

// handleAzureCallback completes the OIDC flow and redirects back to the
// client with tokens in the URL fragment on success (matching the shape
// GoTrue's OAuth redirect used, so captureOAuthRedirect() and the mobile
// deep-link parser need no changes), or a ?sso_error= query param on failure.
func (s *Server) handleAzureCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if errMsg := q.Get("error_description"); errMsg != "" {
		s.redirectSSOError(w, r, q.Get("state"), errMsg)
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || state == "" {
		s.redirectSSOError(w, r, state, "missing code or state")
		return
	}

	pair, redirectTo, err := s.auth.AzureCallback(r.Context(), code, state)
	if err != nil {
		if redirectTo != "" {
			s.redirectSSOError(w, r, state, "sign-in failed")
			return
		}
		writeErr(w, http.StatusBadRequest, "invalid sso callback")
		return
	}

	dest, err := url.Parse(redirectTo)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "invalid redirect target")
		return
	}
	frag := url.Values{}
	frag.Set("access_token", pair.AccessToken)
	frag.Set("refresh_token", pair.RefreshToken)
	dest.Fragment = frag.Encode()
	http.Redirect(w, r, dest.String(), http.StatusFound)
}

// redirectSSOError best-effort recovers redirect_to from the (possibly still
// valid, possibly not) state so the user lands back on a page that can show
// the error rather than a bare API error response.
func (s *Server) redirectSSOError(w http.ResponseWriter, r *http.Request, state, message string) {
	redirectTo := s.auth.AzureStateRedirectTo(state)
	if redirectTo == "" {
		writeErr(w, http.StatusBadRequest, message)
		return
	}
	dest, err := url.Parse(redirectTo)
	if err != nil {
		writeErr(w, http.StatusBadRequest, message)
		return
	}
	q := dest.Query()
	q.Set("sso_error", message)
	dest.RawQuery = q.Encode()
	http.Redirect(w, r, dest.String(), http.StatusFound)
}
