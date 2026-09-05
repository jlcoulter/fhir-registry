// Package fhir loads, indexes, and reasons about FHIR structure definitions.
// It is the single source of truth for FHIR structure knowledge, designed to
// handle arbitrary implementation guides.
//
// The central type is Registry, which indexes every StructureDefinition in a
// set of FHIR packages by canonical URL and base type, and exposes the element
// tree for each. Packages can be loaded from a directory or a .tgz archive,
// and full dependency chains can be resolved from a registry server via
// PackageClient.
//
// Beyond structure definitions, a Registry indexes terminology and conformance
// resources (ValueSets, CodeSystems, CapabilityStatements, SearchParameters)
// and any other resource type as an opaque Resource. A Scope narrows which
// resources are indexed, e.g. to only what a package's CapabilityStatement
// declares as supported.
//
// Typical usage:
//
//	reg := fhir.NewRegistry()
//	if err := reg.LoadPackage("package"); err != nil {
//	    log.Fatal(err)
//	}
//	tree, err := reg.Tree("http://hl7.org/fhir/StructureDefinition/Patient")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	out, report, err := reg.Marshal("Patient", instance)
package fhir

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Max represents the upper bound of an element's cardinality. A value of
// MaxUnbounded corresponds to the FHIR "*" (no upper limit).
type Max int

// MaxUnbounded represents unbounded cardinality (*).
// Cardinality [0..1] → Min=0, Max=1; [0..*] → Min=0, Max=MaxUnbounded.
const MaxUnbounded Max = -1

// IsUnbounded reports whether the max is "*".
func (m Max) IsUnbounded() bool { return m == MaxUnbounded }

// String returns the FHIR "max" representation: "*" or the numeric value.
func (m Max) String() string {
	if m == MaxUnbounded {
		return "*"
	}
	return strconv.Itoa(int(m))
}

// ElementDefinition captures the structural definition of a FHIR element.
// Children holds direct sub-elements, forming a tree rooted at the resource type.
type ElementDefinition struct {
	ID                 string               `json:"id"`
	Path               string               `json:"path"`
	SliceName          string               `json:"sliceName,omitempty"`
	Short              string               `json:"short,omitempty"`
	Definition         string               `json:"definition,omitempty"`
	Comment            string               `json:"comment,omitempty"`
	Min                int                  `json:"min"`
	Max                Max                  `json:"max"` // MaxUnbounded represents "*"
	MustSupport        bool                 `json:"mustSupport,omitempty"`
	BaseMax            *Max                 `json:"baseMax,omitempty"` // nil means "not specified / inherit"
	Types              []ElementType        `json:"types,omitempty"`
	Profile            []string             `json:"profile,omitempty"`
	TargetProfile      []string             `json:"targetProfile,omitempty"`
	IsModifier         bool                 `json:"isModifier"`
	IsSummary          bool                 `json:"isSummary"`
	Binding            *Binding             `json:"binding,omitempty"`
	Fixed              any                  `json:"fixed,omitempty"`
	Pattern            any                  `json:"pattern,omitempty"`
	Examples           []any                `json:"examples,omitempty"`
	Constraints        []ElementConstraint  `json:"constraint,omitempty"`
	Condition          []string             `json:"condition,omitempty"`
	MeaningWhenMissing string               `json:"meaningWhenMissing,omitempty"`
	ContentReference   string               `json:"contentReference,omitempty"`
	Slicing            *Slicing             `json:"slicing,omitempty"`
	Children           []*ElementDefinition `json:"children,omitempty"`
	Slices             []*SliceGroup        `json:"slices,omitempty"`
}

// SliceGroup groups the sliced children of a repeating element under a single
// slice name. Definition is the slice entry element (the child carrying the
// SliceName); its own Children hold the slice's sub-elements.
type SliceGroup struct {
	Name       string
	Definition *ElementDefinition
}

// ElementConstraint is a FHIR invariant (constraint) attached to an element.
type ElementConstraint struct {
	Key        string
	Severity   string
	Human      string
	Expression string
	Source     string
}

// ElementType represents a single type choice for a FHIR element.
type ElementType struct {
	Code          string   `json:"code"`
	Profiles      []string `json:"profiles,omitempty"`
	TargetProfile []string `json:"targetProfiles,omitempty"`
}

