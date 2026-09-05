package fhir

import (
	"testing"
)

// ---------------------------------------------------------------------------
// addResource scope filtering
// ---------------------------------------------------------------------------

func TestAddResourceWithResourceTypesScope_SD(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient")
	if err := reg.addResource("p.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/Patient","type":"Patient","derivation":""}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("o.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/Observation","type":"Observation","derivation":""}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/Patient"); !ok {
		t.Error("Patient SD not indexed")
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/Observation"); ok {
		t.Error("Observation SD should be filtered out")
	}
}

func TestAddResourceWithResourceTypesScope_ProfileAllowed(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithProfiles("http://example.org/StructureDefinition/au-observation")
	if err := reg.addResource("o.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/au-observation","type":"Observation","derivation":"constraint"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/au-observation"); !ok {
		t.Error("profile explicitly in Profiles should be indexed even if type not in ResourceTypes")
	}
}

func TestAddResourceWithResourceTypesScope_BaseDefAlwaysIncluded(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient")
	// Base Patient definition (derivation="") must be included even though its
	// URL is not in Profiles.
	if err := reg.addResource("p.json", []byte(`{"resourceType":"StructureDefinition","url":"http://hl7.org/fhir/StructureDefinition/Patient","type":"Patient","derivation":""}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.Definition("http://hl7.org/fhir/StructureDefinition/Patient"); !ok {
		t.Error("base definition for in-scope type should be indexed")
	}
}

func TestAddResourceWithProfilesScope_OnlyProfiles(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithProfiles("http://example.org/StructureDefinition/au-patient")
	if err := reg.addResource("p.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/au-patient","type":"Patient","derivation":"constraint"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("o.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/au-observation","type":"Observation","derivation":"constraint"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient should be indexed")
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/au-observation"); ok {
		t.Error("au-observation should be filtered out")
	}
}

func TestAddResourceWithScopeNoneValueSets(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithValueSets(ScopeNone)
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs","status":"active"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if vs, ok := reg.ValueSet("http://example.org/ValueSet/vs"); ok || vs != nil {
		t.Error("ValueSet should be filtered out with ScopeNone")
	}
}

func TestAddResourceWithScopeNoneCodeSystems(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeNone)
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs","status":"draft"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if cs, ok := reg.CodeSystem("http://example.org/CodeSystem/cs"); ok || cs != nil {
		t.Error("CodeSystem should be filtered out with ScopeNone")
	}
}

func TestAddResourceWithScopeReferencedSearchParams(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithSearchParams(ScopeReferenced)
	if err := reg.addResource("sp1.json", []byte(`{"resourceType":"SearchParameter","url":"http://example.org/SearchParameter/name","code":"name","base":["Patient"],"type":"string"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("sp2.json", []byte(`{"resourceType":"SearchParameter","url":"http://example.org/SearchParameter/status","code":"status","base":["Observation"],"type":"token"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.SearchParameter("Patient", "name"); !ok {
		t.Error("SearchParameter with base Patient should be indexed")
	}
	if _, ok := reg.SearchParameter("Observation", "status"); ok {
		t.Error("SearchParameter with base Observation should be filtered out")
	}
}

func TestAddResourceWithScopeNoneGenericResources(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithGenericResources(ScopeNone)
	if err := reg.addResource("p.json", []byte(`{"resourceType":"Patient","id":"p1"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if len(reg.ResourcesForType("Patient")) != 0 {
		t.Error("generic Resource should be filtered out with ScopeNone")
	}
}

func TestAddResourceWithScopeReferencedGenericResources(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithGenericResources(ScopeReferenced)
	if err := reg.addResource("p.json", []byte(`{"resourceType":"Patient","id":"p1"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("o.json", []byte(`{"resourceType":"Observation","id":"o1"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if len(reg.ResourcesForType("Patient")) != 1 {
		t.Error("Patient resource should be indexed")
	}
	if len(reg.ResourcesForType("Observation")) != 0 {
		t.Error("Observation resource should be filtered out")
	}
}

func TestAddResourceNilScopeIndexesEverything(t *testing.T) {
	reg := NewRegistry() // scope is nil
	if err := reg.addResource("p.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/StructureDefinition/Patient","type":"Patient","derivation":""}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs","status":"active"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs","status":"draft"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("sp.json", []byte(`{"resourceType":"SearchParameter","url":"http://example.org/SearchParameter/name","code":"name","base":["Patient"],"type":"string"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if err := reg.addResource("res.json", []byte(`{"resourceType":"Patient","id":"p1"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.Definition("http://example.org/StructureDefinition/Patient"); !ok {
		t.Error("nil scope should index SD")
	}
	if _, ok := reg.ValueSet("http://example.org/ValueSet/vs"); !ok {
		t.Error("nil scope should index ValueSet")
	}
	if _, ok := reg.CodeSystem("http://example.org/CodeSystem/cs"); !ok {
		t.Error("nil scope should index CodeSystem")
	}
	if _, ok := reg.SearchParameter("Patient", "name"); !ok {
		t.Error("nil scope should index SearchParameter")
	}
	if len(reg.ResourcesForType("Patient")) != 1 {
		t.Error("nil scope should index generic Resource")
	}
}

func TestAddResourceWithScopeNoneCapabilityStatements(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCapabilityStatements(ScopeNone)
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CapabilityStatement","url":"http://example.org/CapabilityStatement/cs","fhirVersion":"4.0.1"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if len(reg.CapabilityStatements()) != 0 {
		t.Error("CapabilityStatement should be filtered out with ScopeNone")
	}
}
