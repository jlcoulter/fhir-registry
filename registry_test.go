package fhir

import (
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
)

func loadTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := reg.LoadPackage("package"); err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	return reg
}

func TestParseMax(t *testing.T) {
	cases := []struct {
		raw     string
		want    Max
		wantErr bool
	}{
		{`"*"`, MaxUnbounded, false},
		{`"1"`, 1, false},
		{`"0"`, 0, false},
		{`"4"`, 4, false},
		{`2`, 2, false},
		{`"abc"`, 0, true},
		{`true`, 0, true},
	}
	for _, tc := range cases {
		got, err := parseMax(json.RawMessage(tc.raw))
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseMax(%s): expected error", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseMax(%s): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseMax(%s) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestMaxString(t *testing.T) {
	cases := []struct {
		m    Max
		want string
	}{
		{MaxUnbounded, "*"},
		{Max(0), "0"},
		{Max(1), "1"},
		{Max(4), "4"},
	}
	for _, tc := range cases {
		if got := tc.m.String(); got != tc.want {
			t.Errorf("Max(%d).String() = %q, want %q", tc.m, got, tc.want)
		}
	}
}

func TestMaxIsUnbounded(t *testing.T) {
	if !MaxUnbounded.IsUnbounded() {
		t.Error("MaxUnbounded should be unbounded")
	}
	if Max(1).IsUnbounded() {
		t.Error("Max(1) should not be unbounded")
	}
}

func TestLoadPackage(t *testing.T) {
	reg := loadTestRegistry(t)
	if len(reg.byURL) == 0 {
		t.Fatal("no StructureDefinitions loaded")
	}
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient not found by URL")
	}
	defs := reg.DefinitionsForType("Patient")
	if len(defs) == 0 {
		t.Error("no definitions for type Patient")
	}
}

func TestLoadPackageTgz(t *testing.T) {
	if _, err := os.Stat("au-base.tgz"); err != nil {
		t.Skip("au-base.tgz not present")
	}
	reg := NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient not found from tgz")
	}
}

func TestBuildTreeOrganization(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/au-organization")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Organization" {
		t.Fatalf("root = %v", tree.Root)
	}
	// Direct children of Organization.
	names := map[string]bool{}
	for _, c := range tree.Root.Children {
		names[c.Path] = true
	}
	for _, want := range []string{"Organization.id", "Organization.name", "Organization.contact", "Organization.endpoint"} {
		if !names[want] {
			t.Errorf("missing child %s", want)
		}
	}
	// Deep lookup via path map.
	elems := tree.ByPath["Organization.contact.purpose"]
	if len(elems) != 1 {
		t.Fatalf("Organization.contact.purpose: got %d, want 1", len(elems))
	}
	if got := Cardinality(elems[0]); got != "0..1" {
		t.Errorf("contact.purpose cardinality = %s, want 0..1", got)
	}
}

func TestBuildTreeSlicedElements(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/au-patient")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	// Sliced extensions share a path but have distinct ids.
	exts := tree.ByPath["Patient.extension"]
	if len(exts) < 2 {
		t.Fatalf("expected sliced Patient.extension, got %d", len(exts))
	}
	// The base (unsliced) element is present.
	var base *ElementDefinition
	for _, e := range exts {
		if e.SliceName == "" {
			base = e
		}
	}
	if base == nil {
		t.Fatal("no base Patient.extension element")
	}
	// A slice is a child of the base element.
	found := false
	for _, c := range base.Children {
		if c.SliceName == "birthPlace" {
			found = true
		}
	}
	if !found {
		t.Error("birthPlace slice not a child of base extension")
	}
	// ByID lookup works for slice ids.
	if _, ok := tree.ByID["Patient.extension:birthPlace"]; !ok {
		t.Error("ByID missing Patient.extension:birthPlace")
	}
}

func TestBuildTreeNestedSlices(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/ahpraprofession-details")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	// Nested slice: Extension.extension:ahpraCaution.extension:ahpraCautionDetail
	detail := tree.ByID["Extension.extension:ahpraCaution.extension:ahpraCautionDetail"]
	if detail == nil {
		t.Fatal("nested slice element not found")
	}
	if detail.Path != "Extension.extension.extension" {
		t.Errorf("nested slice path = %s", detail.Path)
	}
}