// Binding describes the value set binding for a coded element.
type Binding struct {
	Strength    string `json:"strength"`
	Description string `json:"description,omitempty"`
	ValueSet    string `json:"valueSet,omitempty"`
}

// Slicing describes how a repeating element is sliced.
type Slicing struct {
	Discriminator []Discriminator `json:"discriminator,omitempty"`
	Rules         string          `json:"rules,omitempty"`
	Ordered       bool            `json:"ordered,omitempty"`
}

// Discriminator identifies how slices are told apart.
type Discriminator struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// ---------------------------------------------------------------------------
// Raw JSON types — the snapshot/differential in the JSON file is a flat list
// of elements with dot-separated paths. These structs match that shape.
// ---------------------------------------------------------------------------

type StructureDefinition struct {
	ResourceType   string        `json:"resourceType"`
	ID             string        `json:"id"`
	URL            string        `json:"url"`
	Name           string        `json:"name"`
	Title          string        `json:"title,omitempty"`
	Status         string        `json:"status"`
	Kind           string        `json:"kind"`
	Abstract       bool          `json:"abstract"`
	Type           string        `json:"type"`
	BaseDefinition string        `json:"baseDefinition,omitempty"`
	Derivation     string        `json:"derivation,omitempty"`
	Snapshot       *Snapshot     `json:"snapshot,omitempty"`
	Differential   *Differential `json:"differential,omitempty"`
}

type Snapshot struct {
	Elements []RawElement `json:"element"`
}

type Differential struct {
	Elements []RawElement `json:"element"`
}

// RawElement is the flat JSON shape from the StructureDefinition file.
// The "max" field comes as a string ("1", "*", "0") so it needs custom parsing.
// Fixed, Pattern, and Examples are populated by UnmarshalJSON from the
// polymorphic fixed*, pattern*, and example properties.
type RawElement struct {
	ID                 string          `json:"id"`
	Path               string          `json:"path"`
	SliceName          string          `json:"sliceName,omitempty"`
	Short              string          `json:"short,omitempty"`
	Definition         string          `json:"definition,omitempty"`
	Comment            string          `json:"comment,omitempty"`
	Min                *int            `json:"min"`
	Max                json.RawMessage `json:"max"`
	MustSupport        *bool           `json:"mustSupport,omitempty"`
	Base               *RawBase        `json:"base,omitempty"`
	Types              []RawType       `json:"type,omitempty"`
	Profile            []string        `json:"profile,omitempty"`
	TargetProfile      []string        `json:"targetProfile,omitempty"`
	IsModifier         *bool           `json:"isModifier"`
	IsSummary          *bool           `json:"isSummary"`
	Binding            *RawBinding     `json:"binding,omitempty"`
	Constraint         []RawConstraint `json:"constraint,omitempty"`
	Condition          []string        `json:"condition,omitempty"`
	MeaningWhenMissing string          `json:"meaningWhenMissing,omitempty"`
	ContentReference   string          `json:"contentReference,omitempty"`
	Slicing            *Slicing        `json:"slicing,omitempty"`
	Fixed              any             `json:"-"`
	Pattern            any             `json:"-"`
	Examples           []any           `json:"-"`
}

// RawBase is the "base" sub-object of an element, recording the cardinality
// inherited from the base definition.
type RawBase struct {
	Min *int            `json:"min"`
	Max json.RawMessage `json:"max"`
}

// RawConstraint is the JSON shape of a FHIR invariant.
type RawConstraint struct {
	Key        string `json:"key"`
	Severity   string `json:"severity"`
	Human      string `json:"human"`
	Expression string `json:"expression"`
	Source     string `json:"source"`
}

type RawType struct {
	Code          string   `json:"code"`
	Profiles      []string `json:"profile,omitempty"`
	TargetProfile []string `json:"targetProfile,omitempty"`
}

type RawBinding struct {
	Strength    string `json:"strength"`
	Description string `json:"description,omitempty"`
	ValueSet    string `json:"valueSet,omitempty"`
}

