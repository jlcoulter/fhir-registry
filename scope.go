package fhir

import "strings"

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
	s.ValueSets = ScopeReferenced
	s.CodeSystems = ScopeReferenced
	return s
}

// canonicalURL strips a FHIR version fragment (e.g. "|4.0.1") from a canonical
// URL so it can be matched against stored resource URLs.
func canonicalURL(url string) string {
	if i := strings.Index(url, "|"); i >= 0 {
		return url[:i]
	}
	return url
}

// collectCodeSystemURLs adds every CodeSystem system URL referenced by a
// ValueSet's compose include/exclude blocks to the given set.
func collectCodeSystemURLs(vs *ValueSet, cs map[string]bool) {
	if vs == nil || vs.Compose == nil {
		return
	}
	for _, inc := range vs.Compose.Include {
		if inc.System != "" {
			cs[canonicalURL(inc.System)] = true
		}
	}
	for _, exc := range vs.Compose.Exclude {
		if exc.System != "" {
			cs[canonicalURL(exc.System)] = true
		}
	}
}

// Resolve finalises a scoped Registry after all resources have been loaded.
// When ValueSets or CodeSystems policy is ScopeReferenced, those resources are
// buffered during loading; Resolve indexes the ones referenced by in-scope
// StructureDefinitions (via element bindings) and transitively by other
// referenced ValueSets (via compose includes). It is idempotent and a no-op
// when no category uses ScopeReferenced.
func (r *Registry) Resolve() {
	if r.Scope == nil {
		return
	}
	if r.Scope.ValueSets != ScopeReferenced && r.Scope.CodeSystems != ScopeReferenced {
		return
	}

	// 1. Collect seed ValueSet URLs from in-scope StructureDefinition bindings.
	referencedVS := make(map[string]bool)
	r.mu.RLock()
	for _, sd := range r.byURL {
		if sd.Snapshot != nil {
			for _, elem := range sd.Snapshot.Elements {
				if elem.Binding != nil && elem.Binding.ValueSet != "" {
					referencedVS[canonicalURL(elem.Binding.ValueSet)] = true
				}
			}
		}
		if sd.Differential != nil {
			for _, elem := range sd.Differential.Elements {
				if elem.Binding != nil && elem.Binding.ValueSet != "" {
					referencedVS[canonicalURL(elem.Binding.ValueSet)] = true
				}
			}
		}
	}
	r.mu.RUnlock()

	// 2. Fixpoint: expand referenced ValueSets through compose includes.
	for changed := true; changed; {
		changed = false
		r.mu.RLock()
		for url, vs := range r.pendingValueSets {
			if !referencedVS[url] || vs == nil || vs.Compose == nil {
				continue
			}
			for _, inc := range vs.Compose.Include {
				for _, vsURL := range inc.ValueSet {
					cu := canonicalURL(vsURL)
					if !referencedVS[cu] {
						referencedVS[cu] = true
						changed = true
					}
				}
			}
			for _, exc := range vs.Compose.Exclude {
				for _, vsURL := range exc.ValueSet {
					cu := canonicalURL(vsURL)
					if !referencedVS[cu] {
						referencedVS[cu] = true
						changed = true
					}
				}
			}
		}
		r.mu.RUnlock()
	}

	// 3. Collect referenced CodeSystem URLs.
	referencedCS := make(map[string]bool)
	if r.Scope.CodeSystems == ScopeReferenced {
		r.mu.RLock()
		if r.Scope.ValueSets == ScopeReferenced {
			for url, vs := range r.pendingValueSets {
				if referencedVS[url] {
					collectCodeSystemURLs(vs, referencedCS)
				}
			}
		} else {
			// ValueSets are ScopeAll: all indexed ValueSets are in scope.
			for _, vs := range r.valueSets {
				collectCodeSystemURLs(vs, referencedCS)
			}
		}
		r.mu.RUnlock()
	}

	// 4. Index matching pending resources and clear the buffers.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Scope.ValueSets == ScopeReferenced {
		for url, vs := range r.pendingValueSets {
			if referencedVS[url] {
				r.valueSets[url] = vs
			}
		}
		r.pendingValueSets = make(map[string]*ValueSet)
	}
	if r.Scope.CodeSystems == ScopeReferenced {
		for url, cs := range r.pendingCodeSystems {
			if referencedCS[url] {
				r.codeSystems[url] = cs
			}
		}
		r.pendingCodeSystems = make(map[string]*CodeSystem)
	}
}
