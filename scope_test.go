package fhir

import (
	"testing"
)

// ---------------------------------------------------------------------------
// NewScope defaults
// ---------------------------------------------------------------------------

func TestNewScopeDefaults(t *testing.T) {
	s := NewScope()
	if s == nil {
		t.Fatal("NewScope returned nil")
	}
	if s.ResourceTypes != nil {
		t.Errorf("ResourceTypes = %v, want nil", s.ResourceTypes)
	}
	if s.Profiles != nil {
		t.Errorf("Profiles = %v, want nil", s.Profiles)
	}
	if s.ValueSets != ScopeAll {
		t.Errorf("ValueSets = %q, want ScopeAll", s.ValueSets)
	}
	if s.CodeSystems != ScopeAll {
		t.Errorf("CodeSystems = %q, want ScopeAll", s.CodeSystems)
	}
	if s.SearchParams != ScopeAll {
		t.Errorf("SearchParams = %q, want ScopeAll", s.SearchParams)
	}
	if s.CapabilityStatements != ScopeAll {
		t.Errorf("CapabilityStatements = %q, want ScopeAll", s.CapabilityStatements)
	}
	if s.GenericResources != ScopeAll {
		t.Errorf("GenericResources = %q, want ScopeAll", s.GenericResources)
	}
}

// ---------------------------------------------------------------------------
// AllowsResourceType
// ---------------------------------------------------------------------------

func TestScopeAllowsResourceType_Allowed(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient", "Observation")
	if !s.AllowsResourceType("Patient") {
		t.Error("AllowsResourceType(Patient) = false, want true")
	}
	if !s.AllowsResourceType("Observation") {
		t.Error("AllowsResourceType(Observation) = false, want true")
	}
}

func TestScopeAllowsResourceType_NotAllowed(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient")
	if s.AllowsResourceType("Observation") {
		t.Error("AllowsResourceType(Observation) = true, want false")
	}
}

func TestScopeAllowsResourceType_NilScope(t *testing.T) {
	var s *Scope
	if !s.AllowsResourceType("Anything") {
		t.Error("nil scope AllowsResourceType = false, want true")
	}
}

func TestScopeAllowsResourceType_EmptyMap(t *testing.T) {
	s := &Scope{ResourceTypes: map[string]bool{}}
	if !s.AllowsResourceType("Patient") {
		t.Error("empty ResourceTypes map should allow all types")
	}
}

// ---------------------------------------------------------------------------
// AllowsProfile
// ---------------------------------------------------------------------------

func TestScopeAllowsProfile_Allowed(t *testing.T) {
	s := NewScope().WithProfiles("http://example.org/StructureDefinition/au-patient")
	if !s.AllowsProfile("http://example.org/StructureDefinition/au-patient") {
		t.Error("AllowsProfile = false, want true")
	}
}

func TestScopeAllowsProfile_NotAllowed(t *testing.T) {
	s := NewScope().WithProfiles("http://example.org/StructureDefinition/au-patient")
	if s.AllowsProfile("http://example.org/StructureDefinition/au-observation") {
		t.Error("AllowsProfile = true, want false")
	}
}

func TestScopeAllowsProfile_NilScope(t *testing.T) {
	var s *Scope
	if !s.AllowsProfile("http://example.org/anything") {
		t.Error("nil scope AllowsProfile = false, want true")
	}
}

func TestScopeAllowsProfile_EmptyMap(t *testing.T) {
	s := &Scope{Profiles: map[string]bool{}}
	if !s.AllowsProfile("http://example.org/anything") {
		t.Error("empty Profiles map should allow all profiles")
	}
}

// ---------------------------------------------------------------------------
// AllowsValueSet / AllowsCodeSystem
// ---------------------------------------------------------------------------

func TestScopeAllowsValueSet_All(t *testing.T) {
	var nilScope *Scope
	if !nilScope.AllowsValueSet() {
		t.Error("nil scope AllowsValueSet = false, want true")
	}
	if !NewScope().AllowsValueSet() {
		t.Error("ScopeAll AllowsValueSet = false, want true")
	}
}

func TestScopeAllowsValueSet_None(t *testing.T) {
	s := NewScope().WithValueSets(ScopeNone)
	if s.AllowsValueSet() {
		t.Error("ScopeNone AllowsValueSet = true, want false")
	}
}

func TestScopeAllowsCodeSystem_All(t *testing.T) {
	var nilScope *Scope
	if !nilScope.AllowsCodeSystem() {
		t.Error("nil scope AllowsCodeSystem = false, want true")
	}
	if !NewScope().AllowsCodeSystem() {
		t.Error("ScopeAll AllowsCodeSystem = false, want true")
	}
}

func TestScopeAllowsCodeSystem_None(t *testing.T) {
	s := NewScope().WithCodeSystems(ScopeNone)
	if s.AllowsCodeSystem() {
		t.Error("ScopeNone AllowsCodeSystem = true, want false")
	}
}

