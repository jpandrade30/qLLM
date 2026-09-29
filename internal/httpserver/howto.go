package httpserver

import (
	"net/http"

	"qLLM/internal/agentguide"
)

func (s *Server) howToUseMe(w http.ResponseWriter, r *http.Request) {
	cat := s.Idx.CatalogFor(s.allow(r))
	writeJSON(w, http.StatusOK, agentguide.Build(s.Idx.Preset, cat))
}
