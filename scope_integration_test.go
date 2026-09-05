package fhir

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Phase 3: LoadPackageTgz respects scope
// ---------------------------------------------------------------------------

func TestLoadPackageTgzWithResourceTypesScope(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient")
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	// Patient-type SDs are indexed.
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient should be indexed")
	}
	// A non-Patient SD (e.g. au-organization) is filtered out.
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-organization"); ok {
		t.Error("au-organization should be filtered out")
	}
}

func TestLoadPackageTgzWithScopeNoneCodeSystems(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeNone)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/contact-purpose"); ok {
		t.Error("CodeSystem should be filtered out with ScopeNone")
	}
	// ValueSets are still indexed (ScopeAll default).
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type"); !ok {
		t.Error("ValueSet should still be indexed")
	}
}

func TestLoadPackageTgzWithScopeReferencedSearchParams(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithSearchParams(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	// Patient-based SPs are indexed.
	if _, ok := reg.SearchParameter("Patient", "indigenous-status"); !ok {
		t.Error("indigenous-status (base Patient) should be indexed")
	}
	if _, ok := reg.SearchParameter("Patient", "gender-identity"); !ok {
		t.Error("gender-identity (base includes Patient) should be indexed")
	}
	// Non-Patient SPs are filtered out.
	if _, ok := reg.SearchParameter("Encounter", "discharge-disposition"); ok {
		t.Error("discharge-disposition (base Encounter) should be filtered out")
	}
	if _, ok := reg.SearchParameter("ServiceRequest", "supporting-info"); ok {
		t.Error("supporting-info (base ServiceRequest) should be filtered out")
	}
}

func TestLoadPackageTgzWithScopeReferencedGenericResources(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("ImplementationGuide").WithGenericResources(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	if len(reg.ResourcesForType("ImplementationGuide")) != 1 {
		t.Error("ImplementationGuide resource should be indexed")
	}
	// Patient example resources are filtered out.
	if len(reg.ResourcesForType("Patient")) != 0 {
		t.Error("Patient resources should be filtered out")
	}
}

// ---------------------------------------------------------------------------
// Phase 4: ScopeFromCapabilityStatement end-to-end
// ---------------------------------------------------------------------------

func TestScopeFromCapabilityStatementDerivation(t *testing.T) {
	cs := &CapabilityStatement{
		Rest: []CapabilityStatementRest{
			{
				Mode: "server",
				Resource: []CapabilityStatementRestResource{
					{Type: "Patient", Profile: "http://hl7.org.au/fhir/StructureDefinition/au-patient"},
					{Type: "Observation"},
				},
			},
		},
	}
	s := ScopeFromCapabilityStatement(cs)
	if !s.AllowsResourceType("Patient") || !s.AllowsResourceType("Observation") {
		t.Error("ResourceTypes not derived from CS")
	}
	if s.AllowsResourceType("Encounter") {
		t.Error("Encounter should not be in scope")
	}
	if !s.AllowsProfile("http://hl7.org.au/fhir/StructureDefinition/au-patient") {
		t.Error("Profiles not derived from CS")
	}
	if s.SearchParams != ScopeReferenced {
		t.Errorf("SearchParams = %q, want ScopeReferenced", s.SearchParams)
	}
	if s.GenericResources != ScopeReferenced {
		t.Errorf("GenericResources = %q, want ScopeReferenced", s.GenericResources)
	}
}

func TestLoadPackageTgzWithScopeFromCapabilityStatement(t *testing.T) {
	// A CS declaring support for Patient only.
	cs := &CapabilityStatement{
		Rest: []CapabilityStatementRest{
			{
				Mode: "server",
				Resource: []CapabilityStatementRestResource{
					{Type: "Patient", Profile: "http://hl7.org.au/fhir/StructureDefinition/au-patient"},
				},
			},
		},
	}
	reg := NewRegistry()
	reg.Scope = ScopeFromCapabilityStatement(cs)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	// Patient SD and profile indexed.
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient should be indexed")
	}
	// Non-Patient SD filtered out.
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-organization"); ok {
		t.Error("au-organization should be filtered out")
	}
	// Patient-based SearchParameters indexed (ScopeReferenced default).
	if _, ok := reg.SearchParameter("Patient", "indigenous-status"); !ok {
		t.Error("indigenous-status should be indexed")
	}
	// Non-Patient SearchParameters filtered out.
	if _, ok := reg.SearchParameter("Encounter", "discharge-disposition"); ok {
		t.Error("discharge-disposition should be filtered out")
	}
	// Patient generic resources indexed (ScopeReferenced default).
	if len(reg.ResourcesForType("Patient")) == 0 {
		t.Error("Patient resources should be indexed")
	}
}

// ---------------------------------------------------------------------------
// Phase 5: ScopeReferenced ValueSets/CodeSystems via Resolve
// ---------------------------------------------------------------------------

func TestLoadPackageTgzWithScopeReferencedValueSets(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Location").WithValueSets(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	// Nothing indexed until Resolve is called.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/location-physical-type-extended"); ok {
		t.Error("ValueSet should not be indexed before Resolve")
	}
	reg.Resolve()
	// ValueSets referenced by the in-scope Location SD are indexed.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/location-physical-type-extended"); !ok {
		t.Error("location-physical-type-extended should be indexed after Resolve")
	}
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/v3-ServiceDeliveryLocationRoleType-extended"); !ok {
		t.Error("v3-ServiceDeliveryLocationRoleType-extended should be indexed after Resolve")
	}
	// A ValueSet not referenced by any in-scope SD is filtered out.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type"); ok {
		t.Error("accession-number-type should be filtered out")
	}
}

func TestLoadPackageTgzWithScopeReferencedCodeSystems(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Location").WithValueSets(ScopeReferenced).WithCodeSystems(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	reg.Resolve()
	// CodeSystems referenced by in-scope ValueSets are indexed.
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/location-physical-type"); !ok {
		t.Error("location-physical-type CodeSystem should be indexed")
	}
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/location-type"); !ok {
		t.Error("location-type CodeSystem should be indexed")
	}
	// A CodeSystem referenced only by an out-of-scope ValueSet is filtered out.
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/contact-purpose"); ok {
		t.Error("contact-purpose CodeSystem should be filtered out")
	}
}

func TestLoadPackageTgzWithScopeFromCS_Resolved(t *testing.T) {
	// A CS declaring support for Location only.
	cs := &CapabilityStatement{
		Rest: []CapabilityStatementRest{
			{
				Mode: "server",
				Resource: []CapabilityStatementRestResource{
					{Type: "Location"},
				},
			},
		},
	}
	reg := NewRegistry()
	reg.Scope = ScopeFromCapabilityStatement(cs)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	reg.Resolve()
	// ValueSets referenced by the in-scope Location SD are indexed.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/location-physical-type-extended"); !ok {
		t.Error("location-physical-type-extended should be indexed")
	}
	// Unreferenced ValueSets are filtered out.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type"); ok {
		t.Error("accession-number-type should be filtered out")
	}
}
