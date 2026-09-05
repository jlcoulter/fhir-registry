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

// TestOverlayBindingReplacement verifies that a differential binding replaces
// the base binding.
func TestOverlayBindingReplacement(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.code", Path: "R.code", Min: intptr(0), Max: json.RawMessage(`"1"`),
			Binding: &RawBinding{Strength: "required", ValueSet: "http://base/vs"}},
	}
	diff := []RawElement{
		{ID: "R.code", Path: "R.code",
			Binding: &RawBinding{Strength: "extensible", ValueSet: "http://profile/vs"}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.code" {
			got = &merged[i]
		}
	}
	if got == nil || got.Binding == nil {
		t.Fatal("R.code binding missing")
	}
	if got.Binding.Strength != "extensible" || got.Binding.ValueSet != "http://profile/vs" {
		t.Errorf("binding = %+v, want profile binding", got.Binding)
	}
}

// TestOverlayConditionReplacement verifies that a differential condition list
// replaces the base condition list.
func TestOverlayConditionReplacement(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.cond", Path: "R.cond", Min: intptr(0), Max: json.RawMessage(`"1"`),
			Condition: []string{"base-1"}},
	}
	diff := []RawElement{
		{ID: "R.cond", Path: "R.cond", Condition: []string{"profile-1", "profile-2"}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.cond" {
			got = &merged[i]
		}
	}
	if got == nil || len(got.Condition) != 2 || got.Condition[0] != "profile-1" {
		t.Errorf("condition = %+v, want [profile-1 profile-2]", got.Condition)
	}
}

// TestOverlayContentReference verifies that a differential contentReference
// replaces the base contentReference.
func TestOverlayContentReference(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.ref", Path: "R.ref", Min: intptr(0), Max: json.RawMessage(`"1"`),
			ContentReference: "http://base/ref"},
	}
	diff := []RawElement{
		{ID: "R.ref", Path: "R.ref", ContentReference: "http://profile/ref"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.ref" {
			got = &merged[i]
		}
	}
	if got == nil || got.ContentReference != "http://profile/ref" {
		t.Errorf("contentReference = %q, want profile ref", got.ContentReference)
	}
}

// TestOverlaySlicingReplacement verifies that a differential slicing block
// replaces the base slicing block.
func TestOverlaySlicingReplacement(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.ext", Path: "R.ext", Min: intptr(0), Max: json.RawMessage(`"*"`),
			Slicing: &Slicing{Rules: "open"}},
	}
	diff := []RawElement{
		{ID: "R.ext", Path: "R.ext",
			Slicing: &Slicing{Rules: "closed", Ordered: true,
				Discriminator: []Discriminator{{Type: "value", Path: "url"}}}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.ext" {
			got = &merged[i]
		}
	}
	if got == nil || got.Slicing == nil {
		t.Fatal("R.ext slicing missing")
	}
	if got.Slicing.Rules != "closed" || !got.Slicing.Ordered {
		t.Errorf("slicing = %+v, want closed+ordered", got.Slicing)
	}
	if len(got.Slicing.Discriminator) != 1 || got.Slicing.Discriminator[0].Path != "url" {
		t.Errorf("discriminator = %+v, want [{value url}]", got.Slicing.Discriminator)
	}
}

// TestOverlayMeaningWhenMissing verifies that a differential meaningWhenMissing
// overrides the base value via the or() helper.
func TestOverlayMeaningWhenMissing(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.m", Path: "R.m", Min: intptr(0), Max: json.RawMessage(`"1"`),
			MeaningWhenMissing: "base meaning"},
	}
	diff := []RawElement{
		{ID: "R.m", Path: "R.m", MeaningWhenMissing: "profile meaning"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.m" {
			got = &merged[i]
		}
	}
	if got == nil || got.MeaningWhenMissing != "profile meaning" {
		t.Errorf("meaningWhenMissing = %q, want profile meaning", got.MeaningWhenMissing)
	}
}

// TestOverlayMaxReplacement verifies that a differential max replaces the base
// max, including relaxing to "*".
func TestOverlayMaxReplacement(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.m", Path: "R.m", Min: intptr(0), Max: json.RawMessage(`"1"`)},
	}
	diff := []RawElement{
		{ID: "R.m", Path: "R.m", Max: json.RawMessage(`"*"`)},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.m" {
			got = &merged[i]
		}
	}
	if got == nil || string(got.Max) != `"*"` {
		t.Errorf("max = %s, want \"*\"", got.Max)
	}
}

