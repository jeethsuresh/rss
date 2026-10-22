package httpapi

import (
	"errors"
	"github.com/jeeth/rss-reader/backend/internal/gravitas"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (a *API) webApp() http.Handler {
	root := filepath.Clean(a.cfg.WebDir)
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if path == "." {
			path = "index.html"
		}
		if path != "login" && path != "index.html" && !strings.Contains(filepath.Base(path), ".") {
			path = "index.html"
		}
		if path == "index.html" && r.URL.Path != "/login" {
			authenticated, authErr := a.hasWebSession(r)
			if errors.Is(authErr, gravitas.ErrUnavailable) {
				w.Header().Set("Cache-Control", "no-store")
				http.Error(w, "Sign-in is temporarily unavailable. Please try again.", http.StatusServiceUnavailable)
				return
			}
			if !authenticated {
				if a.cfg.Federation != nil {
					a.clearSessionCookie(w)
				}
				next := gravitas.SafeNext(r.URL.RequestURI())
				destination := "/login"
				if next != "/" {
					destination += "?next=" + url.QueryEscape(next)
				}
				http.Redirect(w, r, destination, http.StatusSeeOther)
				return
			}
		}
		candidate := filepath.Join(root, path)
		relative, err := filepath.Rel(root, candidate)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(candidate); err != nil || info.IsDir() {
			if strings.Contains(filepath.Base(path), ".") {
				http.NotFound(w, r)
				return
			}
			path = "index.html"
		}
		if path == "index.html" || path == "login" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		clone := r.Clone(r.Context())
		clone.URL.Path = "/" + filepath.ToSlash(path)
		files.ServeHTTP(w, clone)
	})
}

func (a *API) hasWebSession(r *http.Request) (bool, error) {
	cookie, err := r.Cookie(a.sessionCookieName())
	if err != nil || cookie.Value == "" {
		return false, nil
	}
	_, err = a.authenticate(r, cookie.Value)
	return err == nil, err
}