// UnmarshalJSON decodes a raw element, capturing the polymorphic fixed*,
// pattern*, and example properties that have no fixed struct field. FHIR
// allows at most one fixed* and one pattern* property per element, so the
// first match wins.
func (r *RawElement) UnmarshalJSON(data []byte) error {
	type Alias RawElement
	aux := (*Alias)(r)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	for key, val := range m {
		if strings.HasPrefix(key, "fixed") && len(key) > len("fixed") {
			var v any
			if err := json.Unmarshal(val, &v); err == nil {
				r.Fixed = v
			}
			break
		}
	}
	for key, val := range m {
		if strings.HasPrefix(key, "pattern") && len(key) > len("pattern") {
			var v any
			if err := json.Unmarshal(val, &v); err == nil {
				r.Pattern = v
			}
			break
		}
	}
	if raw, ok := m["example"]; ok {
		var examples []map[string]any
		if err := json.Unmarshal(raw, &examples); err == nil {
			for _, e := range examples {
				for k, v := range e {
					if strings.HasPrefix(k, "value") {
						r.Examples = append(r.Examples, v)
						break
					}
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

// parseMax converts a FHIR "max" value to a Max.
// "*" → MaxUnbounded, numeric strings like "1" → 1, "0" → 0.
func parseMax(raw json.RawMessage) (Max, error) {
	if len(raw) == 0 {
		return 1, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "*" {
			return MaxUnbounded, nil
		}
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			return 0, fmt.Errorf("cannot parse max value %q: %w", s, err)
		}
		return Max(n), nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return Max(n), nil
	}
	return 0, fmt.Errorf("cannot parse max value: %s", string(raw))
}

// ptrBool dereferences a *bool, defaulting to false when nil.
func ptrBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// convertRawElement converts a RawElement into a typed ElementDefinition.
func convertRawElement(raw RawElement) (ElementDefinition, error) {
	maxVal, err := parseMax(raw.Max)
	if err != nil {
		return ElementDefinition{}, fmt.Errorf("element %q: %w", raw.Path, err)
	}

	types := make([]ElementType, 0, len(raw.Types))
	for _, rt := range raw.Types {
		types = append(types, ElementType{
			Code:          rt.Code,
			Profiles:      rt.Profiles,
			TargetProfile: rt.TargetProfile,
		})
	}

	var binding *Binding
	if raw.Binding != nil {
		binding = &Binding{
			Strength:    raw.Binding.Strength,
			Description: raw.Binding.Description,
			ValueSet:    raw.Binding.ValueSet,
		}
	}

	var min int
	if raw.Min != nil {
		min = *raw.Min
	}

	var baseMax *Max
	if raw.Base != nil {
		if bm, err := parseMax(raw.Base.Max); err == nil {
			baseMax = &bm
		}
	}

	constraints := make([]ElementConstraint, 0, len(raw.Constraint))
	for _, rc := range raw.Constraint {
		constraints = append(constraints, ElementConstraint{
			Key:        rc.Key,
			Severity:   rc.Severity,
			Human:      rc.Human,
			Expression: rc.Expression,
			Source:     rc.Source,
		})
	}

	return ElementDefinition{
		ID:                 raw.ID,
		Path:               raw.Path,
		SliceName:          raw.SliceName,
		Short:              raw.Short,
		Definition:         raw.Definition,
		Comment:            raw.Comment,
		Min:                min,
		Max:                maxVal,
		MustSupport:        ptrBool(raw.MustSupport),
		BaseMax:            baseMax,
		Types:              types,
		Profile:            raw.Profile,
		TargetProfile:      raw.TargetProfile,
		IsModifier:         ptrBool(raw.IsModifier),
		IsSummary:          ptrBool(raw.IsSummary),
		Binding:            binding,
		Fixed:              raw.Fixed,
		Pattern:            raw.Pattern,
		Examples:           raw.Examples,
		Constraints:        constraints,
		Condition:          raw.Condition,
		MeaningWhenMissing: raw.MeaningWhenMissing,
		ContentReference:   raw.ContentReference,
		Slicing:            raw.Slicing,
	}, nil
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry indexes every StructureDefinition in a set of FHIR packages by
// canonical URL and by base type, and exposes the element tree for each.
type Registry struct {
	mu sync.RWMutex
	// byURL maps a canonical StructureDefinition URL to its definition.
	byURL map[string]*StructureDefinition
	// byType maps a base type name (e.g. "Patient", "Identifier") to the
	// definitions that profile it. The first entry is the base definition
	// when the package ships it; otherwise it is a profile.
	byType map[string][]*StructureDefinition
	// trees caches the built element tree per canonical URL.
	trees map[string]*ElementTree
	// valueSets indexes ValueSets by canonical URL.
	valueSets map[string]*ValueSet
	// codeSystems indexes CodeSystems by canonical URL.
	codeSystems map[string]*CodeSystem
	// capabilityStatements holds all registered CapabilityStatements.
	capabilityStatements []*CapabilityStatement
	// searchParams holds all registered SearchParameters.
	searchParams []*SearchParameter
	// searchParamIndex maps "resourceType:code" to a SearchParameter.
	searchParamIndex map[string]*SearchParameter
	// resources indexes generic Resources by resource type.
	resources map[string][]*Resource
	// pendingValueSets buffers ValueSets when ValueSets policy is
	// ScopeReferenced, until Resolve is called.
	pendingValueSets map[string]*ValueSet
	// pendingCodeSystems buffers CodeSystems when CodeSystems policy is
	// ScopeReferenced, until Resolve is called.
	pendingCodeSystems map[string]*CodeSystem
	// Scope narrows which resources are indexed. A nil Scope indexes
	// everything. It must be set before any Load* call.
	Scope *Scope
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byURL:              make(map[string]*StructureDefinition),
		byType:             make(map[string][]*StructureDefinition),
		trees:              make(map[string]*ElementTree),
		valueSets:          make(map[string]*ValueSet),
		codeSystems:        make(map[string]*CodeSystem),
		searchParamIndex:   make(map[string]*SearchParameter),
		resources:          make(map[string][]*Resource),
		pendingValueSets:   make(map[string]*ValueSet),
		pendingCodeSystems: make(map[string]*CodeSystem),
	}
}

// LoadPackage loads every JSON resource in a directory (a FHIR package folder)
// into the registry. Dependencies are NOT resolved; use LoadPackageWithDeps
// for that. Non-JSON files and malformed resources are silently skipped.
func (r *Registry) LoadPackage(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%w: reading package dir %s: %v", ErrPackageNotFound, dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("%w: reading %s: %v", ErrPackageNotFound, e.Name(), err)
		}
		if err := r.addResource(e.Name(), data); err != nil {
			return err
		}
	}
	return nil
}

// LoadPackageTgz loads a FHIR package distributed as a .tgz archive into the
// registry.
func (r *Registry) LoadPackageTgz(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: opening %s: %v", ErrPackageNotFound, path, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: gzip %s: %v", ErrPackageNotFound, path, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: tar %s: %v", ErrPackageNotFound, path, err)
		}
		if hdr.Typeflag != tar.TypeReg || !strings.HasSuffix(hdr.Name, ".json") {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("%w: reading %s: %v", ErrPackageNotFound, hdr.Name, err)
		}
		if err := r.addResource(hdr.Name, data); err != nil {
			return err
		}
	}
	return nil
}

