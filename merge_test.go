package fhir

import (
	"encoding/json"
	"testing"
)

func intptr(i int) *int { return &i }

func boolptr(b bool) *bool { return &b }

func TestMergeDifferential(t *testing.T) {
	base := []RawElement{
		{ID: "Org", Path: "Org"},
		{ID: "Org.name", Path: "Org.name", Min: intptr(0), Max: json.RawMessage(`"1"`), Types: []RawType{{Code: "string"}}},
		{ID: "Org.identifier", Path: "Org.identifier", Min: intptr(0), Max: json.RawMessage(`"*"`), Types: []RawType{{Code: "Identifier"}}},
	}

	diff := []RawElement{
		// Narrow min of name to 1.
		{ID: "Org.name", Path: "Org.name", Min: intptr(1)},
		// Narrow type of identifier to a profile.
		{ID: "Org.identifier", Path: "Org.identifier", Types: []RawType{{Code: "Identifier", Profiles: []string{"http://x"}}}},
	}

	merged := MergeDifferential(base, diff)
	if len(merged) != 3 {
		t.Fatalf("merged len = %d, want 3", len(merged))
	}
	var name, idf *RawElement
	for i := range merged {
		switch merged[i].ID {
		case "Org.name":
			name = &merged[i]
		case "Org.identifier":
			idf = &merged[i]
		}
	}
	if name == nil || name.Min == nil || *name.Min != 1 {
		t.Errorf("name min = %+v, want 1", name)
	}
	if idf == nil || len(idf.Types) != 1 || idf.Types[0].Profiles[0] != "http://x" {
		t.Errorf("identifier types not narrowed: %+v", idf)
	}
}

func TestMergeDifferentialNewElement(t *testing.T) {
	base := []RawElement{
		{ID: "P", Path: "P"},
		{ID: "P.extension", Path: "P.extension", Min: intptr(0), Max: json.RawMessage(`"*"`)},
	}
	diff := []RawElement{
		{ID: "P.extension:myExt", Path: "P.extension", SliceName: "myExt", Min: intptr(0), Max: json.RawMessage(`"1"`)},
	}
	merged := MergeDifferential(base, diff)
	if len(merged) != 3 {
		t.Fatalf("merged len = %d, want 3", len(merged))
	}
	// The slice element must be present and keep its path/sliceName.
	var found bool
	for _, e := range merged {
		if e.ID == "P.extension:myExt" && e.SliceName == "myExt" {
			found = true
		}
	}
	if !found {
		t.Error("new slice element not preserved")
	}
	// Ordering: parent must come before the slice.
	if merged[0].Path != "P" {
		t.Errorf("first element = %s, want P", merged[0].Path)
	}
}

func TestMergeDifferentialOrder(t *testing.T) {
	base := []RawElement{
		{ID: "X", Path: "X"},
		{ID: "X.a", Path: "X.a"},
	}
	// Insert a deep child before its parent appears, to verify sorting.
	diff := []RawElement{
		{ID: "X.a.b", Path: "X.a.b"},
	}
	merged := MergeDifferential(base, diff)
	if len(merged) != 3 {
		t.Fatalf("len = %d", len(merged))
	}
	if merged[0].Path != "X" || merged[1].Path != "X.a" || merged[2].Path != "X.a.b" {
		t.Errorf("bad order: %s, %s, %s", merged[0].Path, merged[1].Path, merged[2].Path)
	}
}

// TestOverlayMinRelaxation verifies that an explicit min: 0 in a differential
// relaxes a required base element to optional. FHIR distinguishes min: 0 from
// an absent min, so the overlay must not treat 0 as "unset".
func TestOverlayMinRelaxation(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.required", Path: "R.required", Min: intptr(1), Max: json.RawMessage(`"1"`)},
	}
	diff := []RawElement{
		{ID: "R.required", Path: "R.required", Min: intptr(0)},
	}
	merged := MergeDifferential(base, diff)
	if len(merged) != 2 {
		t.Fatalf("merged len = %d, want 2", len(merged))
	}
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.required" {
			got = &merged[i]
		}
	}
	if got == nil {
		t.Fatal("R.required not found in merged result")
	}
	if got.Min == nil || *got.Min != 0 {
		t.Errorf("R.required min = %+v, want 0 (relaxed from 1)", got.Min)
	}
}

// TestOverlayMinAbsentPreservesBase verifies that a differential without a min
// field leaves the base minimum untouched.
func TestOverlayMinAbsentPreservesBase(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.required", Path: "R.required", Min: intptr(1), Max: json.RawMessage(`"1"`)},
	}
	diff := []RawElement{
		{ID: "R.required", Path: "R.required"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.required" {
			got = &merged[i]
		}
	}
	if got == nil || got.Min == nil || *got.Min != 1 {
		t.Errorf("R.required min = %+v, want 1 (preserved)", got.Min)
	}
}

// TestOverlayIsModifierClear verifies that a differential explicitly setting
// isModifier to false overrides a base element that marked it a modifier.
func TestOverlayIsModifierClear(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.mod", Path: "R.mod", Min: intptr(0), Max: json.RawMessage(`"1"`), IsModifier: boolptr(true)},
	}
	diff := []RawElement{
		{ID: "R.mod", Path: "R.mod", IsModifier: boolptr(false)},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.mod" {
			got = &merged[i]
		}
	}
	if got == nil || got.IsModifier == nil || *got.IsModifier != false {
		t.Errorf("R.mod isModifier = %+v, want false (cleared)", got.IsModifier)
	}
}

// TestOverlayIsModifierAbsentPreservesBase verifies that a differential without
// an isModifier field leaves the base modifier flag untouched.
func TestOverlayIsModifierAbsentPreservesBase(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.mod", Path: "R.mod", Min: intptr(0), Max: json.RawMessage(`"1"`), IsModifier: boolptr(true)},
	}
	diff := []RawElement{
		{ID: "R.mod", Path: "R.mod"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.mod" {
			got = &merged[i]
		}
	}
	if got == nil || got.IsModifier == nil || *got.IsModifier != true {
		t.Errorf("R.mod isModifier = %+v, want true (preserved)", got.IsModifier)
	}
}

// TestOverlayIsSummaryClear verifies that a differential explicitly setting
// isSummary to false overrides a base element marked as summary.
func TestOverlayIsSummaryClear(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.sum", Path: "R.sum", Min: intptr(0), Max: json.RawMessage(`"1"`), IsSummary: boolptr(true)},
	}
	diff := []RawElement{
		{ID: "R.sum", Path: "R.sum", IsSummary: boolptr(false)},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.sum" {
			got = &merged[i]
		}
	}
	if got == nil || got.IsSummary == nil || *got.IsSummary != false {
		t.Errorf("R.sum isSummary = %+v, want false (cleared)", got.IsSummary)
	}
}

// TestOverlayIsSummaryAbsentPreservesBase verifies that a differential without
// an isSummary field leaves the base summary flag untouched.
func TestOverlayIsSummaryAbsentPreservesBase(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.sum", Path: "R.sum", Min: intptr(0), Max: json.RawMessage(`"1"`), IsSummary: boolptr(true)},
	}
	diff := []RawElement{
		{ID: "R.sum", Path: "R.sum"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.sum" {
			got = &merged[i]
		}
	}
	if got == nil || got.IsSummary == nil || *got.IsSummary != true {
		t.Errorf("R.sum isSummary = %+v, want true (preserved)", got.IsSummary)
	}
}
