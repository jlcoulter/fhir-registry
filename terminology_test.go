package fhir

import (
	"testing"
)

func TestIsMeaningfulCoding(t *testing.T) {
	cases := []struct {
		code, display string
		want          bool
	}{
		{"a", "", true},
		{"", "A", false},
		{"  ", "", false},
		{"XX", "", false},         // v2-0203 null code
		{"UNK", "", false},        // placeholder
		{"_ActReason", "", false}, // v3 abstract/group code
		{"ACTIVE", "Active", true},
	}
	for _, c := range cases {
		if got := IsMeaningfulCoding(c.code, c.display); got != c.want {
			t.Errorf("IsMeaningfulCoding(%q,%q) = %v, want %v", c.code, c.display, got, c.want)
		}
	}
}

func TestIsPlaceholderURL(t *testing.T) {
	if !IsPlaceholderURL("http://www.acme.com/identifiers/patient") {
		t.Error("acme.com should be a placeholder URL")
	}
	if !IsPlaceholderURL("http://example.org/codes") {
		t.Error("example.org should be a placeholder URL")
	}
	if !IsPlaceholderURL("urn:oid:1.2.3.4.5") {
		t.Error("urn:oid test OID should be a placeholder URL")
	}
	if !IsPlaceholderURL("") {
		t.Error("empty URL should be a placeholder URL")
	}
	if IsPlaceholderURL("http://ns.electronichealth.net.au/id/hi/hpii/1.0") {
		t.Error("real AU system should not be a placeholder URL")
	}
	if IsPlaceholderURL("http://digitalhealth.gov.au/fhir/cc/CodeSystem/pbprn") {
		t.Error("real digitalhealth.gov.au system should not be a placeholder URL")
	}
}

func TestFirstConceptFromExpansion(t *testing.T) {
	reg := NewRegistry()
	vs := &ValueSet{URL: "http://example.org/ValueSet/vs", Status: "active",
		Expansion: &ValueSetExpansion{Contains: []ValueSetExpansionContains{
			{System: "http://example.org/cs", Code: "a", Display: "A"},
			{System: "http://example.org/cs", Code: "XX"}, // placeholder, skipped
		}}}
	reg.AddValueSet(vs)
	c, ok := reg.FirstConcept(vs)
	if !ok || c.Code != "a" || c.System != "http://example.org/cs" || c.Display != "A" {
		t.Errorf("FirstConcept = %+v, %v; want a/A", c, ok)
	}
}

func TestFirstConceptFromComposeAndCodeSystem(t *testing.T) {
	reg := NewRegistry()
	reg.AddCodeSystem(&CodeSystem{URL: "http://example.org/cs", Status: "active",
		Concepts: []CodeSystemConcept{{Code: "x", Display: "X"}}})
	vs := &ValueSet{URL: "http://example.org/ValueSet/vs", Status: "active",
		Compose: &ValueSetCompose{Include: []ValueSetInclude{
			{System: "http://example.org/cs"},
		}}}
	c, ok := reg.FirstConcept(vs)
	if !ok || c.Code != "x" || c.Display != "X" {
		t.Errorf("FirstConcept = %+v, %v; want x/X", c, ok)
	}
}

func TestFirstConceptNil(t *testing.T) {
	reg := NewRegistry()
	if _, ok := reg.FirstConcept(nil); ok {
		t.Error("FirstConcept(nil) should be false")
	}
	if _, ok := reg.FirstConcept(&ValueSet{URL: "u", Status: "active"}); ok {
		t.Error("FirstConcept(empty) should be false")
	}
}

func TestResolveBoundCodingVersioned(t *testing.T) {
	reg := NewRegistry()
	reg.AddValueSet(&ValueSet{URL: "http://example.org/ValueSet/vs", Status: "active",
		Compose: &ValueSetCompose{Include: []ValueSetInclude{
			{System: "http://example.org/cs", Concept: []ConceptReference{{Code: "a", Display: "A"}}},
		}}})
	// Versioned canonical URL must resolve to the versionless index.
	c, ok := reg.ResolveBoundCoding("http://example.org/ValueSet/vs|1.0.0")
	if !ok || c.Code != "a" {
		t.Errorf("ResolveBoundCoding = %+v, %v; want a", c, ok)
	}
	if _, ok := reg.ResolveBoundCoding("http://example.org/ValueSet/missing"); ok {
		t.Error("ResolveBoundCoding(missing) should be false")
	}
}

