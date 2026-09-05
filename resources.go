package fhir

import "sort"

// ValueSet — a terminology resource defining a set of codes.
type ValueSet struct {
	ResourceType string             `json:"resourceType"`
	URL          string             `json:"url"`
	Version      string             `json:"version,omitempty"`
	Name         string             `json:"name,omitempty"`
	Status       string             `json:"status"`
	Compose      *ValueSetCompose   `json:"compose,omitempty"`
	Expansion    *ValueSetExpansion `json:"expansion,omitempty"`
}

// ValueSetCompose is the "compose" block of a ValueSet, listing the code
// systems and concepts that define its membership.
type ValueSetCompose struct {
	Include []ValueSetInclude `json:"include"`
	Exclude []ValueSetInclude `json:"exclude,omitempty"`
}

// ValueSetInclude references a code system (and optionally specific concepts,
// filters, or other value sets) contributing codes to a ValueSet.
type ValueSetInclude struct {
	System   string             `json:"system,omitempty"`
	Version  string             `json:"version,omitempty"`
	Concept  []ConceptReference `json:"concept,omitempty"`
	Filter   []ValueSetFilter   `json:"filter,omitempty"`
	ValueSet []string           `json:"valueSet,omitempty"`
}

// ValueSetFilter constrains which codes from a system are included.
type ValueSetFilter struct {
	Property string `json:"property"`
	Op       string `json:"op"`
	Value    string `json:"value"`
}

// ConceptReference is a single code reference within a ValueSet include.
type ConceptReference struct {
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}

// ValueSetExpansion is the "expansion" block of a ValueSet, holding the
// computed set of codes.
type ValueSetExpansion struct {
	Contains []ValueSetExpansionContains `json:"contains,omitempty"`
}

// ValueSetExpansionContains is a single code in a ValueSet expansion, which
// may itself contain nested codes.
type ValueSetExpansionContains struct {
	System   string                      `json:"system,omitempty"`
	Code     string                      `json:"code,omitempty"`
	Display  string                      `json:"display,omitempty"`
	Contains []ValueSetExpansionContains `json:"contains,omitempty"`
}

// CodeSystem — a terminology resource defining a set of codes with hierarchy.
type CodeSystem struct {
	ResourceType string              `json:"resourceType"`
	URL          string              `json:"url"`
	Version      string              `json:"version,omitempty"`
	Name         string              `json:"name,omitempty"`
	Status       string              `json:"status"`
	Concepts     []CodeSystemConcept `json:"concept,omitempty"`
}

// CodeSystemConcept is a single code in a CodeSystem, which may have child
// concepts forming a hierarchy.
type CodeSystemConcept struct {
	Code     string              `json:"code"`
	Display  string              `json:"display,omitempty"`
	Concepts []CodeSystemConcept `json:"concept,omitempty"`
}

// CapabilityStatement — describes what a FHIR server supports.
type CapabilityStatement struct {
	ResourceType string                    `json:"resourceType"`
	URL          string                    `json:"url,omitempty"`
	Version      string                    `json:"version,omitempty"`
	Name         string                    `json:"name,omitempty"`
	Status       string                    `json:"status"`
	FhirVersion  string                    `json:"fhirVersion"`
	Rest         []CapabilityStatementRest `json:"rest,omitempty"`
}

// CapabilityStatementRest describes one RESTful interaction mode (server or
// client) supported by the server.
type CapabilityStatementRest struct {
	Mode     string                            `json:"mode"`
	Resource []CapabilityStatementRestResource `json:"resource,omitempty"`
}

// CapabilityStatementRestResource describes the support for one resource type.
type CapabilityStatementRestResource struct {
	Type             string                           `json:"type"`
	Profile          string                           `json:"profile,omitempty"`
	SupportedProfile []string                         `json:"supportedProfile,omitempty"`
	Interaction      []CapabilityStatementInteraction `json:"interaction,omitempty"`
	Operation        []CapabilityStatementOperation   `json:"operation,omitempty"`
	SearchParam      []CapabilityStatementSearchParam `json:"searchParam,omitempty"`
}

// CapabilityStatementInteraction is a supported REST interaction (e.g. read).
type CapabilityStatementInteraction struct {
	Code string `json:"code"`
}

// CapabilityStatementOperation is a supported operation on a resource type.
type CapabilityStatementOperation struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
}

// CapabilityStatementSearchParam is a supported search parameter.
type CapabilityStatementSearchParam struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
	Type       string `json:"type"`
}

