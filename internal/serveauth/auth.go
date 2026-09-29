package serveauth

import (
	"encoding/json"
	"net/http"
	"strings"

	"qLLM/internal/access"
	"qLLM/internal/appctx"
	"qLLM/internal/cryptox"
	"qLLM/internal/protocol"
)

// Middleware requires Authorization: Bearer <token> when token is non-empty.
// GET /v1/health and OPTIONS are exempt (probes / CORS preflight).
func Middleware(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/v1/health" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		got := bearerToken(r.Header.Get("Authorization"))
		if !secureEqual(got, token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(protocol.ErrorResponse{
				ProtocolVersion: protocol.ProtocolVersion,
				Error: protocol.NewError(protocol.ErrUnauthorized,
					"missing or invalid Authorization bearer token", nil),
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AppsMiddleware(reg *access.Registry, next http.Handler) http.Handler {
	if reg == nil || len(reg.Apps) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/v1/health" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		got := bearerToken(r.Header.Get("Authorization"))
		app := reg.LookupBearer(got)
		if app == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(protocol.ErrorResponse{
				ProtocolVersion: protocol.ProtocolVersion,
				Error: protocol.NewError(protocol.ErrUnauthorized,
					"missing or invalid Authorization bearer token", nil),
			})
			return
		}
		next.ServeHTTP(w, r.WithContext(appctx.WithApp(r.Context(), app)))
	})
}

func bearerToken(h string) string {
	const p = "Bearer "
	if len(h) < len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}

func secureEqual(got, want string) bool {
	return cryptox.HMACEqual(got, want)
}

// MaxBytes limits request body size (0 = no limit).
func MaxBytes(max int64, next http.Handler) http.Handler {
	if max <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, max)
		}
		next.ServeHTTP(w, r)
	})
}
