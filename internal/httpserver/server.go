package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"qLLM/internal/access"
	"qLLM/internal/appctx"
	"qLLM/internal/catalogidx"
	"qLLM/internal/executor"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
	"qLLM/internal/serveauth"
)

type Server struct {
	Idx          *catalogidx.Index
	Exec         *executor.Executor
	Store        *querystore.Store
	Log          *slog.Logger
	AuthToken    string
	ACL          *access.Registry
	MaxBodyBytes int64
}

func (s *Server) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/howtouseme", s.howToUseMe)
	mux.HandleFunc("GET /v1/catalog", s.catalog)
	mux.HandleFunc("POST /v1/queries", s.createQuery)
	mux.HandleFunc("POST /v1/sql", s.createSQL)
	mux.HandleFunc("GET /v1/queries/{id}", s.getQuery)
	mux.HandleFunc("GET /v1/queries/{id}/result", s.getResult)
	var h http.Handler = s.logMiddleware(mux)
	if s.ACL != nil {
		h = serveauth.AppsMiddleware(s.ACL, h)
	} else {
		h = serveauth.Middleware(s.AuthToken, h)
	}
	return h
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, protocol.HealthResponse{OK: true, ProtocolVersion: protocol.ProtocolVersion})
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Idx.CatalogResponseFor(s.allow(r)))
}

func (s *Server) allow(r *http.Request) map[string]struct{} {
	if s.ACL == nil {
		return nil
	}
	if app := appctx.App(r.Context()); app != nil {
		return app.TableSet()
	}
	return map[string]struct{}{}
}

func (s *Server) createSQL(w http.ResponseWriter, r *http.Request) {
	max := s.MaxBodyBytes
	if max <= 0 {
		max = protocol.DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	var req protocol.SQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := err.Error()
		if err == io.EOF {
			msg = "empty body"
		}
		writeErr(w, http.StatusBadRequest, protocol.NewError(protocol.ErrInvalidSQL, msg, nil))
		return
	}
	resp := s.Exec.ExecuteSQL(r.Context(), &req)
	status := http.StatusOK
	if resp.Status == protocol.StatusFailed && resp.Error != nil {
		status = httpStatus(resp.Error.Code)
	}
	attrs := []any{"queryId", resp.QueryID, "status", string(resp.Status)}
	if resp.Meta != nil {
		attrs = append(attrs, "elapsedMs", resp.Meta.ElapsedMs, "app", resp.Meta.App)
	}
	if resp.Error != nil {
		attrs = append(attrs, "error.code", string(resp.Error.Code))
	}
	s.logger().Info("execute_sql", attrs...)
	writeJSON(w, status, resp)
}

func (s *Server) createQuery(w http.ResponseWriter, r *http.Request) {
	max := s.MaxBodyBytes
	if max <= 0 {
		max = protocol.DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	var q protocol.QueryIR
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		msg := err.Error()
		if err == io.EOF {
			msg = "empty body"
		}
		writeErr(w, http.StatusBadRequest, protocol.NewError(protocol.ErrInvalidIR, msg, nil))
		return
	}
	resp := s.Exec.Execute(r.Context(), &q)
	status := http.StatusOK
	if resp.Status == protocol.StatusAccepted {
		status = http.StatusAccepted
	}
	if resp.Status == protocol.StatusFailed && resp.Error != nil {
		status = httpStatus(resp.Error.Code)
	}
	attrs := []any{"queryId", resp.QueryID, "status", string(resp.Status)}
	if resp.Meta != nil {
		attrs = append(attrs, "elapsedMs", resp.Meta.ElapsedMs)
	}
	if resp.Error != nil {
		attrs = append(attrs, "error.code", string(resp.Error.Code))
	}
	s.logger().Info("execute_query", attrs...)
	writeJSON(w, status, resp)
}

func (s *Server) getQuery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := s.Store.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := s.Store.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	switch resp.Status {
	case protocol.StatusSucceeded:
		writeJSON(w, http.StatusOK, resp)
	case protocol.StatusFailed:
		code := protocol.ErrInternal
		if resp.Error != nil {
			code = resp.Error.Code
		}
		writeJSON(w, httpStatus(code), resp)
	default:
		writeErr(w, http.StatusConflict, protocol.NewError(protocol.ErrNotReady, "query not ready", nil))
	}
}

func httpStatus(code protocol.ErrorCode) int {
	switch code {
	case protocol.ErrInvalidIR, protocol.ErrUnknownEntity, protocol.ErrUnknownField,
		protocol.ErrAmbiguousField, protocol.ErrAmbiguousAlias, protocol.ErrLimitExceeded,
		protocol.ErrUnsupported, protocol.ErrUnsupportedVersion, protocol.ErrInvalidSQL:
		return http.StatusBadRequest
	case protocol.ErrForbidden:
		return http.StatusForbidden
	case protocol.ErrUnauthorized:
		return http.StatusUnauthorized
	case protocol.ErrTimeout:
		return http.StatusGatewayTimeout
	case protocol.ErrSourceError:
		return http.StatusBadGateway
	case protocol.ErrNotReady:
		return http.StatusConflict
	case protocol.ErrNotFound:
		return http.StatusNotFound
	case protocol.ErrConfigError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err *protocol.ProtocolError) {
	writeJSON(w, status, protocol.ErrorResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		Error:           err,
	})
}

func ListenAndServe(addr string, s *Server) error {
	if s.Log == nil {
		s.Log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger().Info("http", "method", r.Method, "path", r.URL.Path, "elapsedMs", time.Since(start).Milliseconds())
	})
}