// ElementTree is the recursively-linked element tree for one StructureDefinition.
type ElementTree struct {
	SD     *StructureDefinition
	Root   *ElementDefinition
	ByPath map[string][]*ElementDefinition
	ByID   map[string]*ElementDefinition
}

// addResource dispatches a raw JSON resource into the registry.
func (r *Registry) addResource(name string, data []byte) error {
	var head struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(data, &head); err != nil || head.ResourceType == "" {
		return nil // package.json, .index.json, etc.
	}
	switch head.ResourceType {
	case "StructureDefinition":
		var sd StructureDefinition
		if err := json.Unmarshal(data, &sd); err != nil {
			return fmt.Errorf("%w: parsing %s: %v", ErrParseFailure, name, err)
		}
		if sd.URL == "" {
			return nil
		}
		if r.Scope != nil && !r.Scope.AllowsStructureDefinition(&sd) {
			return nil
		}
		r.mu.Lock()
		r.byURL[sd.URL] = &sd
		if sd.Type != "" {
			r.byType[sd.Type] = append(r.byType[sd.Type], &sd)
		}
		r.mu.Unlock()
	case "ValueSet":
		var vs ValueSet
		if err := json.Unmarshal(data, &vs); err != nil {
			return fmt.Errorf("%w: parsing %s: %v", ErrParseFailure, name, err)
		}
		if vs.URL == "" {
			return nil
		}
		if r.Scope != nil && r.Scope.ValueSets == ScopeReferenced {
			r.mu.Lock()
			r.pendingValueSets[vs.URL] = &vs
			r.mu.Unlock()
			return nil
		}
		if r.Scope != nil && !r.Scope.AllowsValueSet() {
			return nil
		}
		r.mu.Lock()
		r.valueSets[vs.URL] = &vs
		r.mu.Unlock()
	case "CodeSystem":
		var cs CodeSystem
		if err := json.Unmarshal(data, &cs); err != nil {
			return fmt.Errorf("%w: parsing %s: %v", ErrParseFailure, name, err)
		}
		if cs.URL == "" {
			return nil
		}
		if r.Scope != nil && r.Scope.CodeSystems == ScopeReferenced {
			r.mu.Lock()
			r.pendingCodeSystems[cs.URL] = &cs
			r.mu.Unlock()
			return nil
		}
		if r.Scope != nil && !r.Scope.AllowsCodeSystem() {
			return nil
		}
		r.mu.Lock()
		r.codeSystems[cs.URL] = &cs
		r.mu.Unlock()
	case "CapabilityStatement":
		var cs CapabilityStatement
		if err := json.Unmarshal(data, &cs); err != nil {
			return fmt.Errorf("%w: parsing %s: %v", ErrParseFailure, name, err)
		}
		if r.Scope != nil && !r.Scope.AllowsCapabilityStatement() {
			return nil
		}
		r.mu.Lock()
		r.capabilityStatements = append(r.capabilityStatements, &cs)
		r.mu.Unlock()
	case "SearchParameter":
		var sp SearchParameter
		if err := json.Unmarshal(data, &sp); err != nil {
			return fmt.Errorf("%w: parsing %s: %v", ErrParseFailure, name, err)
		}
		if r.Scope != nil && !r.Scope.AllowsSearchParam(&sp) {
			return nil
		}
		r.mu.Lock()
		r.searchParams = append(r.searchParams, &sp)
		for _, base := range sp.Base {
			r.searchParamIndex[base+":"+sp.Code] = &sp
		}
		r.mu.Unlock()
	default:
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil // not a JSON object resource
		}
		resourceType, _ := raw["resourceType"].(string)
		if resourceType == "" {
			return nil
		}
		if r.Scope != nil && !r.Scope.AllowsGenericResource(resourceType) {
			return nil
		}
		res := &Resource{
			ResourceType: resourceType,
			ProfileURLs:  profileURLsOf(raw),
			Raw:          raw,
		}
		r.mu.Lock()
		r.resources[resourceType] = append(r.resources[resourceType], res)
		r.mu.Unlock()
	}
	return nil
}

