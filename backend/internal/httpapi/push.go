package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jeeth/rss-reader/backend/internal/gravitas"
	"github.com/jeeth/rss-reader/backend/internal/push"
	"net/http"
)

func (a *API) pushBinding(r *http.Request) string {
	if token := bearerToken(r); token != "" {
		return token
	}
	if cookie, err := r.Cookie(a.sessionCookieName()); err == nil {
		return cookie.Value
	}
	return ""
}
func (a *API) pushRoutes(mux *http.ServeMux) {
	if a.cfg.Push == nil {
		return
	}
	a.cfg.Push.NewsReady = a.store.PushFeedsReady
	a.cfg.Push.Validate = func(ctx context.Context, d push.Device) (bool, error) {
		if a.cfg.Federation != nil {
			auth, err := a.cfg.Federation.AuthenticatePushAuthorization(ctx, d.Binding)
			if errors.Is(err, gravitas.ErrUnavailable) {
				return false, err
			}
			if err != nil {
				return false, nil
			}
			return a.store.PushIdentityMatches(ctx, d.Owner, auth.Issuer, auth.Subject)
		}
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://news.gravitas.cool/", nil)
		user, err := a.authenticate(request, d.Binding)
		return err == nil && user.ID == d.Owner, nil
	}
	mux.Handle("GET /v1/push/key", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, map[string]string{"publicKey": a.cfg.Push.PublicKey(), "owner": currentUser(r.Context()).ID})
	})))
	mux.Handle("POST /v1/push/subscription", a.auth(a.requireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var subscription push.Subscription
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&subscription); err != nil {
			writeError(w, 400, "INVALID_PARAMS", push.ErrInvalid)
			return
		}
		user := currentUser(r.Context())
		binding, expires := a.pushBinding(r), user.SessionExpiresAt
		if a.cfg.Federation != nil {
			bound, e := a.cfg.Federation.AppSession(r, a.sessionCookieName())
			if e != nil {
				writeError(w, 401, "UNAUTHORIZED", e)
				return
			}
			binding, expires, e = a.cfg.Federation.SealPushAuthorization(bound)
			if e != nil {
				writeError(w, 500, "INTERNAL", e)
				return
			}
		}
		if err := a.cfg.Push.Subscribe(push.Device{Owner: user.ID, Binding: binding, Session: push.SessionKey(a.pushBinding(r)), Expires: expires, Subscription: subscription}); err != nil {
			writeError(w, 400, "INVALID_PARAMS", err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))))
	mux.Handle("DELETE /v1/push/subscription", a.auth(a.requireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
			writeError(w, 400, "INVALID_PARAMS", push.ErrInvalid)
			return
		}
		if err := a.cfg.Push.Unsubscribe(currentUser(r.Context()).ID, body.Endpoint); err != nil {
			writeError(w, 500, "INTERNAL", err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))))
}