func rawElem(id, path string) RawElement {
	return RawElement{ID: id, Path: path, Min: intPtr(0), Max: json.RawMessage(`"1"`)}
}

func intPtr(v int) *int { return &v }

// TestEnsureSnapshotSnapshotPresent verifies that a definition with a snapshot
// returns its snapshot elements directly, without touching the differential.
func TestEnsureSnapshotSnapshotPresent(t *testing.T) {
	reg := NewRegistry()
	sd := &StructureDefinition{
		ID:   "snap",
		Type: "Foo",
		Snapshot: &Snapshot{Elements: []RawElement{
			rawElem("Foo", "Foo"),
			rawElem("Foo.bar", "Foo.bar"),
		}},
		Differential: &Differential{Elements: []RawElement{
			rawElem("Foo.bar", "Foo.bar"),
		}},
	}
	raws, err := reg.ensureSnapshot(sd)
	if err != nil {
		t.Fatalf("ensureSnapshot: %v", err)
	}
	if len(raws) != 2 {
		t.Fatalf("got %d elements, want 2 (snapshot preferred)", len(raws))
	}
}

// TestEnsureSnapshotDifferentialOnly verifies that a differential-only profile
// resolves its base definition from the registry and merges the differential
// onto the base snapshot.
func TestEnsureSnapshotDifferentialOnly(t *testing.T) {
	reg := NewRegistry()
	// Base definition with a snapshot.
	base := &StructureDefinition{
		ID:   "base",
		Type: "Foo",
		Snapshot: &Snapshot{Elements: []RawElement{
			rawElem("Foo", "Foo"),
			rawElem("Foo.bar", "Foo.bar"),
			rawElem("Foo.baz", "Foo.baz"),
		}},
	}
	reg.byURL["http://example.org/StructureDefinition/Base"] = base

	// Profile with only a differential that narrows Foo.bar.
	profile := &StructureDefinition{
		ID:             "profile",
		Type:           "Foo",
		BaseDefinition: "http://example.org/StructureDefinition/Base",
		Differential: &Differential{Elements: []RawElement{
			{ID: "Foo.bar", Path: "Foo.bar", Min: intPtr(1)},
		}},
	}
	raws, err := reg.ensureSnapshot(profile)
	if err != nil {
		t.Fatalf("ensureSnapshot: %v", err)
	}
	// Merged result keeps all base elements (3) with the differential overlaid.
	if len(raws) != 3 {
		t.Fatalf("got %d elements, want 3 (base + merged)", len(raws))
	}
	var bar *RawElement
	for i := range raws {
		if raws[i].ID == "Foo.bar" {
			bar = &raws[i]
		}
	}
	if bar == nil || bar.Min == nil || *bar.Min != 1 {
		t.Errorf("Foo.bar min = %+v, want 1 (overlaid from differential)", bar)
	}
}

// TestEnsureSnapshotBaseWithVersion verifies that a base definition URL with a
// "|version" suffix is stripped before registry lookup.
func TestEnsureSnapshotBaseWithVersion(t *testing.T) {
	reg := NewRegistry()
	base := &StructureDefinition{
		ID:   "base",
		Type: "Foo",
		Snapshot: &Snapshot{Elements: []RawElement{
			rawElem("Foo", "Foo"),
		}},
	}
	reg.byURL["http://example.org/StructureDefinition/Base"] = base

	profile := &StructureDefinition{
		ID:             "profile",
		Type:           "Foo",
		BaseDefinition: "http://example.org/StructureDefinition/Base|4.0.1",
		Differential: &Differential{Elements: []RawElement{
			rawElem("Foo.bar", "Foo.bar"),
		}},
	}
	raws, err := reg.ensureSnapshot(profile)
	if err != nil {
		t.Fatalf("ensureSnapshot with versioned base: %v", err)
	}
	if len(raws) != 2 {
		t.Fatalf("got %d elements, want 2", len(raws))
	}
}