// profileURLsOf extracts the meta.profile URLs from a decoded resource map.
func profileURLsOf(raw map[string]any) []string {
	meta, ok := raw["meta"].(map[string]any)
	if !ok {
		return nil
	}
	profiles, ok := meta["profile"].([]any)
	if !ok {
		return nil
	}
	var urls []string
	for _, p := range profiles {
		if s, ok := p.(string); ok {
			urls = append(urls, s)
		}
	}
	return urls
}

// Definition returns the StructureDefinition for a canonical URL. The bool
// reports whether the definition was found.
func (r *Registry) Definition(url string) (*StructureDefinition, bool) {
	r.mu.RLock()
	sd, ok := r.byURL[url]
	r.mu.RUnlock()
	return sd, ok
}

// DefinitionsForType returns all definitions that profile the given base type.
// The first entry is the base definition when the package ships it; otherwise
// it is a profile.
func (r *Registry) DefinitionsForType(typeName string) []*StructureDefinition {
	r.mu.RLock()
	defs := r.byType[typeName]
	r.mu.RUnlock()
	return defs
}

// Tree builds (and caches) the element tree for a canonical URL. For a
// profile that has only a differential, the base definition is resolved from
// the registry and the differential is merged onto its snapshot.
func (r *Registry) Tree(url string) (*ElementTree, error) {
	r.mu.RLock()
	if t, ok := r.trees[url]; ok {
		r.mu.RUnlock()
		return t, nil
	}
	r.mu.RUnlock()

	// Build under the write lock so byURL reads and the cache store are
	// atomic with respect to addResource, which also takes the write lock.
	r.mu.Lock()
	defer r.mu.Unlock()

	// Another goroutine may have built it while we waited for the lock.
	if t, ok := r.trees[url]; ok {
		return t, nil
	}

	sd, ok := r.byURL[url]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDefinitionNotFound, url)
	}

	// Ensure the definition has a complete element list, merging the
	// differential onto the base snapshot when necessary.
	raws, err := r.ensureSnapshot(sd)
	if err != nil {
		return nil, err
	}

	t, err := BuildTreeElements(sd, raws)
	if err != nil {
		return nil, err
	}
	r.trees[url] = t
	return t, nil
}

