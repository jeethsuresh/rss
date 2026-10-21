package httpapi

import (
	"errors"
	"github.com/jeeth/rss-reader/backend/internal/gravitas"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"net/http"
	"net/url"
)

func (a *API) authenticate(r *http.Request, token string) (*serverstore.User, error) {
	if a.cfg.Federation != nil {
		return a.store.AuthenticateFederated(r.Context(), token, a.cfg.Federation.Issuer)
	}
	return a.store.Authenticate(r.Context(), token)
}
func (a *API) requireFederatedLogin(w http.ResponseWriter) bool {
	if a.cfg.Federation == nil {
		return false
	}
	writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "FEDERATED_LOGIN_REQUIRED", "message": "Sign in with Gravitas", "loginUrl": "/auth/gravitas/start"}})
	return true
}
func (a *API) federationStart(w http.ResponseWriter, r *http.Request) { a.startFederation(w, r, false) }
func (a *API) federationRegister(w http.ResponseWriter, r *http.Request) {
	a.startFederation(w, r, true)
}
func (a *API) startFederation(w http.ResponseWriter, r *http.Request, register bool) {
	if a.cfg.Federation == nil {
		http.NotFound(w, r)
		return
	}
	if err := a.cfg.Federation.Start(w, r, register); err != nil {
		writeError(w, http.StatusServiceUnavailable, "IAM_UNAVAILABLE", errors.New("Gravitas sign-in is temporarily unavailable; please try again"))
	}
}
func (a *API) federationCallback(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Federation == nil {
		http.NotFound(w, r)
		return
	}
	identity, next, err := a.cfg.Federation.Callback(w, r)
	if err != nil {
		if errors.Is(err, gravitas.ErrLoginRequired) || errors.Is(err, gravitas.ErrConsentRequired) {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(gravitas.SafeNext(next)), http.StatusSeeOther)
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_LOGIN", errors.New("Sign-in could not be completed; start again from the RSS login page"))
		return
	}
	result, err := a.store.LoginFederated(r.Context(), identity.Issuer, identity.Subject, identity.UserID)
	if err != nil {
		a.log.Error("create federated RSS session", "err", err)
		writeError(w, http.StatusInternalServerError, "SESSION_ERROR", errors.New("Unable to create the RSS session"))
		return
	}
	a.setSessionCookie(w, result)
	http.Redirect(w, r, gravitas.SafeNext(next), http.StatusSeeOther)
}
