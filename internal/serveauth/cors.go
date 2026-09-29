package serveauth

import (
	"net/http"
	"strings"

	"qLLM/internal/protocol"
)

// CORS wraps next with an origin allowlist. Empty origins disables CORS headers.
func CORS(cfg protocol.CORSConfig, next http.Handler) http.Handler {
	origins := make(map[string]struct{}, len(cfg.Origins))
	for _, o := range cfg.Origins {
		origins[o] = struct{}{}
	}
	headers := strings.Join(cfg.AllowHeaders, ", ")
	methods := strings.Join(cfg.AllowMethods, ", ")
	if headers == "" {
		headers = "Content-Type, Accept, Mcp-Session-Id, Authorization"
	}
	if methods == "" {
		methods = "GET, POST, DELETE, OPTIONS"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(origins) == 0 {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if _, ok := origins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Allow-Methods", methods)
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