// ensureSnapshot returns a complete element list for a definition, merging the
// differential onto the base snapshot when the definition has no snapshot.
func (r *Registry) ensureSnapshot(sd *StructureDefinition) ([]RawElement, error) {
	if sd.Snapshot != nil && len(sd.Snapshot.Elements) > 0 {
		return sd.Snapshot.Elements, nil
	}
	if sd.Differential == nil || len(sd.Differential.Elements) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrIncompleteDefinition, sd.ID)
	}
	baseURL := stripVersion(sd.BaseDefinition)
	base, ok := r.byURL[baseURL]
	if !ok {
		return nil, fmt.Errorf("%w: %q for %s", ErrBaseDefinition, baseURL, sd.ID)
	}
	baseRaw, err := r.ensureSnapshot(base)
	if err != nil {
		return nil, err
	}
	return MergeDifferential(baseRaw, sd.Differential.Elements), nil
}

// stripVersion removes a "|version" suffix from a canonical URL.
func stripVersion(url string) string {
	if i := strings.LastIndex(url, "|"); i >= 0 {
		return url[:i]
	}
	return url
}

// TreeForType returns the element tree for a base type name, preferring the
// base definition when present and otherwise the first profile.
func (r *Registry) TreeForType(typeName string) (*ElementTree, error) {
	r.mu.RLock()
	defs := r.byType[typeName]
	r.mu.RUnlock()
	if len(defs) == 0 {
		return nil, fmt.Errorf("%w: type %s", ErrDefinitionNotFound, typeName)
	}
	// Prefer the base definition (derivation empty) over profiles.
	sorted := make([]*StructureDefinition, len(defs))
	copy(sorted, defs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Derivation == "" && sorted[j].Derivation != ""
	})
	return r.Tree(sorted[0].URL)
}

