package httpserver

import (
	"net/http"

	"qLLM/internal/agentguide"
)

func (s *Server) howToUseMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, agentguide.Build(s.Idx.Preset, s.Idx.Catalog))
}
