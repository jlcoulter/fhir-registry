package fhir

import (
	"strings"
	"testing"
)

// TestMarshalArrayComplexTypeResolution verifies that complex objects inside a
// repeating element are resolved through the registry so that cardinality on
// their children is enforced. The AU Identifier profile (au-ihi) requires
// type, system and value; omitting a required child in an array item must be
// reported as a violation.
func TestMarshalArrayComplexTypeResolution(t *testing.T) {
	reg := loadTestRegistry(t)
	in := map[string]any{
		"resourceType": "Patient",
		"identifier": []any{
			// Missing required "type" (min=1) for au-ihi.
			map[string]any{"system": "urn:x", "value": "123"},
		},
	}
	_, rep, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if rep == nil {
		t.Fatal("nil report")
	}
	var missing bool
	for _, it := range rep.Items {
		if it.Path == "Identifier.type" && it.Severity == SeverityViolation {
			missing = true
		}
	}
	if !missing {
		t.Errorf("expected a violation for missing required Identifier.type, got %+v", rep.Items)
	}
}

// TestMarshalBareArrayComplexType verifies the []any branch of marshalValue:
// a bare array of complex objects must still resolve the element's type
// (au-ihi) and wrap repeating children such as Address.line into arrays.
func TestMarshalBareArrayComplexType(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/au-organization")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	rep := &MarshalReport{}
	root := tree.ByPath["Organization.address"][0]

	v := []any{
		map[string]any{
			"use":  "home",
			"line": "31 Pacquola St",
		},
	}
	out, err := marshalValue(reg, root, tree, v, rep)
	if err != nil {
		t.Fatalf("marshalValue: %v", err)
	}
	items, ok := out.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("out = %#v, want []any of length 1", out)
	}
	addr, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("item = %#v, want map", items[0])
	}
	if addr["use"] != "home" {
		t.Errorf("use = %#v, want home", addr["use"])
	}
	// Repeating Address.line (0..*) must be wrapped into an array.
	lines, ok := addr["line"].([]any)
	if !ok || len(lines) != 1 || lines[0] != "31 Pacquola St" {
		t.Errorf("Address.line should be wrapped to array, got %#v", addr["line"])
	}
}

// TestMarshalBareArrayRequiredChild verifies that a bare []any of complex
// objects reports a violation for a required child that is missing, exercising
// the []any branch reaching marshalObject's cardinality checks.
func TestMarshalBareArrayRequiredChild(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/au-patient")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	rep := &MarshalReport{}
	// Patient.identifier resolves to au-ihi, whose "type" is required (min=1).
	elem := tree.ByPath["Patient.identifier"][0]

	v := []any{
		map[string]any{"system": "urn:x", "value": "123"},
	}
	if _, err := marshalValue(reg, elem, tree, v, rep); err != nil {
		t.Fatalf("marshalValue: %v", err)
	}
	var missing bool
	for _, it := range rep.Items {
		if it.Path == "Identifier.type" && it.Severity == SeverityViolation {
			missing = true
		}
	}
	if !missing {
		t.Errorf("expected a violation for missing Identifier.type, got %+v", rep.Items)
	}
}

// TestMarshalBareArrayScalarUnwrap verifies that a single-element array for a
// scalar (max=1) element is unwrapped to a scalar, matching the behavior of
// marshalObject. The []any branch of marshalValue must not keep it an array.
func TestMarshalBareArrayScalarUnwrap(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/au-patient")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	rep := &MarshalReport{}
	// Patient.active is a scalar 0..1 element.
	elem := tree.ByPath["Patient.active"][0]

	out, err := marshalValue(reg, elem, tree, []any{true}, rep)
	if err != nil {
		t.Fatalf("marshalValue: %v", err)
	}
	if out != true {
		t.Errorf("single-element array for scalar element = %#v, want true", out)
	}
}

