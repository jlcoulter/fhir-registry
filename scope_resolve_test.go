package fhir

import (
	"testing"
)

// ---------------------------------------------------------------------------
// addResource buffers ValueSets/CodeSystems when policy is ScopeReferenced
// ---------------------------------------------------------------------------

func TestAddResource_ValueSetsReferenced_Buffered(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithValueSets(ScopeReferenced)
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs","status":"active"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("ValueSet should be buffered, not indexed, under ScopeReferenced")
	}
	if _, ok := reg.pendingValueSets["http://example.org/ValueSet/vs"]; !ok {
		t.Error("ValueSet should be in pendingValueSets under ScopeReferenced")
	}
}

func TestAddResource_CodeSystemsReferenced_Buffered(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeReferenced)
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs","status":"draft"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("CodeSystem should be buffered, not indexed, under ScopeReferenced")
	}
	if _, ok := reg.pendingCodeSystems["http://example.org/CodeSystem/cs"]; !ok {
		t.Error("CodeSystem should be in pendingCodeSystems under ScopeReferenced")
	}
}

func TestAddResource_ValueSetsAll_Immediate(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithValueSets(ScopeAll)
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs","status":"active"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs"]; !ok {
		t.Error("ValueSet should be indexed immediately under ScopeAll")
	}
	if _, ok := reg.pendingValueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("ValueSet should not be buffered under ScopeAll")
	}
}

func TestAddResource_CodeSystemsAll_Immediate(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeAll)
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs","status":"draft"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs"]; !ok {
		t.Error("CodeSystem should be indexed immediately under ScopeAll")
	}
	if _, ok := reg.pendingCodeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("CodeSystem should not be buffered under ScopeAll")
	}
}

func TestAddResource_ValueSetsNone_Skipped(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithValueSets(ScopeNone)
	if err := reg.addResource("vs.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs","status":"active"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("ValueSet should be skipped under ScopeNone")
	}
	if _, ok := reg.pendingValueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("ValueSet should not be buffered under ScopeNone")
	}
}

func TestAddResource_CodeSystemsNone_Skipped(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeNone)
	if err := reg.addResource("cs.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs","status":"draft"}`)); err != nil {
		t.Fatalf("addResource: %v", err)
	}
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("CodeSystem should be skipped under ScopeNone")
	}
	if _, ok := reg.pendingCodeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("CodeSystem should not be buffered under ScopeNone")
	}
}

// ---------------------------------------------------------------------------
// AddValueSet / AddCodeSystem respect scope
// ---------------------------------------------------------------------------

func TestAddValueSet_Referenced_Buffered(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithValueSets(ScopeReferenced)
	reg.AddValueSet(&ValueSet{URL: "http://example.org/ValueSet/vs"})
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("AddValueSet should buffer under ScopeReferenced")
	}
	if _, ok := reg.pendingValueSets["http://example.org/ValueSet/vs"]; !ok {
		t.Error("AddValueSet should buffer into pendingValueSets under ScopeReferenced")
	}
}

func TestAddValueSet_None_Skipped(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithValueSets(ScopeNone)
	reg.AddValueSet(&ValueSet{URL: "http://example.org/ValueSet/vs"})
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("AddValueSet should skip under ScopeNone")
	}
	if _, ok := reg.pendingValueSets["http://example.org/ValueSet/vs"]; ok {
		t.Error("AddValueSet should not buffer under ScopeNone")
	}
}

func TestAddValueSet_All_Indexed(t *testing.T) {
	reg := NewRegistry()
	reg.AddValueSet(&ValueSet{URL: "http://example.org/ValueSet/vs"})
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs"]; !ok {
		t.Error("AddValueSet should index immediately with nil scope")
	}
}

func TestAddCodeSystem_Referenced_Buffered(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeReferenced)
	reg.AddCodeSystem(&CodeSystem{URL: "http://example.org/CodeSystem/cs"})
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("AddCodeSystem should buffer under ScopeReferenced")
	}
	if _, ok := reg.pendingCodeSystems["http://example.org/CodeSystem/cs"]; !ok {
		t.Error("AddCodeSystem should buffer into pendingCodeSystems under ScopeReferenced")
	}
}

func TestAddCodeSystem_None_Skipped(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCodeSystems(ScopeNone)
	reg.AddCodeSystem(&CodeSystem{URL: "http://example.org/CodeSystem/cs"})
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("AddCodeSystem should skip under ScopeNone")
	}
	if _, ok := reg.pendingCodeSystems["http://example.org/CodeSystem/cs"]; ok {
		t.Error("AddCodeSystem should not buffer under ScopeNone")
	}
}

func TestAddCodeSystem_All_Indexed(t *testing.T) {
	reg := NewRegistry()
	reg.AddCodeSystem(&CodeSystem{URL: "http://example.org/CodeSystem/cs"})
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs"]; !ok {
		t.Error("AddCodeSystem should index immediately with nil scope")
	}
}

// ---------------------------------------------------------------------------
// Resolve
// ---------------------------------------------------------------------------

func sdWithBinding(url, vsURL string) *StructureDefinition {
	return &StructureDefinition{
		URL:  url,
		Type: "Patient",
		Snapshot: &Snapshot{
			Elements: []RawElement{
				{Binding: &RawBinding{ValueSet: vsURL}},
			},
		},
	}
}

func TestResolve_NilScope_NoOp(t *testing.T) {
	reg := NewRegistry()
	reg.Resolve()
	if len(reg.pendingValueSets) != 0 || len(reg.pendingCodeSystems) != 0 {
		t.Error("Resolve with nil scope should be a no-op")
	}
}

func TestResolve_ScopeAll_NoOp(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope() // all ScopeAll
	reg.Resolve()
	if len(reg.pendingValueSets) != 0 || len(reg.pendingCodeSystems) != 0 {
		t.Error("Resolve with all-ScopeAll should be a no-op")
	}
}

func TestResolve_ReferencedValueSets_Simple(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithValueSets(ScopeReferenced)
	reg.addResource("sd.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/SD/Patient","type":"Patient","snapshot":{"element":[{"binding":{"valueSet":"http://example.org/ValueSet/vs1"}}]}}`))
	reg.addResource("vs1.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs1","status":"active"}`))
	reg.addResource("vs2.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs2","status":"active"}`))
	reg.Resolve()
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs1"]; !ok {
		t.Error("referenced ValueSet vs1 should be indexed after Resolve")
	}
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs2"]; ok {
		t.Error("unreferenced ValueSet vs2 should not be indexed")
	}
}