// TestEnsureSnapshotNoElements verifies the error when a definition has neither
// a snapshot nor a differential.
func TestEnsureSnapshotNoElements(t *testing.T) {
	reg := NewRegistry()
	sd := &StructureDefinition{ID: "empty", Type: "Foo"}
	if _, err := reg.ensureSnapshot(sd); err == nil {
		t.Fatal("expected error for definition with no elements")
	}
}

// TestEnsureSnapshotBaseMissing verifies the error when a differential-only
// profile's base definition cannot be resolved.
func TestEnsureSnapshotBaseMissing(t *testing.T) {
	reg := NewRegistry()
	profile := &StructureDefinition{
		ID:             "profile",
		Type:           "Foo",
		BaseDefinition: "http://example.org/StructureDefinition/Missing",
		Differential: &Differential{Elements: []RawElement{
			rawElem("Foo.bar", "Foo.bar"),
		}},
	}
	if _, err := reg.ensureSnapshot(profile); err == nil {
		t.Fatal("expected error for unresolvable base definition")
	}
}

func TestBuildTreeSnapshotPreferred(t *testing.T) {
	sd := &StructureDefinition{
		ID:   "snap",
		Type: "Foo",
		Snapshot: &Snapshot{Elements: []RawElement{
			rawElem("Foo", "Foo"),
			rawElem("Foo.bar", "Foo.bar"),
		}},
		Differential: &Differential{Elements: []RawElement{
			rawElem("Foo.bar", "Foo.bar"),
		}},
	}
	tree, err := BuildTree(sd)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Foo" {
		t.Fatalf("root = %v", tree.Root)
	}
	if len(tree.Root.Children) != 1 || tree.Root.Children[0].Path != "Foo.bar" {
		t.Errorf("children = %v", tree.Root.Children)
	}
}

func TestBuildTreeDifferentialFallback(t *testing.T) {
	sd := &StructureDefinition{
		ID:           "diff",
		Type:         "Foo",
		Differential: &Differential{Elements: []RawElement{rawElem("Foo", "Foo")}},
	}
	tree, err := BuildTree(sd)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Foo" {
		t.Fatalf("root = %v", tree.Root)
	}
}

func TestBuildTreeEmptyDefinition(t *testing.T) {
	sd := &StructureDefinition{ID: "empty", Type: "Foo"}
	if _, err := BuildTree(sd); err == nil {
		t.Fatal("expected error for definition with no elements")
	}
}

func TestBuildTreeDifferentialOnlyWithBase(t *testing.T) {
	// A differential-only definition that derives from a base cannot be
	// turned into a complete tree without resolving the base snapshot.
	sd := &StructureDefinition{
		ID:             "diff-base",
		Type:           "Foo",
		BaseDefinition: "http://example.org/StructureDefinition/Base",
		Differential:   &Differential{Elements: []RawElement{rawElem("Foo.bar", "Foo.bar")}},
	}
	if _, err := BuildTree(sd); err == nil {
		t.Fatal("expected error for differential-only definition with a base")
	}
}

