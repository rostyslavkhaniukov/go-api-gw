package middleware

import (
	"log/slog"
	"net/http"
	"path"
	"strings"

	"api-gw/internal/token"
)

// RouteCheck verifies the request path is in the token's allowed routes.
func RouteCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := token.FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusForbidden, "no token in context")
			return
		}

		// Clean the path to prevent traversal bypasses (e.g. "/api/v1/users/../../admin").
		clean := path.Clean(r.URL.Path)
		if !matchesAny(clean, tok.AllowedRoutes) {
			writeJSON(w, http.StatusForbidden, "route not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func matchesAny(reqPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchRoute(pattern, reqPath) {
			return true
		}
	}
	return false
}

// matchRoute checks if reqPath matches the pattern.
// If the pattern ends with "/*", it uses prefix matching for multi-level paths.
// Otherwise, it falls back to path.Match.
func matchRoute(pattern, reqPath string) bool {
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(reqPath, prefix+"/") || reqPath == prefix
	}
	matched, err := path.Match(pattern, reqPath)
	if err != nil {
		slog.Warn("malformed route pattern", "pattern", pattern, "error", err)
		return false
	}
	return matched
}