func TestCodingDisplay(t *testing.T) {
	reg := NewRegistry()
	reg.AddCodeSystem(&CodeSystem{URL: "http://example.org/cs", Status: "active",
		Concepts: []CodeSystemConcept{{Code: "a", Display: "A", Concepts: []CodeSystemConcept{{Code: "a1", Display: "A1"}}}}})
	if d := reg.CodingDisplay("http://example.org/cs", "a"); d != "A" {
		t.Errorf("CodingDisplay(a) = %q, want A", d)
	}
	if d := reg.CodingDisplay("http://example.org/cs", "a1"); d != "A1" {
		t.Errorf("CodingDisplay(a1) = %q, want A1", d)
	}
	if d := reg.CodingDisplay("http://example.org/cs", "missing"); d != "" {
		t.Errorf("CodingDisplay(missing) = %q, want empty", d)
	}
	if d := reg.CodingDisplay("http://example.org/unknown", "a"); d != "" {
		t.Errorf("CodingDisplay(unknown) = %q, want empty", d)
	}
}

func TestFirstExampleCoding(t *testing.T) {
	reg := NewRegistry()
	reg.AddResource(&Resource{ResourceType: "Patient", ProfileURLs: []string{"http://example.org/StructureDefinition/au-patient"},
		Raw: map[string]any{"code": map[string]any{"coding": []any{
			map[string]any{"system": "http://example.org/cs", "code": "a", "display": "A"},
		}}}})
	// Prefer the profile-matched instance.
	c, ok := reg.FirstExampleCoding("Patient", "code", "http://example.org/StructureDefinition/au-patient")
	if !ok || c.Code != "a" || c.Display != "A" {
		t.Errorf("FirstExampleCoding = %+v, %v; want a/A", c, ok)
	}
	if _, ok := reg.FirstExampleCoding("Patient", "missing", ""); ok {
		t.Error("FirstExampleCoding(missing path) should be false")
	}
	if _, ok := reg.FirstExampleCoding("UnknownType", "code", ""); ok {
		t.Error("FirstExampleCoding(unknown type) should be false")
	}
}

func TestFirstExampleCodingArrayPath(t *testing.T) {
	reg := NewRegistry()
	reg.AddResource(&Resource{ResourceType: "Patient",
		Raw: map[string]any{"communication": []any{
			map[string]any{"language": map[string]any{"coding": []any{
				map[string]any{"system": "http://example.org/cs", "code": "en", "display": "English"},
			}}},
		}}})
	c, ok := reg.FirstExampleCoding("Patient", "communication.language", "")
	if !ok || c.Code != "en" || c.Display != "English" {
		t.Errorf("FirstExampleCoding(array) = %+v, %v; want en/English", c, ok)
	}
}

func TestExampleCodingForExtension(t *testing.T) {
	reg := NewRegistry()
	reg.AddResource(&Resource{ResourceType: "HealthcareService",
		Raw: map[string]any{"extension": []any{
			map[string]any{"url": "http://example.org/StructureDefinition/deactivated",
				"valueCodeableConcept": map[string]any{"coding": []any{
					map[string]any{"system": "http://example.org/cs", "code": "beta", "display": "Beta"},
				}}},
		}}})
	c, ok := reg.ExampleCodingForExtension("http://example.org/StructureDefinition/deactivated|1.0.0")
	if !ok || c.Code != "beta" {
		t.Errorf("ExampleCodingForExtension = %+v, %v; want beta", c, ok)
	}
	if _, ok := reg.ExampleCodingForExtension("http://example.org/StructureDefinition/missing"); ok {
		t.Error("ExampleCodingForExtension(missing) should be false")
	}
}