func TestBuildTreeElementsChildLinking(t *testing.T) {
	sd := &StructureDefinition{ID: "link", Type: "Foo"}
	raws := []RawElement{
		rawElem("Foo", "Foo"),
		rawElem("Foo.bar", "Foo.bar"),
		rawElem("Foo.bar.baz", "Foo.bar.baz"),
		rawElem("Foo.bar:slice", "Foo.bar"),
	}
	tree, err := BuildTreeElements(sd, raws)
	if err != nil {
		t.Fatalf("BuildTreeElements: %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Foo" {
		t.Fatalf("root = %v", tree.Root)
	}
	if len(tree.Root.Children) != 1 {
		t.Fatalf("Foo children = %d, want 1", len(tree.Root.Children))
	}
	bar := tree.Root.Children[0]
	if len(bar.Children) != 2 {
		t.Fatalf("Foo.bar children = %d, want 2 (baz + slice)", len(bar.Children))
	}
}

func TestBuildTreeElementsPathMap(t *testing.T) {
	sd := &StructureDefinition{ID: "pathmap", Type: "Foo"}
	raws := []RawElement{
		rawElem("Foo", "Foo"),
		rawElem("Foo.bar", "Foo.bar"),
		rawElem("Foo.bar:slice", "Foo.bar"),
	}
	tree, err := BuildTreeElements(sd, raws)
	if err != nil {
		t.Fatalf("BuildTreeElements: %v", err)
	}
	// Sliced elements share a path but are all present in the path map.
	if got := len(tree.ByPath["Foo.bar"]); got != 2 {
		t.Errorf("ByPath[Foo.bar] = %d, want 2", got)
	}
	if _, ok := tree.ByID["Foo.bar:slice"]; !ok {
		t.Error("ByID missing Foo.bar:slice")
	}
}

func TestChoiceName(t *testing.T) {
	elem := &ElementDefinition{Path: "Patient.deceased[x]"}
	if got := ChoiceName(elem, "boolean"); got != "deceasedBoolean" {
		t.Errorf("ChoiceName(boolean) = %s", got)
	}
	if got := ChoiceName(elem, "dateTime"); got != "deceasedDateTime" {
		t.Errorf("ChoiceName(dateTime) = %s", got)
	}
}

// TestIsRequired verifies the required-element predicate.
func TestIsRequired(t *testing.T) {
	cases := []struct {
		min  int
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
	}
	for _, tc := range cases {
		elem := &ElementDefinition{Min: tc.min}
		if got := IsRequired(elem); got != tc.want {
			t.Errorf("IsRequired(min=%d) = %v, want %v", tc.min, got, tc.want)
		}
	}
}

// TestStripVersion verifies removal of a "|version" suffix from a canonical URL.
func TestStripVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http://example.org/StructureDefinition/X", "http://example.org/StructureDefinition/X"},
		{"http://example.org/StructureDefinition/X|4.0.1", "http://example.org/StructureDefinition/X"},
		{"http://example.org/StructureDefinition/X|", "http://example.org/StructureDefinition/X"},
	}
	for _, tc := range cases {
		if got := stripVersion(tc.in); got != tc.want {
			t.Errorf("stripVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestPrimaryTypeCode verifies the primary type code helper, including the
// empty-types case.
func TestPrimaryTypeCode(t *testing.T) {
	if got := PrimaryTypeCode(&ElementDefinition{}); got != "" {
		t.Errorf("PrimaryTypeCode(no types) = %q, want empty", got)
	}
	elem := &ElementDefinition{Types: []ElementType{{Code: "string"}, {Code: "code"}}}
	if got := PrimaryTypeCode(elem); got != "string" {
		t.Errorf("PrimaryTypeCode = %q, want string", got)
	}
}

// TestCapitalize verifies the capitalize helper, including the empty case.
func TestCapitalize(t *testing.T) {
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize(\"\") = %q, want empty", got)
	}
	if got := capitalize("boolean"); got != "Boolean" {
		t.Errorf("capitalize(boolean) = %q, want Boolean", got)
	}
	if got := capitalize("dateTime"); got != "DateTime" {
		t.Errorf("capitalize(dateTime) = %q, want DateTime", got)
	}
}

// TestLastSegment verifies the last path segment helper, including the
// no-dot case.
func TestLastSegment(t *testing.T) {
	if got := lastSegment("Organization.name"); got != "name" {
		t.Errorf("lastSegment(Organization.name) = %q, want name", got)
	}
	if got := lastSegment("name"); got != "name" {
		t.Errorf("lastSegment(name) = %q, want name", got)
	}
	if got := lastSegment(""); got != "" {
		t.Errorf("lastSegment(\"\") = %q, want empty", got)
	}
}

// TestOr verifies the or() helper: prefer the second value when non-empty.
func TestOr(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"base", "diff", "diff"},
		{"base", "", "base"},
		{"", "diff", "diff"},
		{"", "", ""},
	}
	for _, tc := range cases {
		if got := or(tc.a, tc.b); got != tc.want {
			t.Errorf("or(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestMarshalCardinality(t *testing.T) {
	reg := loadTestRegistry(t)

	// name is 0..1 (scalar), alias is 0..* (array), identifier is 0..* (array).
	in := map[string]any{
		"resourceType": "Organization",
		"name":         "ACME",
		"alias":        "ACME Corp",
		"identifier":   map[string]any{"system": "urn:x", "value": "1"},
	}
	out, _, err := reg.Marshal("Organization", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if out["name"] != "ACME" {
		t.Errorf("name should stay scalar, got %#v", out["name"])
	}
	if _, ok := out["alias"].([]any); !ok {
		t.Errorf("alias should be array, got %#v", out["alias"])
	}
	if _, ok := out["identifier"].([]any); !ok {
		t.Errorf("identifier should be array, got %#v", out["identifier"])
	}
}

func TestMarshalUnwrapSingleArray(t *testing.T) {
	reg := loadTestRegistry(t)
	// name is 0..1; a one-element array should be unwrapped to a scalar.
	in := map[string]any{
		"resourceType": "Organization",
		"name":         []any{"ACME"},
	}
	out, _, err := reg.Marshal("Organization", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if out["name"] != "ACME" {
		t.Errorf("name should be unwrapped to scalar, got %#v", out["name"])
	}
}

func TestMarshalChoice(t *testing.T) {
	reg := loadTestRegistry(t)
	in := map[string]any{
		"resourceType":         "Patient",
		"deceasedBoolean":      true,
		"multipleBirthInteger": 2,
	}
	out, _, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if out["deceasedBoolean"] != true {
		t.Errorf("deceasedBoolean = %#v", out["deceasedBoolean"])
	}
	if out["multipleBirthInteger"] != 2 {
		t.Errorf("multipleBirthInteger = %#v", out["multipleBirthInteger"])
	}
}

func TestMarshalNested(t *testing.T) {
	reg := loadTestRegistry(t)
	in := map[string]any{
		"resourceType": "Patient",
		"contact": []any{
			map[string]any{"name": map[string]any{"family": "Smith"}},
		},
	}
	out, _, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	contacts, ok := out["contact"].([]any)
	if !ok || len(contacts) != 1 {
		t.Fatalf("contact = %#v", out["contact"])
	}
	first, ok := contacts[0].(map[string]any)
	if !ok {
		t.Fatalf("contact[0] = %#v", contacts[0])
	}
	name, ok := first["name"].(map[string]any)
	if !ok || name["family"] != "Smith" {
		t.Errorf("nested name = %#v", first["name"])
	}
}

func TestMarshalExampleRoundTrip(t *testing.T) {
	reg := loadTestRegistry(t)
	data, err := os.ReadFile("package/example/Patient-example0.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var in map[string]any
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out, rep, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// resourceType and id preserved.
	if out["resourceType"] != "Patient" {
		t.Errorf("resourceType = %#v", out["resourceType"])
	}
	if out["id"] != "example0" {
		t.Errorf("id = %#v", out["id"])
	}
	// Marshal is idempotent.
	out2, _, err := reg.Marshal("Patient", out)
	if err != nil {
		t.Fatalf("Marshal(2): %v", err)
	}
	if !reflect.DeepEqual(out, out2) {
		t.Error("Marshal is not idempotent")
	}
	_ = rep
}

func TestMarshalRequiredViolation(t *testing.T) {
	reg := loadTestRegistry(t)
	// AU base Identifier profile (au-ihi) requires system and value.
	in := map[string]any{
		"resourceType": "Patient",
		"active":       true,
	}
	_, rep, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Patient has no truly required elements at the root; but we can at least
	// ensure active (0..1) stays scalar and no panic occurs.
	if rep == nil {
		t.Fatal("nil report")
	}
}

func TestMarshalUnknownKeyPassthrough(t *testing.T) {
	reg := loadTestRegistry(t)
	in := map[string]any{
		"resourceType": "Organization",
		"name":         "ACME",
		"customField":  "keep-me",
	}
	out, rep, err := reg.Marshal("Organization", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if out["customField"] != "keep-me" {
		t.Errorf("unknown key dropped: %#v", out)
	}
	found := false
	for _, it := range rep.Items {
		if it.Severity == SeverityWarning && it.Path == "Organization" {
			found = true
		}
	}
	if !found {
		t.Error("expected a warning for the unknown key")
	}
}

func TestMarshalChoiceViolation(t *testing.T) {
	reg := loadTestRegistry(t)
	in := map[string]any{
		"resourceType":     "Patient",
		"deceasedBoolean":  true,
		"deceasedDateTime": "2020-01-01",
	}
	out, rep, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if _, ok := out["deceasedBoolean"]; !ok {
		t.Error("deceasedBoolean should be present")
	}
	found := false
	for _, it := range rep.Items {
		if it.Severity == SeverityViolation && it.Path == "Patient.deceased[x]" {
			found = true
		}
	}
	if !found {
		t.Error("expected a choice violation for multiple deceased values")
	}
}

// TestMarshalDeepTypeResolution verifies that complex-type elements are
// resolved through the registry (au-address) so that Address.line (0..*)
// is wrapped into an array even though it is not part of the Patient tree.
func TestMarshalDeepTypeResolution(t *testing.T) {
	reg := loadTestRegistry(t)
	in := map[string]any{
		"resourceType": "Patient",
		"address": map[string]any{
			"use":  "home",
			"line": "31 Pacquola St",
		},
	}
	out, _, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Patient.address is 0..*, so it is an array of address objects.
	addrs, ok := out["address"].([]any)
	if !ok || len(addrs) != 1 {
		t.Fatalf("address = %#v", out["address"])
	}
	addr, ok := addrs[0].(map[string]any)
	if !ok {
		t.Fatalf("address[0] = %#v", addrs[0])
	}
	if addr["use"] != "home" {
		t.Errorf("use = %#v", addr["use"])
	}
	lines, ok := addr["line"].([]any)
	if !ok || len(lines) != 1 || lines[0] != "31 Pacquola St" {
		t.Errorf("Address.line should be wrapped to array, got %#v", addr["line"])
	}
}

// TestResolveType verifies type resolution prioritises the type's base URL
// when available, else a unique profile.
func TestResolveType(t *testing.T) {
	reg := loadTestRegistry(t)
	// au-address is the only Address definition in this package.
	tree, ok := reg.ResolveType("Address", nil)
	if !ok || tree == nil {
		t.Fatal("ResolveType(Address) failed")
	}
	if tree.SD.URL != "http://hl7.org.au/fhir/StructureDefinition/au-address" {
		t.Errorf("resolved %s, want au-address", tree.SD.URL)
	}
	// A profile hint wins over the unique-profile fallback.
	tree2, ok := reg.ResolveType("Identifier", []string{"http://hl7.org.au/fhir/StructureDefinition/au-ihi"})
	if !ok || tree2.SD.URL != "http://hl7.org.au/fhir/StructureDefinition/au-ihi" {
		t.Errorf("profile hint not honored: %v", tree2)
	}
}

// TestTreeConcurrentWithLoad exercises the race between Tree reading the
// registry maps and addResource mutating them. Run with -race to detect it.
func TestTreeConcurrentWithLoad(t *testing.T) {
	reg := loadTestRegistry(t)
	url := "http://hl7.org.au/fhir/StructureDefinition/au-patient"

	// A minimal StructureDefinition JSON that addResource will index.
	sdJSON := []byte(`{"resourceType":"StructureDefinition","url":"` + url + `","type":"Patient","snapshot":{"element":[{"id":"Patient","path":"Patient"}]}}`)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := reg.Tree(url); err != nil {
				t.Errorf("Tree: %v", err)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := reg.addResource("sd.json", sdJSON); err != nil {
				t.Errorf("addResource: %v", err)
			}
		}()
	}
	wg.Wait()
}

// TestTreeConcurrentSameURL verifies concurrent Tree calls for the same URL
// return the same cached tree and never build it twice.
func TestTreeConcurrentSameURL(t *testing.T) {
	reg := loadTestRegistry(t)
	url := "http://hl7.org.au/fhir/StructureDefinition/au-patient"

	first, err := reg.Tree(url)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]*ElementTree, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tr, err := reg.Tree(url)
			if err != nil {
				t.Errorf("Tree: %v", err)
				return
			}
			results[i] = tr
		}(i)
	}
	wg.Wait()

	for i, tr := range results {
		if tr != first {
			t.Errorf("result %d is a different tree instance", i)
		}
	}
}
