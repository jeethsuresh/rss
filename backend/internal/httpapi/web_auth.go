package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

const webSessionCookie = "rss_session"

func (a *API) webConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"registrationEnabled": a.cfg.RegistrationEnabled,
		"version":             a.cfg.Version,
	})
}

func (a *API) webRegister(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthAttempt(r) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", errors.New("too many authentication attempts"))
		return
	}
	if !a.cfg.RegistrationEnabled {
		writeError(w, http.StatusForbidden, "REGISTRATION_DISABLED", errors.New("registration is disabled"))
		return
	}
	var body credentials
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.Register(r.Context(), body.Username, body.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	a.setSessionCookie(w, result)
	writeJSON(w, http.StatusCreated, webAuthResponse(result))
}

func (a *API) webLogin(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthAttempt(r) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", errors.New("too many authentication attempts"))
		return
	}
	var body credentials
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	result, err := a.store.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", errors.New("invalid username or password"))
		return
	}
	a.setSessionCookie(w, result)
	writeJSON(w, http.StatusOK, webAuthResponse(result))
}

func (a *API) webSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"user": currentUser(r.Context())})
}

func (a *API) webLogout(w http.ResponseWriter, r *http.Request) {
	token := ""
	if cookie, err := r.Cookie(webSessionCookie); err == nil {
		token = cookie.Value
	}
	if token == "" {
		token = bearerToken(r)
	}
	if err := a.store.RevokeSession(r.Context(), token); err != nil {
		writeDomainError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: webSessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) setSessionCookie(w http.ResponseWriter, result *serverstore.AuthResult) {
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name: webSessionCookie, Value: result.Token, Path: "/", MaxAge: maxAge,
		Expires: result.ExpiresAt, HttpOnly: true, Secure: a.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func webAuthResponse(result *serverstore.AuthResult) map[string]any {
	return map[string]any{"user": result.User, "expiresAt": result.ExpiresAt}
}

func (a *API) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A custom header makes cross-origin form submissions impossible; no CORS
		// policy is enabled, so browsers cannot add it from another origin.
		if strings.TrimSpace(r.Header.Get("X-RSS-CSRF")) != "1" {
			writeError(w, http.StatusForbidden, "CSRF_REQUIRED", errors.New("missing request verification header"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