// ResolveType returns the element tree for a type reference. Profiles are
// tried first (in order), then the type's base definition URL, then a unique
// profile for the type if one exists.
func (r *Registry) ResolveType(typeCode string, profiles []string) (*ElementTree, bool) {
	for _, p := range profiles {
		if t, err := r.Tree(stripVersion(p)); err == nil {
			return t, true
		}
	}
	base := "http://hl7.org/fhir/StructureDefinition/" + typeCode
	if t, err := r.Tree(base); err == nil {
		return t, true
	}
	defs := r.DefinitionsForType(typeCode)
	if len(defs) == 1 {
		if t, err := r.Tree(defs[0].URL); err == nil {
			return t, true
		}
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Tree building
// ---------------------------------------------------------------------------

// BuildTree links a flat snapshot/differential element list into a tree.
// Elements are keyed by their unique id (which carries slice names), so
// sliced elements sharing a path do not collide.
//
// A differential-only definition that derives from a base cannot be turned
// into a complete tree here: resolving it requires the base snapshot, which
// only a Registry can provide. Callers in that situation should use
// Registry.Tree instead.
func BuildTree(sd *StructureDefinition) (*ElementTree, error) {
	var raws []RawElement
	if sd.Snapshot != nil && len(sd.Snapshot.Elements) > 0 {
		raws = sd.Snapshot.Elements
	} else if sd.Differential != nil {
		if sd.BaseDefinition != "" {
			return nil, fmt.Errorf("%w: %s has only a differential and a base definition; use Registry.Tree to resolve the base snapshot", ErrIncompleteDefinition, sd.ID)
		}
		raws = sd.Differential.Elements
	}
	if len(raws) == 0 {
		return nil, fmt.Errorf("%w: %s has no snapshot or differential elements", ErrIncompleteDefinition, sd.ID)
	}
	return BuildTreeElements(sd, raws)
}

// BuildTreeElements links a complete element list into a tree.
func BuildTreeElements(sd *StructureDefinition, raws []RawElement) (*ElementTree, error) {
	all := make([]ElementDefinition, 0, len(raws))
	byID := make(map[string]*ElementDefinition, len(raws))
	for _, raw := range raws {
		elem, err := convertRawElement(raw)
		if err != nil {
			return nil, err
		}
		all = append(all, elem)
		byID[elem.ID] = &all[len(all)-1]
	}

	// Link children by id prefix: a child's id is its parent's id plus a
	// "." segment or a ":" slice suffix.
	for i := range all {
		elem := &all[i]
		parentID := parentIDOf(elem.ID)
		if parentID == "" {
			continue // root
		}
		if parent, ok := byID[parentID]; ok {
			parent.Children = append(parent.Children, elem)
		}
	}

	// Group sliced children into Slices and remove them from Children, so
	// Children holds only non-sliced direct children. A slice entry (a child
	// with a SliceName) becomes a SliceGroup on its parent; the slice's own
	// sub-elements remain reachable via SliceGroup.Definition.Children.
	for i := range all {
		elem := &all[i]
		if len(elem.Children) == 0 {
			continue
		}
		kept := elem.Children[:0]
		var slices []*SliceGroup
		for _, child := range elem.Children {
			if child.SliceName == "" {
				kept = append(kept, child)
				continue
			}
			slices = append(slices, &SliceGroup{Name: child.SliceName, Definition: child})
		}
		elem.Children = kept
		elem.Slices = slices
	}

	root := byID[sd.Type]
	if root == nil {
		// Fall back to the first element (the root of the snapshot).
		root = &all[0]
	}

	// Build the path map (a path may map to several sliced elements).
	byPath := make(map[string][]*ElementDefinition)
	for i := range all {
		elem := &all[i]
		byPath[elem.Path] = append(byPath[elem.Path], elem)
	}

	return &ElementTree{SD: sd, Root: root, ByPath: byPath, ByID: byID}, nil
}

// parentIDOf strips the last "." segment or ":" slice suffix from an id,
// whichever separator appears last. For "Foo.ext:birthPlace.url" the parent is
// "Foo.ext:birthPlace" (the "." before "url" is the last separator), not
// "Foo.ext".
func parentIDOf(id string) string {
	lastColon := strings.LastIndex(id, ":")
	lastDot := strings.LastIndex(id, ".")
	if lastColon > lastDot {
		return id[:lastColon]
	}
	if lastDot > lastColon {
		return id[:lastDot]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Cardinality helpers
// ---------------------------------------------------------------------------

// IsMulti reports whether the element has unbounded upper cardinality
// (max = "*").
func IsMulti(elem *ElementDefinition) bool { return elem.Max.IsUnbounded() }

// IsRequired reports whether the element has a non-zero minimum cardinality.
func IsRequired(elem *ElementDefinition) bool { return elem.Min > 0 }

// Cardinality returns the FHIR cardinality string for the element, e.g. "0..*".
func Cardinality(elem *ElementDefinition) string {
	return fmt.Sprintf("%d..%s", elem.Min, elem.Max)
}

// PrimaryTypeCode returns the code of the element's first type choice, or the
// empty string if the element has no types.
func PrimaryTypeCode(elem *ElementDefinition) string {
	if len(elem.Types) == 0 {
		return ""
	}
	return elem.Types[0].Code
}

// IsChoice reports whether the element is a choice-of-type ([x]) element.
func IsChoice(elem *ElementDefinition) bool {
	return strings.HasSuffix(elem.Path, "[x]")
}

// ChoiceName returns the concrete JSON key for a choice element given a type
// code, e.g. ("deceased[x]", "boolean") → "deceasedBoolean".
func ChoiceName(elem *ElementDefinition, typeCode string) string {
	base := strings.TrimSuffix(elem.Path, "[x]")
	base = base[strings.LastIndex(base, ".")+1:]
	return base + capitalize(typeCode)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