// TestMarshalObjectDuplicateLastSegment verifies that marshalObject matches a
// JSON key to exactly one element even when two sibling elements share the
// same last path segment. The first (direct) child must win; the colliding
// sibling must not re-process the same value, which would otherwise emit a
// spurious cardinality violation for a key that belongs to the first child.
func TestMarshalObjectDuplicateLastSegment(t *testing.T) {
	reg := NewRegistry()
	parent := &ElementDefinition{
		ID:   "TestParent",
		Path: "TestParent",
		Children: []*ElementDefinition{
			// Direct child: the key "name" belongs to this element, whose
			// required child is present in the input.
			{ID: "TestParent.name", Path: "TestParent.name", Min: 1, Max: 1, Types: []ElementType{{Code: "TestComplex"}}, Children: []*ElementDefinition{
				{ID: "TestParent.name.a", Path: "TestParent.name.a", Min: 1, Max: 1, Types: []ElementType{{Code: "string"}}},
			}},
			// Colliding sibling: same last segment, but a different complex
			// type with a different required child. If it re-processes the
			// value it reports a violation that must not occur.
			{ID: "TestParent.alias.name", Path: "TestParent.alias.name", Min: 1, Max: 1, Types: []ElementType{{Code: "TestComplex"}}, Children: []*ElementDefinition{
				{ID: "TestParent.alias.name.b", Path: "TestParent.alias.name.b", Min: 1, Max: 1, Types: []ElementType{{Code: "string"}}},
			}},
		},
	}
	tree := &ElementTree{
		Root: parent,
		ByPath: map[string][]*ElementDefinition{
			"TestParent":            {parent},
			"TestParent.name":       {parent.Children[0]},
			"TestParent.alias.name": {parent.Children[1]},
		},
		ByID: map[string]*ElementDefinition{
			"TestParent":            parent,
			"TestParent.name":       parent.Children[0],
			"TestParent.alias.name": parent.Children[1],
		},
	}

	rep := &MarshalReport{}
	out, err := marshalObject(reg, parent, tree, map[string]any{"name": map[string]any{"a": "x"}}, rep)
	if err != nil {
		t.Fatalf("marshalObject: %v", err)
	}
	m := out.(map[string]any)
	if _, ok := m["name"]; !ok {
		t.Errorf("name key missing from output: %#v", m)
	}
	for _, it := range rep.Items {
		if it.Severity == SeverityViolation {
			t.Errorf("unexpected violation: %+v", it)
		}
	}
}

// TestMarshalExtensionBothValueAndSubExtensions verifies that an Extension
// carrying both a value[x] and nested sub-extensions is reported as a violation
// and the value[x] is stripped, satisfying FHIR's ext-1 invariant.
func TestMarshalExtensionBothValueAndSubExtensions(t *testing.T) {
	reg := loadTestRegistry(t)
	instance := map[string]any{
		"resourceType": "Patient",
		"extension": []any{
			map[string]any{
				"url":         "http://example.org/StructureDefinition/Ext",
				"valueString": "some-value",
				"extension": []any{
					map[string]any{"url": "sub", "valueString": "nested"},
				},
			},
		},
	}
	out, rep, err := reg.Marshal("Patient", instance)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var found bool
	for _, it := range rep.Items {
		if it.Severity == SeverityViolation && strings.Contains(it.Message, "must not have both a value") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a violation for extension with both value and sub-extensions, got %+v", rep.Items)
	}

	exts, ok := out["extension"].([]any)
	if !ok || len(exts) != 1 {
		t.Fatalf("extension = %#v, want []any of length 1", out["extension"])
	}
	ext, ok := exts[0].(map[string]any)
	if !ok {
		t.Fatalf("extension item = %#v, want map", exts[0])
	}
	if _, ok := ext["valueString"]; ok {
		t.Error("valueString should have been removed from output")
	}
	if _, ok := ext["extension"]; !ok {
		t.Error("extension should still be present in output")
	}
}

// TestMarshalSimpleExtensionKeepsValue verifies that a simple extension (value
// but no sub-extensions) is left untouched.
func TestMarshalSimpleExtensionKeepsValue(t *testing.T) {
	reg := loadTestRegistry(t)
	instance := map[string]any{
		"resourceType": "Patient",
		"extension": []any{
			map[string]any{
				"url":         "http://example.org/StructureDefinition/Ext",
				"valueString": "some-value",
			},
		},
	}
	out, rep, err := reg.Marshal("Patient", instance)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, it := range rep.Items {
		if it.Severity == SeverityViolation {
			t.Errorf("unexpected violation: %+v", it)
		}
	}
	exts := out["extension"].([]any)
	ext := exts[0].(map[string]any)
	if ext["valueString"] != "some-value" {
		t.Errorf("valueString = %#v, want some-value", ext["valueString"])
	}
}