// SearchParameter — describes a search parameter for a FHIR resource type.
type SearchParameter struct {
	ResourceType string   `json:"resourceType"`
	URL          string   `json:"url"`
	Name         string   `json:"name"`
	Code         string   `json:"code"`
	Base         []string `json:"base"`
	Type         string   `json:"type"`
	Expression   string   `json:"expression,omitempty"`
}

// Resource — a generic FHIR instance resource, kept opaque. ResourceType and
// ProfileURLs (from meta.profile) are extracted for indexing; Raw holds the
// full decoded JSON.
type Resource struct {
	ResourceType string
	ProfileURLs  []string
	Raw          map[string]any
}

// ---------------------------------------------------------------------------
// Registry methods
// ---------------------------------------------------------------------------

// AddValueSet indexes a ValueSet by its canonical URL.
func (r *Registry) AddValueSet(vs *ValueSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Scope != nil && r.Scope.ValueSets == ScopeReferenced {
		r.pendingValueSets[vs.URL] = vs
		return
	}
	if r.Scope != nil && !r.Scope.AllowsValueSet() {
		return
	}
	r.valueSets[vs.URL] = vs
}

// ValueSet returns the ValueSet for a canonical URL.
func (r *Registry) ValueSet(url string) (*ValueSet, bool) {
	r.mu.RLock()
	vs, ok := r.valueSets[url]
	r.mu.RUnlock()
	return vs, ok
}

// AddCodeSystem indexes a CodeSystem by its canonical URL.
func (r *Registry) AddCodeSystem(cs *CodeSystem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Scope != nil && r.Scope.CodeSystems == ScopeReferenced {
		r.pendingCodeSystems[cs.URL] = cs
		return
	}
	if r.Scope != nil && !r.Scope.AllowsCodeSystem() {
		return
	}
	r.codeSystems[cs.URL] = cs
}

// CodeSystem returns the CodeSystem for a canonical URL.
func (r *Registry) CodeSystem(url string) (*CodeSystem, bool) {
	r.mu.RLock()
	cs, ok := r.codeSystems[url]
	r.mu.RUnlock()
	return cs, ok
}

// AddCapabilityStatement appends a CapabilityStatement to the registry.
func (r *Registry) AddCapabilityStatement(cs *CapabilityStatement) {
	r.mu.Lock()
	r.capabilityStatements = append(r.capabilityStatements, cs)
	r.mu.Unlock()
}

// CapabilityStatements returns all registered CapabilityStatements.
func (r *Registry) CapabilityStatements() []*CapabilityStatement {
	r.mu.RLock()
	out := make([]*CapabilityStatement, len(r.capabilityStatements))
	copy(out, r.capabilityStatements)
	r.mu.RUnlock()
	return out
}

// AddSearchParameter indexes a SearchParameter by each of its base resource
// types and its code.
func (r *Registry) AddSearchParameter(sp *SearchParameter) {
	r.mu.Lock()
	r.searchParams = append(r.searchParams, sp)
	for _, base := range sp.Base {
		r.searchParamIndex[base+":"+sp.Code] = sp
	}
	r.mu.Unlock()
}

// SearchParameter returns the SearchParameter for a resource type and code.
func (r *Registry) SearchParameter(resourceType, code string) (*SearchParameter, bool) {
	r.mu.RLock()
	sp, ok := r.searchParamIndex[resourceType+":"+code]
	r.mu.RUnlock()
	return sp, ok
}

// SearchParameters returns all registered SearchParameters.
func (r *Registry) SearchParameters() []*SearchParameter {
	r.mu.RLock()
	out := make([]*SearchParameter, len(r.searchParams))
	copy(out, r.searchParams)
	r.mu.RUnlock()
	return out
}

// AddResource indexes a generic Resource by its resource type.
func (r *Registry) AddResource(res *Resource) {
	r.mu.Lock()
	r.resources[res.ResourceType] = append(r.resources[res.ResourceType], res)
	r.mu.Unlock()
}

// ResourcesForType returns all generic Resources of a given resource type.
func (r *Registry) ResourcesForType(resourceType string) []*Resource {
	r.mu.RLock()
	out := make([]*Resource, len(r.resources[resourceType]))
	copy(out, r.resources[resourceType])
	r.mu.RUnlock()
	return out
}

// AllResources returns every generic Resource across all resource types,
// ordered deterministically by resource type.
func (r *Registry) AllResources() []*Resource {
	r.mu.RLock()
	var out []*Resource
	for _, resources := range r.resources {
		out = append(out, resources...)
	}
	r.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ResourceType < out[j].ResourceType
	})
	return out
}