func TestResolve_ReferencedCodeSystems_Simple(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithCodeSystems(ScopeReferenced)
	reg.addResource("sd.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/SD/Patient","type":"Patient","snapshot":{"element":[{"binding":{"valueSet":"http://example.org/ValueSet/vs1"}}]}}`))
	reg.addResource("vs1.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs1","status":"active","compose":{"include":[{"system":"http://example.org/CodeSystem/cs1"}]}}`))
	reg.addResource("cs1.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs1","status":"draft"}`))
	reg.addResource("cs2.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs2","status":"draft"}`))
	reg.Resolve()
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs1"]; !ok {
		t.Error("referenced CodeSystem cs1 should be indexed after Resolve")
	}
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs2"]; ok {
		t.Error("unreferenced CodeSystem cs2 should not be indexed")
	}
}

func TestResolve_TransitiveValueSets(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithValueSets(ScopeReferenced).WithCodeSystems(ScopeReferenced)
	reg.addResource("sd.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/SD/Patient","type":"Patient","snapshot":{"element":[{"binding":{"valueSet":"http://example.org/ValueSet/vsA"}}]}}`))
	// vsA references vsB; vsB references csC.
	reg.addResource("vsA.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vsA","status":"active","compose":{"include":[{"valueSet":["http://example.org/ValueSet/vsB"]}]}}`))
	reg.addResource("vsB.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vsB","status":"active","compose":{"include":[{"system":"http://example.org/CodeSystem/csC"}]}}`))
	reg.addResource("vsUnref.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vsUnref","status":"active"}`))
	reg.addResource("csC.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/csC","status":"draft"}`))
	reg.Resolve()
	if _, ok := reg.valueSets["http://example.org/ValueSet/vsA"]; !ok {
		t.Error("vsA should be indexed")
	}
	if _, ok := reg.valueSets["http://example.org/ValueSet/vsB"]; !ok {
		t.Error("transitively referenced vsB should be indexed")
	}
	if _, ok := reg.valueSets["http://example.org/ValueSet/vsUnref"]; ok {
		t.Error("unreferenced vsUnref should not be indexed")
	}
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/csC"]; !ok {
		t.Error("transitively referenced csC should be indexed")
	}
}

func TestResolve_VersionCanonicalization(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithValueSets(ScopeReferenced)
	reg.addResource("sd.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/SD/Patient","type":"Patient","snapshot":{"element":[{"binding":{"valueSet":"http://example.org/ValueSet/vs1|4.0.1"}}]}}`))
	reg.addResource("vs1.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs1","status":"active"}`))
	reg.Resolve()
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs1"]; !ok {
		t.Error("ValueSet should match despite version fragment in binding")
	}
}

func TestResolve_Idempotent(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithValueSets(ScopeReferenced)
	reg.addResource("sd.json", []byte(`{"resourceType":"StructureDefinition","url":"http://example.org/SD/Patient","type":"Patient","snapshot":{"element":[{"binding":{"valueSet":"http://example.org/ValueSet/vs1"}}]}}`))
	reg.addResource("vs1.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs1","status":"active"}`))
	reg.Resolve()
	reg.Resolve()
	if _, ok := reg.valueSets["http://example.org/ValueSet/vs1"]; !ok {
		t.Error("ValueSet should be indexed after first Resolve")
	}
	if len(reg.pendingValueSets) != 0 {
		t.Error("pendingValueSets should be cleared after Resolve")
	}
}

func TestResolve_CodeSystemsReferenced_WithAllValueSets(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient").WithCodeSystems(ScopeReferenced)
	// ValueSets are ScopeAll (default), so they index immediately.
	reg.addResource("vs1.json", []byte(`{"resourceType":"ValueSet","url":"http://example.org/ValueSet/vs1","status":"active","compose":{"include":[{"system":"http://example.org/CodeSystem/cs1"}]}}`))
	reg.addResource("cs1.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs1","status":"draft"}`))
	reg.addResource("cs2.json", []byte(`{"resourceType":"CodeSystem","url":"http://example.org/CodeSystem/cs2","status":"draft"}`))
	reg.Resolve()
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs1"]; !ok {
		t.Error("CodeSystem referenced by indexed ValueSet should be indexed")
	}
	if _, ok := reg.codeSystems["http://example.org/CodeSystem/cs2"]; ok {
		t.Error("unreferenced CodeSystem cs2 should not be indexed")
	}
}
