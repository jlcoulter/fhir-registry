package fhir

// ScopePolicy controls how a resource category is indexed by a scoped
// Registry.
type ScopePolicy string

const (
	// ScopeAll (the default) indexes every resource in the category.
	ScopeAll ScopePolicy = ""
	// ScopeReferenced indexes only resources that are in scope (e.g. whose
	// base resource type is in the scope's ResourceTypes).
	ScopeReferenced ScopePolicy = "referenced"
	// ScopeNone skips the entire category.
	ScopeNone ScopePolicy = "none"
)

// Scope narrows which resources a Registry indexes. It is set on a Registry
// before any Load* call; modifying it after loading has undefined behaviour.
// A nil Scope (or a zero-value Scope) indexes everything.
type Scope struct {
	// ResourceTypes is an allowlist of base resource types to index. When
	// empty, all types are indexed.
	ResourceTypes map[string]bool
	// Profiles is an allowlist of canonical StructureDefinition URLs to
	// index. When empty, all profiles are indexed. Base definitions for
	// types in ResourceTypes are always included.
	Profiles map[string]bool
	// ValueSets controls ValueSet indexing.
	ValueSets ScopePolicy
	// CodeSystems controls CodeSystem indexing.
	CodeSystems ScopePolicy
	// SearchParams controls SearchParameter indexing.
	SearchParams ScopePolicy
	// CapabilityStatements controls CapabilityStatement indexing.
	CapabilityStatements ScopePolicy
	// GenericResources controls generic Resource indexing.
	GenericResources ScopePolicy
}

// NewScope returns a Scope with all policies set to ScopeAll and no
// allowlists, i.e. it indexes everything.
func NewScope() *Scope {
	return &Scope{}
}

// WithResourceTypes sets the ResourceTypes allowlist.
func (s *Scope) WithResourceTypes(types ...string) *Scope {
	s.ResourceTypes = make(map[string]bool, len(types))
	for _, t := range types {
		s.ResourceTypes[t] = true
	}
	return s
}

// WithProfiles sets the Profiles allowlist.
func (s *Scope) WithProfiles(urls ...string) *Scope {
	s.Profiles = make(map[string]bool, len(urls))
	for _, u := range urls {
		s.Profiles[u] = true
	}
	return s
}

// WithValueSets sets the ValueSets policy.
func (s *Scope) WithValueSets(p ScopePolicy) *Scope { s.ValueSets = p; return s }

// WithCodeSystems sets the CodeSystems policy.
func (s *Scope) WithCodeSystems(p ScopePolicy) *Scope { s.CodeSystems = p; return s }

// WithSearchParams sets the SearchParams policy.
func (s *Scope) WithSearchParams(p ScopePolicy) *Scope { s.SearchParams = p; return s }

// WithCapabilityStatements sets the CapabilityStatements policy.
func (s *Scope) WithCapabilityStatements(p ScopePolicy) *Scope { s.CapabilityStatements = p; return s }

// WithGenericResources sets the GenericResources policy.
func (s *Scope) WithGenericResources(p ScopePolicy) *Scope { s.GenericResources = p; return s }

// AllowsResourceType reports whether a base resource type is in scope. A nil
// scope or an empty allowlist allows everything.
func (s *Scope) AllowsResourceType(typeName string) bool {
	if s == nil || len(s.ResourceTypes) == 0 {
		return true
	}
	return s.ResourceTypes[typeName]
}

// AllowsProfile reports whether a canonical StructureDefinition URL is in
// scope. A nil scope or an empty allowlist allows everything.
func (s *Scope) AllowsProfile(url string) bool {
	if s == nil || len(s.Profiles) == 0 {
		return true
	}
	return s.Profiles[url]
}

// AllowsValueSet reports whether ValueSets are indexed.
func (s *Scope) AllowsValueSet() bool {
	return s == nil || s.ValueSets != ScopeNone
}

// AllowsCodeSystem reports whether CodeSystems are indexed.
func (s *Scope) AllowsCodeSystem() bool {
	return s == nil || s.CodeSystems != ScopeNone
}

// AllowsSearchParam reports whether a SearchParameter is indexed. Under
// ScopeReferenced, at least one of its base types must be in ResourceTypes.
func (s *Scope) AllowsSearchParam(sp *SearchParameter) bool {
	if s == nil || s.SearchParams == ScopeAll {
		return true
	}
	if s.SearchParams == ScopeNone {
		return false
	}
	// ScopeReferenced: any base type in ResourceTypes.
	if len(s.ResourceTypes) == 0 {
		return true
	}
	for _, base := range sp.Base {
		if s.ResourceTypes[base] {
			return true
		}
	}
	return false
}

// AllowsCapabilityStatement reports whether CapabilityStatements are indexed.
func (s *Scope) AllowsCapabilityStatement() bool {
	return s == nil || s.CapabilityStatements != ScopeNone
}

// AllowsGenericResource reports whether a generic Resource of the given type
// is indexed. Under ScopeReferenced, the resource type must be in
// ResourceTypes.
func (s *Scope) AllowsGenericResource(resourceType string) bool {
	if s == nil || s.GenericResources == ScopeAll {
		return true
	}
	if s.GenericResources == ScopeNone {
		return false
	}
	// ScopeReferenced: resource type in ResourceTypes.
	return s.AllowsResourceType(resourceType)
}

// AllowsStructureDefinition reports whether a StructureDefinition is indexed.
// A definition is allowed if its URL is in Profiles, its type is in
// ResourceTypes, or it is a base definition (derivation="") for an in-scope
// type. With no allowlists set, everything is allowed.
func (s *Scope) AllowsStructureDefinition(sd *StructureDefinition) bool {
	if s == nil {
		return true
	}
	// Base definitions for in-scope types are always included, even when the
	// URL is not explicitly listed in Profiles.
	if sd.Derivation == "" && len(s.ResourceTypes) > 0 && s.ResourceTypes[sd.Type] {
		return true
	}
	if len(s.Profiles) > 0 && s.Profiles[sd.URL] {
		return true
	}
	if len(s.ResourceTypes) > 0 && s.ResourceTypes[sd.Type] {
		return true
	}
	// No allowlists set: everything allowed.
	return len(s.Profiles) == 0 && len(s.ResourceTypes) == 0
}

// ScopeFromCapabilityStatement derives a Scope from a CapabilityStatement.
// ResourceTypes and Profiles are populated from the statement's rest blocks;
// SearchParams and GenericResources default to ScopeReferenced.
func ScopeFromCapabilityStatement(cs *CapabilityStatement) *Scope {
	if cs == nil {
		return nil
	}
	s := NewScope()
	for _, rest := range cs.Rest {
		for _, res := range rest.Resource {
			if res.Type != "" {
				if s.ResourceTypes == nil {
					s.ResourceTypes = make(map[string]bool)
				}
				s.ResourceTypes[res.Type] = true
			}
			if res.Profile != "" {
				if s.Profiles == nil {
					s.Profiles = make(map[string]bool)
				}
				s.Profiles[res.Profile] = true
			}
			for _, sp := range res.SupportedProfile {
				if s.Profiles == nil {
					s.Profiles = make(map[string]bool)
				}
				s.Profiles[sp] = true
			}
		}
	}
	s.SearchParams = ScopeReferenced
	s.GenericResources = ScopeReferenced
	return s
}