// TestOverlayMustSupport verifies that a differential mustSupport overrides the
// base, and an absent mustSupport leaves the base untouched.
func TestOverlayMustSupport(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.ms", Path: "R.ms", Min: intptr(0), Max: json.RawMessage(`"1"`)},
	}
	diff := []RawElement{
		{ID: "R.ms", Path: "R.ms", MustSupport: boolptr(true)},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.ms" {
			got = &merged[i]
		}
	}
	if got == nil || got.MustSupport == nil || !*got.MustSupport {
		t.Errorf("R.ms mustSupport = %+v, want true", got.MustSupport)
	}

	// Absent mustSupport preserves base.
	base2 := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.ms", Path: "R.ms", Min: intptr(0), Max: json.RawMessage(`"1"`), MustSupport: boolptr(true)},
	}
	diff2 := []RawElement{{ID: "R.ms", Path: "R.ms"}}
	merged2 := MergeDifferential(base2, diff2)
	for i := range merged2 {
		if merged2[i].ID == "R.ms" {
			got = &merged2[i]
		}
	}
	if got == nil || got.MustSupport == nil || !*got.MustSupport {
		t.Errorf("R.ms mustSupport = %+v, want true (preserved)", got.MustSupport)
	}
}

// TestOverlayBaseMax verifies that a differential base block replaces the base.
func TestOverlayBaseMax(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.b", Path: "R.b", Min: intptr(0), Max: json.RawMessage(`"1"`), Base: &RawBase{Min: intptr(0), Max: json.RawMessage(`"1"`)}},
	}
	diff := []RawElement{
		{ID: "R.b", Path: "R.b", Base: &RawBase{Min: intptr(0), Max: json.RawMessage(`"*"`)}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.b" {
			got = &merged[i]
		}
	}
	if got == nil || got.Base == nil || string(got.Base.Max) != `"*"` {
		t.Errorf("R.b base = %+v, want max \"*\"", got.Base)
	}
}

// TestOverlayConstraints verifies that a differential constraint list replaces
// the base list.
func TestOverlayConstraints(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.c", Path: "R.c", Min: intptr(0), Max: json.RawMessage(`"1"`),
			Constraint: []RawConstraint{{Key: "base-1"}}},
	}
	diff := []RawElement{
		{ID: "R.c", Path: "R.c", Constraint: []RawConstraint{{Key: "profile-1"}, {Key: "profile-2"}}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.c" {
			got = &merged[i]
		}
	}
	if got == nil || len(got.Constraint) != 2 || got.Constraint[0].Key != "profile-1" {
		t.Errorf("R.c constraint = %+v, want [profile-1 profile-2]", got.Constraint)
	}
}

// TestOverlayFixed verifies that a differential fixed value replaces the base.
func TestOverlayFixed(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.f", Path: "R.f", Min: intptr(0), Max: json.RawMessage(`"1"`), Fixed: "http://base"},
	}
	diff := []RawElement{
		{ID: "R.f", Path: "R.f", Fixed: "http://profile"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.f" {
			got = &merged[i]
		}
	}
	if got == nil || got.Fixed != "http://profile" {
		t.Errorf("R.f fixed = %#v, want http://profile", got.Fixed)
	}
}

// TestOverlayPattern verifies that a differential pattern replaces the base.
func TestOverlayPattern(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.p", Path: "R.p", Min: intptr(0), Max: json.RawMessage(`"1"`), Pattern: "base"},
	}
	diff := []RawElement{
		{ID: "R.p", Path: "R.p", Pattern: "profile"},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.p" {
			got = &merged[i]
		}
	}
	if got == nil || got.Pattern != "profile" {
		t.Errorf("R.p pattern = %#v, want profile", got.Pattern)
	}
}

// TestOverlayExamples verifies that a differential example list replaces the
// base list.
func TestOverlayExamples(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.e", Path: "R.e", Min: intptr(0), Max: json.RawMessage(`"1"`), Examples: []any{"base"}},
	}
	diff := []RawElement{
		{ID: "R.e", Path: "R.e", Examples: []any{"profile"}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.e" {
			got = &merged[i]
		}
	}
	if got == nil || len(got.Examples) != 1 || got.Examples[0] != "profile" {
		t.Errorf("R.e examples = %#v, want [profile]", got.Examples)
	}
}

// TestOverlayProfileTargetProfile verifies that differential profile and
// targetProfile lists replace the base lists.
func TestOverlayProfileTargetProfile(t *testing.T) {
	base := []RawElement{
		{ID: "R", Path: "R"},
		{ID: "R.p", Path: "R.p", Min: intptr(0), Max: json.RawMessage(`"1"`),
			Profile: []string{"http://base"}, TargetProfile: []string{"http://base-t"}},
	}
	diff := []RawElement{
		{ID: "R.p", Path: "R.p", Profile: []string{"http://profile"}, TargetProfile: []string{"http://profile-t"}},
	}
	merged := MergeDifferential(base, diff)
	var got *RawElement
	for i := range merged {
		if merged[i].ID == "R.p" {
			got = &merged[i]
		}
	}
	if got == nil || len(got.Profile) != 1 || got.Profile[0] != "http://profile" {
		t.Errorf("R.p profile = %v, want [http://profile]", got.Profile)
	}
	if got == nil || len(got.TargetProfile) != 1 || got.TargetProfile[0] != "http://profile-t" {
		t.Errorf("R.p targetProfile = %v, want [http://profile-t]", got.TargetProfile)
	}
}
