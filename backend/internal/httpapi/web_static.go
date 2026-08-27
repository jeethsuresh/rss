package httpapi

import (
	"net/http"
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
		if path == "index.html" && r.URL.Path != "/login" && !a.hasWebSession(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
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

func (a *API) hasWebSession(r *http.Request) bool {
	cookie, err := r.Cookie(webSessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	_, err = a.store.Authenticate(r.Context(), cookie.Value)
	return err == nil
}
