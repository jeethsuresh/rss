package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/gravitas"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
)

const webSessionCookie = "rss_session"

func (a *API) sessionCookieName() string {
	if a.cfg.Federation != nil {
		return "__Host-rss_session"
	}
	return webSessionCookie
}
func (a *API) secureSessionCookie() bool { return a.cfg.CookieSecure || a.cfg.Federation != nil }

func (a *API) webConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"registrationEnabled": a.registrationEnabled(),
		"federated":           a.cfg.Federation != nil,
		"autoSignIn":          a.cfg.Federation != nil && gravitas.HasSharedSignInHint(r),
		"version":             a.cfg.Version,
	})
}

func (a *API) webRegister(w http.ResponseWriter, r *http.Request) {
	if !a.allowAuthAttempt(r) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", errors.New("too many authentication attempts"))
		return
	}
	if !a.registrationEnabled() {
		writeError(w, http.StatusForbidden, "REGISTRATION_DISABLED", errors.New("registration is disabled"))
		return
	}
	var body registrationCredentials
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMS", err)
		return
	}
	if !a.authorizeInvite(w, body.InviteKey) {
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
	if a.requireFederatedLogin(w) {
		return
	}
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
	if cookie, err := r.Cookie(a.sessionCookieName()); err == nil {
		token = cookie.Value
	}
	if token == "" {
		token = bearerToken(r)
	}
	if a.cfg.Federation != nil {
		decoded, _ := a.cfg.Federation.DecodeAppSession(token)
		if decoded.LocalToken != "" {
			token = decoded.LocalToken
		}
	}
	if err := a.store.RevokeSession(r.Context(), token); err != nil {
		writeDomainError(w, err)
		return
	}
	a.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: a.sessionCookieName(), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.secureSessionCookie(), SameSite: http.SameSiteStrictMode,
	})
}

func (a *API) setSessionCookie(w http.ResponseWriter, result *serverstore.AuthResult) {
	w.Header().Set("Cache-Control", "no-store")
	sameSite := http.SameSiteStrictMode
	if a.cfg.Federation != nil {
		sameSite = http.SameSiteLaxMode
	}
	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name: a.sessionCookieName(), Value: result.Token, Path: "/", MaxAge: maxAge,
		Expires: result.ExpiresAt, HttpOnly: true, Secure: a.secureSessionCookie(),
		SameSite: sameSite,
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