// ---------------------------------------------------------------------------
// AllowsSearchParam
// ---------------------------------------------------------------------------

func TestScopeAllowsSearchParam_All(t *testing.T) {
	var nilScope *Scope
	sp := &SearchParameter{Base: []string{"Patient"}}
	if !nilScope.AllowsSearchParam(sp) {
		t.Error("nil scope AllowsSearchParam = false, want true")
	}
	if !NewScope().AllowsSearchParam(sp) {
		t.Error("ScopeAll AllowsSearchParam = false, want true")
	}
}

func TestScopeAllowsSearchParam_None(t *testing.T) {
	s := NewScope().WithSearchParams(ScopeNone)
	if s.AllowsSearchParam(&SearchParameter{Base: []string{"Patient"}}) {
		t.Error("ScopeNone AllowsSearchParam = true, want false")
	}
}

func TestScopeAllowsSearchParam_Referenced_Yes(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient").WithSearchParams(ScopeReferenced)
	if !s.AllowsSearchParam(&SearchParameter{Base: []string{"Patient"}}) {
		t.Error("ScopeReferenced with matching base = false, want true")
	}
}

func TestScopeAllowsSearchParam_Referenced_No(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient").WithSearchParams(ScopeReferenced)
	if s.AllowsSearchParam(&SearchParameter{Base: []string{"Observation"}}) {
		t.Error("ScopeReferenced with non-matching base = true, want false")
	}
}

func TestScopeAllowsSearchParam_Referenced_NilResourceTypes(t *testing.T) {
	s := NewScope().WithSearchParams(ScopeReferenced)
	if !s.AllowsSearchParam(&SearchParameter{Base: []string{"Observation"}}) {
		t.Error("ScopeReferenced with nil ResourceTypes = false, want true (all allowed)")
	}
}

// ---------------------------------------------------------------------------
// AllowsCapabilityStatement
// ---------------------------------------------------------------------------

func TestScopeAllowsCapabilityStatement_All(t *testing.T) {
	var nilScope *Scope
	if !nilScope.AllowsCapabilityStatement() {
		t.Error("nil scope AllowsCapabilityStatement = false, want true")
	}
	if !NewScope().AllowsCapabilityStatement() {
		t.Error("ScopeAll AllowsCapabilityStatement = false, want true")
	}
}

func TestScopeAllowsCapabilityStatement_None(t *testing.T) {
	s := NewScope().WithCapabilityStatements(ScopeNone)
	if s.AllowsCapabilityStatement() {
		t.Error("ScopeNone AllowsCapabilityStatement = true, want false")
	}
}

// ---------------------------------------------------------------------------
// AllowsGenericResource
// ---------------------------------------------------------------------------

func TestScopeAllowsGenericResource_All(t *testing.T) {
	var nilScope *Scope
	if !nilScope.AllowsGenericResource("Patient") {
		t.Error("nil scope AllowsGenericResource = false, want true")
	}
	if !NewScope().AllowsGenericResource("Patient") {
		t.Error("ScopeAll AllowsGenericResource = false, want true")
	}
}

func TestScopeAllowsGenericResource_None(t *testing.T) {
	s := NewScope().WithGenericResources(ScopeNone)
	if s.AllowsGenericResource("Patient") {
		t.Error("ScopeNone AllowsGenericResource = true, want false")
	}
}

func TestScopeAllowsGenericResource_Referenced_Yes(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient").WithGenericResources(ScopeReferenced)
	if !s.AllowsGenericResource("Patient") {
		t.Error("ScopeReferenced with matching type = false, want true")
	}
}

func TestScopeAllowsGenericResource_Referenced_No(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient").WithGenericResources(ScopeReferenced)
	if s.AllowsGenericResource("Observation") {
		t.Error("ScopeReferenced with non-matching type = true, want false")
	}
}

// ---------------------------------------------------------------------------
// AllowsStructureDefinition
// ---------------------------------------------------------------------------

func TestScopeAllowsStructureDefinition_All(t *testing.T) {
	var nilScope *Scope
	sd := &StructureDefinition{Type: "Patient", URL: "http://example.org/StructureDefinition/Patient", Derivation: ""}
	if !nilScope.AllowsStructureDefinition(sd) {
		t.Error("nil scope AllowsStructureDefinition = false, want true")
	}
	if !NewScope().AllowsStructureDefinition(sd) {
		t.Error("ScopeAll AllowsStructureDefinition = false, want true")
	}
}

func TestScopeAllowsStructureDefinition_TypeAllowed(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient")
	sd := &StructureDefinition{Type: "Patient", URL: "http://example.org/StructureDefinition/Patient", Derivation: ""}
	if !s.AllowsStructureDefinition(sd) {
		t.Error("type in ResourceTypes should be allowed")
	}
}

