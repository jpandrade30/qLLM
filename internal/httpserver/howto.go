package httpserver

import (
	"net/http"

	"qLLM/internal/agentguide"
)

// howToUseMe implements runtime behavior for this package.
func (s *Server) howToUseMe(w http.ResponseWriter, r *http.Request) {
	cat := s.Idx.CatalogFor(s.allow(r))
	writeJSON(w, http.StatusOK, agentguide.Build(s.Idx.Preset, cat))
}
