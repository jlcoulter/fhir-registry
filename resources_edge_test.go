package fhir

import (
	"testing"
)

// ---------------------------------------------------------------------------
// profileURLsOf edge cases
// ---------------------------------------------------------------------------

func TestProfileURLsOf(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
		want []string
	}{
		{"no meta", map[string]any{"resourceType": "Patient"}, nil},
		{"meta no profile", map[string]any{"resourceType": "Patient", "meta": map[string]any{}}, nil},
		{"profile empty", map[string]any{"resourceType": "Patient", "meta": map[string]any{"profile": []any{}}}, nil},
		{"profile valid", map[string]any{"resourceType": "Patient", "meta": map[string]any{"profile": []any{"http://a", "http://b"}}}, []string{"http://a", "http://b"}},
		{"profile mixed types", map[string]any{"resourceType": "Patient", "meta": map[string]any{"profile": []any{"http://a", 123, "http://b"}}}, []string{"http://a", "http://b"}},
		{"profile not array", map[string]any{"resourceType": "Patient", "meta": map[string]any{"profile": "http://a"}}, nil},
		{"meta not object", map[string]any{"resourceType": "Patient", "meta": "http://a"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := profileURLsOf(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("profileURLsOf() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("profileURLsOf()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// addResource malformed JSON is silently skipped (consistent with the
// pre-existing StructureDefinition behavior: a resource that cannot be parsed
// is ignored rather than aborting the package load).
// ---------------------------------------------------------------------------

func TestAddResourceValueSetMalformedSkipped(t *testing.T) {
	reg := NewRegistry()
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","url":`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if vs, ok := reg.ValueSet(""); ok || vs != nil {
		t.Errorf("ValueSet(\"\") = %v, %v; want nil, false", vs, ok)
	}
}

func TestAddResourceCodeSystemMalformedSkipped(t *testing.T) {
	reg := NewRegistry()
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","url":`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if cs, ok := reg.CodeSystem(""); ok || cs != nil {
		t.Errorf("CodeSystem(\"\") = %v, %v; want nil, false", cs, ok)
	}
}

func TestAddResourceCapabilityStatementMalformedSkipped(t *testing.T) {
	reg := NewRegistry()
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CapabilityStatement","url":`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if len(reg.CapabilityStatements()) != 0 {
		t.Errorf("CapabilityStatements() len = %d, want 0", len(reg.CapabilityStatements()))
	}
}

func TestAddResourceSearchParameterMalformedSkipped(t *testing.T) {
	reg := NewRegistry()
	if err := reg.addResource("sp.json", []byte(`{"resourceType":"SearchParameter","url":`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if len(reg.SearchParameters()) != 0 {
		t.Errorf("SearchParameters() len = %d, want 0", len(reg.SearchParameters()))
	}
}

// ---------------------------------------------------------------------------
// empty URL skipped
// ---------------------------------------------------------------------------

func TestAddResourceValueSetEmptyURLSkipped(t *testing.T) {
	reg := NewRegistry()
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","status":"active"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if vs, ok := reg.ValueSet(""); ok || vs != nil {
		t.Errorf("ValueSet(\"\") = %v, %v; want nil, false", vs, ok)
	}
}

func TestAddResourceCodeSystemEmptyURLSkipped(t *testing.T) {
	reg := NewRegistry()
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","status":"draft"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if cs, ok := reg.CodeSystem(""); ok || cs != nil {
		t.Errorf("CodeSystem(\"\") = %v, %v; want nil, false", cs, ok)
	}
}

// ---------------------------------------------------------------------------
// SearchParameter edge cases
// ---------------------------------------------------------------------------

func TestAddResourceSearchParameterEmptyBase(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"SearchParameter","url":"http://example.org/SearchParameter/x","code":"x","base":[],"type":"string"}`)
	if err := reg.addResource("sp.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	// Present in the collection but not findable by (type, code).
	if len(reg.SearchParameters()) != 1 {
		t.Errorf("SearchParameters() len = %d, want 1", len(reg.SearchParameters()))
	}
	if _, ok := reg.SearchParameter("Patient", "x"); ok {
		t.Error("SearchParameter with empty base should not be findable by (type, code)")
	}
}

func TestSearchParameterLastWriteWins(t *testing.T) {
	reg := NewRegistry()
	first := &SearchParameter{URL: "http://example.org/SearchParameter/a", Code: "name", Base: []string{"Patient"}}
	second := &SearchParameter{URL: "http://example.org/SearchParameter/b", Code: "name", Base: []string{"Patient"}}
	reg.AddSearchParameter(first)
	reg.AddSearchParameter(second)
	got, ok := reg.SearchParameter("Patient", "name")
	if !ok || got != second {
		t.Errorf("SearchParameter() = %v, %v; want second (last write wins)", got, ok)
	}
	// Both remain in the collection.
	if len(reg.SearchParameters()) != 2 {
		t.Errorf("SearchParameters() len = %d, want 2", len(reg.SearchParameters()))
	}
}

// ---------------------------------------------------------------------------
// Generic Resource no meta / no profile key
// ---------------------------------------------------------------------------

func TestAddResourceGenericResourceNoMeta(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"Patient","id":"p1"}`)
	if err := reg.addResource("p.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	res := reg.ResourcesForType("Patient")
	if len(res) != 1 {
		t.Fatalf("ResourcesForType(Patient) len = %d, want 1", len(res))
	}
	if res[0].ProfileURLs != nil {
		t.Errorf("ProfileURLs = %v, want nil when no meta", res[0].ProfileURLs)
	}
}

func TestAddResourceGenericResourceNoProfileKey(t *testing.T) {
	reg := NewRegistry()
	data := []byte(`{"resourceType":"Patient","id":"p1","meta":{"versionId":"1"}}`)
	if err := reg.addResource("p.json", data); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	res := reg.ResourcesForType("Patient")
	if len(res) != 1 {
		t.Fatalf("ResourcesForType(Patient) len = %d, want 1", len(res))
	}
	if res[0].ProfileURLs != nil {
		t.Errorf("ProfileURLs = %v, want nil when meta has no profile", res[0].ProfileURLs)
	}
}