func TestScopeAllowsStructureDefinition_ProfileAllowed(t *testing.T) {
	s := NewScope().WithProfiles("http://example.org/StructureDefinition/au-patient")
	sd := &StructureDefinition{Type: "Patient", URL: "http://example.org/StructureDefinition/au-patient", Derivation: "constraint"}
	if !s.AllowsStructureDefinition(sd) {
		t.Error("URL in Profiles should be allowed")
	}
}

func TestScopeAllowsStructureDefinition_BaseDefAllowed(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient")
	// Base definition (derivation="") for an in-scope type is always allowed,
	// even if its URL is not in Profiles.
	sd := &StructureDefinition{Type: "Patient", URL: "http://hl7.org/fhir/StructureDefinition/Patient", Derivation: ""}
	if !s.AllowsStructureDefinition(sd) {
		t.Error("base definition for in-scope type should be allowed")
	}
}

func TestScopeAllowsStructureDefinition_NotAllowed(t *testing.T) {
	s := NewScope().WithResourceTypes("Patient")
	sd := &StructureDefinition{Type: "Observation", URL: "http://example.org/StructureDefinition/au-observation", Derivation: "constraint"}
	if s.AllowsStructureDefinition(sd) {
		t.Error("type not in ResourceTypes and URL not in Profiles should be disallowed")
	}
}

func TestScopeAllowsStructureDefinition_NilResourceTypes(t *testing.T) {
	s := NewScope().WithProfiles("http://example.org/StructureDefinition/au-patient")
	// With Profiles set but ResourceTypes nil, the URL must still be in Profiles.
	sd := &StructureDefinition{Type: "Patient", URL: "http://example.org/StructureDefinition/au-patient", Derivation: "constraint"}
	if !s.AllowsStructureDefinition(sd) {
		t.Error("URL in Profiles should be allowed even with nil ResourceTypes")
	}
	other := &StructureDefinition{Type: "Observation", URL: "http://example.org/StructureDefinition/au-observation", Derivation: "constraint"}
	if s.AllowsStructureDefinition(other) {
		t.Error("URL not in Profiles should be disallowed")
	}
}

// ---------------------------------------------------------------------------
// ScopeFromCapabilityStatement
// ---------------------------------------------------------------------------

func TestScopeFromCapabilityStatement(t *testing.T) {
	cs := &CapabilityStatement{
		Rest: []CapabilityStatementRest{
			{
				Mode: "server",
				Resource: []CapabilityStatementRestResource{
					{Type: "Patient", Profile: "http://example.org/StructureDefinition/au-patient"},
					{Type: "Observation"},
				},
			},
		},
	}
	s := ScopeFromCapabilityStatement(cs)
	if s == nil {
		t.Fatal("ScopeFromCapabilityStatement returned nil")
	}
	if !s.AllowsResourceType("Patient") || !s.AllowsResourceType("Observation") {
		t.Error("ResourceTypes not populated from CS")
	}
	if s.AllowsResourceType("Encounter") {
		t.Error("Encounter should not be in ResourceTypes")
	}
	if !s.AllowsProfile("http://example.org/StructureDefinition/au-patient") {
		t.Error("Profiles not populated from CS profile")
	}
	if s.SearchParams != ScopeReferenced {
		t.Errorf("SearchParams = %q, want ScopeReferenced", s.SearchParams)
	}
	if s.GenericResources != ScopeReferenced {
		t.Errorf("GenericResources = %q, want ScopeReferenced", s.GenericResources)
	}
	if s.ValueSets != ScopeReferenced {
		t.Errorf("ValueSets = %q, want ScopeReferenced", s.ValueSets)
	}
	if s.CodeSystems != ScopeReferenced {
		t.Errorf("CodeSystems = %q, want ScopeReferenced", s.CodeSystems)
	}
}

func TestScopeFromCapabilityStatement_SupportedProfiles(t *testing.T) {
	cs := &CapabilityStatement{
		Rest: []CapabilityStatementRest{
			{
				Mode: "server",
				Resource: []CapabilityStatementRestResource{
					{Type: "Patient", SupportedProfile: []string{"http://example.org/StructureDefinition/au-patient", "http://example.org/StructureDefinition/au-ihi"}},
				},
			},
		},
	}
	s := ScopeFromCapabilityStatement(cs)
	if !s.AllowsProfile("http://example.org/StructureDefinition/au-patient") {
		t.Error("supportedProfile not added to Profiles")
	}
	if !s.AllowsProfile("http://example.org/StructureDefinition/au-ihi") {
		t.Error("second supportedProfile not added to Profiles")
	}
}

func TestScopeFromCapabilityStatement_Nil(t *testing.T) {
	if s := ScopeFromCapabilityStatement(nil); s != nil {
		t.Errorf("ScopeFromCapabilityStatement(nil) = %v, want nil", s)
	}
}
