package catalogidx

import (
	"qLLM/internal/protocol"
)

type Index struct {
	Preset  *protocol.Preset
	Catalog *protocol.Catalog
	byName  map[string]*protocol.Entity // name + aliases
	sources map[string]*protocol.Source
}

// New constructs a value.
func New(preset *protocol.Preset, catalog *protocol.Catalog) (*Index, *protocol.ProtocolError) {
	idx := &Index{
		Preset:  preset,
		Catalog: catalog,
		byName:  make(map[string]*protocol.Entity),
		sources: make(map[string]*protocol.Source),
	}
	for i := range preset.Sources {
		s := &preset.Sources[i]
		if _, exists := idx.sources[s.ID]; exists {
			return nil, protocol.NewError(protocol.ErrConfigError,
				"duplicate source id: "+s.ID, nil)
		}
		idx.sources[s.ID] = s
	}
	for i := range catalog.Entities {
		e := &catalog.Entities[i]
		if err := idx.register(e.Name, e); err != nil {
			return nil, err
		}
		for _, a := range e.Aliases {
			if err := idx.register(a, e); err != nil {
				return nil, err
			}
		}
		if _, ok := idx.sources[e.Source]; !ok {
			return nil, protocol.NewError(protocol.ErrConfigError,
				"entity "+e.Name+" references unknown source "+e.Source, nil)
		}
	}
	return idx, nil
}

// register implements runtime behavior for this package.
func (idx *Index) register(name string, e *protocol.Entity) *protocol.ProtocolError {
	if _, exists := idx.byName[name]; exists {
		return protocol.NewError(protocol.ErrConfigError,
			"duplicate entity name or alias: "+name, nil)
	}
	idx.byName[name] = e
	return nil
}

// ResolveEntity resolves names or paths.
func (idx *Index) ResolveEntity(ref string) (*protocol.Entity, *protocol.ProtocolError) {
	e, ok := idx.byName[ref]
	if !ok {
		return nil, protocol.NewError(protocol.ErrUnknownEntity,
			"unknown entity: "+ref, map[string]any{"entity": ref})
	}
	return e, nil
}

// Source implements runtime behavior for this package.
func (idx *Index) Source(id string) (*protocol.Source, bool) {
	s, ok := idx.sources[id]
	return s, ok
}

// Field implements runtime behavior for this package.
func (idx *Index) Field(e *protocol.Entity, name string) (*protocol.Field, bool) {
	for i := range e.Fields {
		if e.Fields[i].Name == name {
			return &e.Fields[i], true
		}
	}
	return nil, false
}

// DefaultCapabilities implements runtime behavior for this package.
func DefaultCapabilities(t protocol.SourceType) protocol.Capabilities {
	switch protocol.WireFamily(t) {
	case protocol.SourcePostgres, protocol.SourceMySQL, protocol.SourceMSSQL, protocol.SourceSQLite, protocol.SourceClickHouse:
		return protocol.Capabilities{
			Filter: true, Project: true, Agg: true, GroupBy: true,
			JoinSameSource: true, OrderBy: true, Limit: true,
		}
	case protocol.SourceMongoDB:
		return protocol.Capabilities{
			Filter: true, Project: true, Agg: true, GroupBy: true,
			JoinSameSource: false, OrderBy: true, Limit: true,
		}
	case protocol.SourceREST, protocol.SourceDynamoDB, protocol.SourceCassandra, protocol.SourceKSQL:
		return protocol.Capabilities{
			Filter: true, Project: true, Agg: false, GroupBy: false,
			JoinSameSource: false, OrderBy: false, Limit: true,
		}
	default:
		return protocol.Capabilities{}
	}
}

// CatalogResponse builds catalog data.
func (idx *Index) CatalogResponse() protocol.CatalogResponse {
	return idx.CatalogResponseFor(nil)
}

// CatalogResponseFor builds catalog data.
func (idx *Index) CatalogResponseFor(allow map[string]struct{}) protocol.CatalogResponse {
	ents := idx.Catalog.Entities
	if allow != nil {
		filtered := make([]protocol.Entity, 0, len(ents))
		for _, e := range ents {
			if _, ok := allow[e.Name]; ok {
				filtered = append(filtered, e)
			}
		}
		ents = filtered
	}
	srcIDs := map[string]struct{}{}
	for _, e := range ents {
		srcIDs[e.Source] = struct{}{}
	}
	sources := make([]protocol.SourceInfo, 0, len(idx.Preset.Sources))
	for _, s := range idx.Preset.Sources {
		if allow != nil {
			if _, ok := srcIDs[s.ID]; !ok {
				continue
			}
		}
		sources = append(sources, protocol.SourceInfo{
			ID:           s.ID,
			Type:         s.Type,
			Capabilities: DefaultCapabilities(s.Type),
		})
	}
	return protocol.CatalogResponse{
		ProtocolVersion: protocol.ProtocolVersion,
		Project:         idx.Catalog.Project,
		Entities:        ents,
		Sources:         sources,
	}
}

// CatalogFor builds catalog data.
func (idx *Index) CatalogFor(allow map[string]struct{}) *protocol.Catalog {
	if allow == nil {
		return idx.Catalog
	}
	cp := *idx.Catalog
	cp.Entities = nil
	for _, e := range idx.Catalog.Entities {
		if _, ok := allow[e.Name]; ok {
			cp.Entities = append(cp.Entities, e)
		}
	}
	return &cp
}
