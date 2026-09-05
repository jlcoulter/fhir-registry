package fhir

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// JSON unmarshaling
// ---------------------------------------------------------------------------

func TestValueSetUnmarshal(t *testing.T) {
	data := []byte(`{
		"resourceType": "ValueSet",
		"url": "http://example.org/ValueSet/vs",
		"version": "1.0.0",
		"name": "ExampleVS",
		"status": "active",
		"compose": {
			"include": [
				{"system": "http://example.org/cs", "concept": [{"code": "a", "display": "A"}]}
			]
		},
		"expansion": {
			"contains": [
				{"system": "http://example.org/cs", "code": "a", "display": "A",
				 "contains": [{"system": "http://example.org/cs", "code": "a1", "display": "A1"}]}
			]
		}
	}`)
	var vs ValueSet
	if err := json.Unmarshal(data, &vs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if vs.URL != "http://example.org/ValueSet/vs" || vs.Version != "1.0.0" || vs.Name != "ExampleVS" || vs.Status != "active" {
		t.Errorf("scalar fields = %+v", vs)
	}
	if vs.Compose == nil || len(vs.Compose.Include) != 1 {
		t.Fatalf("compose.include = %+v", vs.Compose)
	}
	inc := vs.Compose.Include[0]
	if inc.System != "http://example.org/cs" || len(inc.Concept) != 1 || inc.Concept[0].Code != "a" || inc.Concept[0].Display != "A" {
		t.Errorf("include = %+v", inc)
	}
	if vs.Expansion == nil || len(vs.Expansion.Contains) != 1 {
		t.Fatalf("expansion.contains = %+v", vs.Expansion)
	}
	top := vs.Expansion.Contains[0]
	if top.Code != "a" || len(top.Contains) != 1 || top.Contains[0].Code != "a1" {
		t.Errorf("expansion contains = %+v", top)
	}
}

func TestCodeSystemUnmarshal(t *testing.T) {
	data := []byte(`{
		"resourceType": "CodeSystem",
		"url": "http://example.org/CodeSystem/cs",
		"version": "2.0.0",
		"name": "ExampleCS",
		"status": "draft",
		"concept": [
			{"code": "a", "display": "A",
			 "concept": [{"code": "a1", "display": "A1"}]}
		]
	}`)
	var cs CodeSystem
	if err := json.Unmarshal(data, &cs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cs.URL != "http://example.org/CodeSystem/cs" || cs.Version != "2.0.0" || cs.Name != "ExampleCS" || cs.Status != "draft" {
		t.Errorf("scalar fields = %+v", cs)
	}
	if len(cs.Concepts) != 1 || cs.Concepts[0].Code != "a" || len(cs.Concepts[0].Concepts) != 1 || cs.Concepts[0].Concepts[0].Code != "a1" {
		t.Errorf("concepts = %+v", cs.Concepts)
	}
}

func TestCapabilityStatementUnmarshal(t *testing.T) {
	data := []byte(`{
		"resourceType": "CapabilityStatement",
		"url": "http://example.org/CapabilityStatement/cs",
		"version": "1.0.0",
		"name": "ExampleCS",
		"status": "active",
		"fhirVersion": "4.0.1",
		"rest": [{
			"mode": "server",
			"resource": [{
				"type": "Patient",
				"profile": "http://example.org/StructureDefinition/Patient",
				"supportedProfile": ["http://example.org/StructureDefinition/au-patient"],
				"interaction": [{"code": "read"}],
				"operation": [{"name": "validate", "definition": "http://example.org/OperationDefinition/validate"}],
				"searchParam": [{"name": "name", "definition": "http://example.org/SearchParameter/name", "type": "string"}]
			}]
		}]
	}`)
	var cs CapabilityStatement
	if err := json.Unmarshal(data, &cs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cs.FhirVersion != "4.0.1" || len(cs.Rest) != 1 || cs.Rest[0].Mode != "server" {
		t.Fatalf("rest = %+v", cs.Rest)
	}
	res := cs.Rest[0].Resource[0]
	if res.Type != "Patient" || res.Profile != "http://example.org/StructureDefinition/Patient" {
		t.Errorf("resource = %+v", res)
	}
	if len(res.SupportedProfile) != 1 || res.SupportedProfile[0] != "http://example.org/StructureDefinition/au-patient" {
		t.Errorf("supportedProfile = %v", res.SupportedProfile)
	}
	if len(res.Interaction) != 1 || res.Interaction[0].Code != "read" {
		t.Errorf("interaction = %+v", res.Interaction)
	}
	if len(res.Operation) != 1 || res.Operation[0].Name != "validate" {
		t.Errorf("operation = %+v", res.Operation)
	}
	if len(res.SearchParam) != 1 || res.SearchParam[0].Name != "name" || res.SearchParam[0].Type != "string" {
		t.Errorf("searchParam = %+v", res.SearchParam)
	}
}

func TestSearchParameterUnmarshal(t *testing.T) {
	data := []byte(`{
		"resourceType": "SearchParameter",
		"url": "http://example.org/SearchParameter/name",
		"name": "name",
		"code": "name",
		"base": ["Patient", "Practitioner"],
		"type": "string",
		"expression": "Patient.name"
	}`)
	var sp SearchParameter
	if err := json.Unmarshal(data, &sp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if sp.URL != "http://example.org/SearchParameter/name" || sp.Code != "name" || sp.Type != "string" || sp.Expression != "Patient.name" {
		t.Errorf("scalar fields = %+v", sp)
	}
	if len(sp.Base) != 2 || sp.Base[0] != "Patient" || sp.Base[1] != "Practitioner" {
		t.Errorf("base = %v", sp.Base)
	}
}

// ---------------------------------------------------------------------------
// addResource dispatch
// ---------------------------------------------------------------------------

func TestAddResourceValueSet(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs","status":"active"}`)
	if err := reg.addResource("vs.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	vs, ok := reg.ValueSet("http://example.org/ValueSet/vs")
	if !ok || vs == nil {
		t.Fatal("ValueSet not indexed by URL")
	}
	if vs.Status != "active" {
		t.Errorf("status = %q", vs.Status)
	}
}

func TestAddResourceCodeSystem(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs","status":"draft"}`)
	if err := reg.addResource("cs.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	cs, ok := reg.CodeSystem("http://example.org/CodeSystem/cs")
	if !ok || cs == nil {
		t.Fatal("CodeSystem not indexed by URL")
	}
	if cs.Status != "draft" {
		t.Errorf("status = %q", cs.Status)
	}
}

func TestAddResourceCapabilityStatement(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"CapabilityStatement","url":"http://example.org/CapabilityStatement/cs","fhirVersion":"4.0.1"}`)
	if err := reg.addResource("cs.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	all := reg.CapabilityStatements()
	if len(all) != 1 || all[0].FhirVersion != "4.0.1" {
		t.Errorf("capabilityStatements = %+v", all)
	}
}

func TestAddResourceSearchParameter(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"SearchParameter","url":"http://example.org/SearchParameter/name","code":"name","base":["Patient","Practitioner"],"type":"string"}`)
	if err := reg.addResource("sp.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	// Indexed for each base type.
	if sp, ok := reg.SearchParameter("Patient", "name"); !ok || sp == nil {
		t.Error("SearchParameter not indexed for Patient")
	}
	if sp, ok := reg.SearchParameter("Practitioner", "name"); !ok || sp == nil {
		t.Error("SearchParameter not indexed for Practitioner")
	}
	if _, ok := reg.SearchParameter("Organization", "name"); ok {
		t.Error("SearchParameter should not be indexed for Organization")
	}
	if len(reg.SearchParameters()) != 1 {
		t.Errorf("SearchParameters() len = %d, want 1", len(reg.SearchParameters()))
	}
}

func TestAddResourceGenericResource(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{
		"resourceType": "Patient",
		"id": "p1",
		"meta": {"profile": ["http://example.org/StructureDefinition/au-patient"]},
		"active": true
	}`)
	if err := reg.addResource("patient.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	resources := reg.ResourcesForType("Patient")
	if len(resources) != 1 {
		t.Fatalf("ResourcesForType(Patient) len = %d, want 1", len(resources))
	}
	res := resources[0]
	if res.ResourceType != "Patient" {
		t.Errorf("ResourceType = %q, want Patient", res.ResourceType)
	}
	if len(res.ProfileURLs) != 1 || res.ProfileURLs[0] != "http://example.org/StructureDefinition/au-patient" {
		t.Errorf("ProfileURLs = %v", res.ProfileURLs)
	}
	if res.Raw["id"] != "p1" || res.Raw["active"] != true {
		t.Errorf("Raw = %+v", res.Raw)
	}
}

func TestAddResourceNonResourceIgnored(t *testing.T) {
	reg := NewRegistry()
	// package.json and .index.json have no resourceType and must be ignored.
	if err := reg.addResource("package.json", []byte(`{"name":"x","version":"1.0.0"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if len(reg.AllResources()) != 0 {
		t.Errorf("AllResources() = %d, want 0", len(reg.AllResources()))
	}
}

// ---------------------------------------------------------------------------
// Add* methods
// ---------------------------------------------------------------------------

func TestAddValueSet(t *testing.T) {
	reg := NewRegistry()
	vs := &ValueSet{URL: "http://example.org/ValueSet/vs", Status: "active"}
	reg.AddValueSet(vs)
	got, ok := reg.ValueSet("http://example.org/ValueSet/vs")
	if !ok || got != vs {
		t.Errorf("ValueSet() = %v, %v; want same pointer, true", got, ok)
	}
}

func TestAddCodeSystem(t *testing.T) {
	reg := NewRegistry()
	cs := &CodeSystem{URL: "http://example.org/CodeSystem/cs", Status: "draft"}
	reg.AddCodeSystem(cs)
	got, ok := reg.CodeSystem("http://example.org/CodeSystem/cs")
	if !ok || got != cs {
		t.Errorf("CodeSystem() = %v, %v; want same pointer, true", got, ok)
	}
}

func TestAddCapabilityStatement(t *testing.T) {
	reg := NewRegistry()
	cs := &CapabilityStatement{URL: "http://example.org/CapabilityStatement/cs", FhirVersion: "4.0.1"}
	reg.AddCapabilityStatement(cs)
	all := reg.CapabilityStatements()
	if len(all) != 1 || all[0] != cs {
		t.Errorf("CapabilityStatements() = %+v, want [cs]", all)
	}
}

func TestAddSearchParameter(t *testing.T) {
	reg := NewRegistry()
	sp := &SearchParameter{URL: "http://example.org/SearchParameter/name", Code: "name", Base: []string{"Patient"}}
	reg.AddSearchParameter(sp)
	got, ok := reg.SearchParameter("Patient", "name")
	if !ok || got != sp {
		t.Errorf("SearchParameter() = %v, %v; want same pointer, true", got, ok)
	}
}

func TestAddResource(t *testing.T) {
	reg := NewRegistry()
	res := &Resource{ResourceType: "Observation", Raw: map[string]any{"id": "o1"}}
	reg.AddResource(res)
	got := reg.ResourcesForType("Observation")
	if len(got) != 1 || got[0] != res {
		t.Errorf("ResourcesForType() = %+v, want [res]", got)
	}
}

// ---------------------------------------------------------------------------
// Collection methods
// ---------------------------------------------------------------------------

func TestAllResources(t *testing.T) {
	reg := NewRegistry()
	reg.AddResource(&Resource{ResourceType: "Patient", Raw: map[string]any{"id": "p1"}})
	reg.AddResource(&Resource{ResourceType: "Observation", Raw: map[string]any{"id": "o1"}})
	reg.AddResource(&Resource{ResourceType: "Patient", Raw: map[string]any{"id": "p2"}})
	all := reg.AllResources()
	if len(all) != 3 {
		t.Errorf("AllResources() len = %d, want 3", len(all))
	}
}

func TestResourcesForTypeEmpty(t *testing.T) {
	reg := NewRegistry()
	if got := reg.ResourcesForType("Patient"); len(got) != 0 {
		t.Errorf("ResourcesForType(Patient) = %d, want 0", len(got))
	}
}

func TestValueSetNotFound(t *testing.T) {
	reg := NewRegistry()
	if vs, ok := reg.ValueSet("http://example.org/ValueSet/missing"); ok || vs != nil {
		t.Errorf("ValueSet(missing) = %v, %v; want nil, false", vs, ok)
	}
}

func TestCodeSystemNotFound(t *testing.T) {
	reg := NewRegistry()
	if cs, ok := reg.CodeSystem("http://example.org/CodeSystem/missing"); ok || cs != nil {
		t.Errorf("CodeSystem(missing) = %v, %v; want nil, false", cs, ok)
	}
}
