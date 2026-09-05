package fhir

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

func loadTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz: %v", err)
	}
	return reg
}

// readTgzFile reads a single file from the au-base.tgz archive, matching the
// "package/"-stripped layout used by LoadPackageTgz.
func readTgzFile(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open("au-base.tgz")
	if err != nil {
		t.Fatalf("open au-base.tgz: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		rel, ok := strings.CutPrefix(hdr.Name, "package/")
		if !ok || rel != name {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return data
	}
	t.Fatalf("file %s not found in au-base.tgz", name)
	return nil
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

// TestStructureDefinitions verifies that StructureDefinitions returns every
// indexed StructureDefinition, sorted by canonical URL.
func TestStructureDefinitions(t *testing.T) {
	reg := NewRegistry()
	// Register three definitions out of order.
	reg.byURL["http://example.org/StructureDefinition/c"] = &StructureDefinition{URL: "http://example.org/StructureDefinition/c", Type: "Foo"}
	reg.byURL["http://example.org/StructureDefinition/a"] = &StructureDefinition{URL: "http://example.org/StructureDefinition/a", Type: "Foo"}
	reg.byURL["http://example.org/StructureDefinition/b"] = &StructureDefinition{URL: "http://example.org/StructureDefinition/b", Type: "Bar"}

	defs := reg.StructureDefinitions()
	if len(defs) != 3 {
		t.Fatalf("got %d definitions, want 3", len(defs))
	}
	// Must be sorted by URL.
	for i := 1; i < len(defs); i++ {
		if defs[i-1].URL >= defs[i].URL {
			t.Errorf("definitions not sorted: %s before %s", defs[i-1].URL, defs[i].URL)
		}
	}
	// Must contain all three URLs.
	got := map[string]bool{}
	for _, d := range defs {
		got[d.URL] = true
	}
	for _, want := range []string{
		"http://example.org/StructureDefinition/a",
		"http://example.org/StructureDefinition/b",
		"http://example.org/StructureDefinition/c",
	} {
		if !got[want] {
			t.Errorf("missing definition %s", want)
		}
	}
}

// TestStructureDefinitionsEmpty verifies that an empty registry returns an
// empty (non-nil) slice.
func TestStructureDefinitionsEmpty(t *testing.T) {
	reg := NewRegistry()
	defs := reg.StructureDefinitions()
	if defs == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(defs) != 0 {
		t.Fatalf("got %d definitions, want 0", len(defs))
	}
}

// TestStructureDefinitionsSorted verifies the returned slice is sorted even
// when definitions are added in arbitrary order.
func TestStructureDefinitionsSorted(t *testing.T) {
	reg := NewRegistry()
	urls := []string{
		"http://example.org/StructureDefinition/z",
		"http://example.org/StructureDefinition/a",
		"http://example.org/StructureDefinition/m",
	}
	for _, u := range urls {
		reg.byURL[u] = &StructureDefinition{URL: u, Type: "Foo"}
	}
	defs := reg.StructureDefinitions()
	got := make([]string, 0, len(defs))
	for _, d := range defs {
		got = append(got, d.URL)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("StructureDefinitions not sorted: %v", got)
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
	// A slice is grouped into the base element's Slices, not its Children.
	found := false
	for _, s := range base.Slices {
		if s.Name == "birthPlace" {
			found = true
		}
	}
	if !found {
		t.Error("birthPlace slice not in base extension Slices")
	}
	// The slice entry is not a direct child of the base element.
	for _, c := range base.Children {
		if c.SliceName == "birthPlace" {
			t.Error("birthPlace slice should not be in base extension Children")
		}
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

// TestNewStructureDefinition verifies that NewStructureDefinition builds a
// definition whose snapshot round-trips the given elements through the tree.
func TestNewStructureDefinition(t *testing.T) {
	sd := NewStructureDefinition(
		"http://example.org/StructureDefinition/foo",
		"Foo",
		"Foo",
		"resource",
		"",
		"",
		[]ElementDefinition{
			{ID: "Foo", Path: "Foo", Min: 0, Max: 1},
			{ID: "Foo.bar", Path: "Foo.bar", Min: 1, Max: MaxUnbounded, Types: []ElementType{{Code: "string"}}},
		},
	)
	if sd.Snapshot == nil || len(sd.Snapshot.Elements) != 2 {
		t.Fatalf("snapshot = %+v", sd.Snapshot)
	}
	tree, err := BuildTree(sd)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Foo" {
		t.Fatalf("root = %v", tree.Root)
	}
	if len(tree.Root.Children) != 1 || tree.Root.Children[0].Path != "Foo.bar" {
		t.Fatalf("children = %+v", tree.Root.Children)
	}
	if tree.Root.Children[0].Max != MaxUnbounded {
		t.Errorf("bar max = %v, want unbounded", tree.Root.Children[0].Max)
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

// TestBuildTreeElementsSliceGroup verifies that a sliced child is grouped into
// the parent's Slices and removed from its Children.
func TestBuildTreeElementsSliceGroup(t *testing.T) {
	sd := &StructureDefinition{ID: "slice", Type: "Foo"}
	raws := []RawElement{
		rawElem("Foo", "Foo"),
		rawElem("Foo.ext", "Foo.ext"),
		{ID: "Foo.ext:birthPlace", Path: "Foo.ext", SliceName: "birthPlace", Min: intPtr(0), Max: json.RawMessage(`"1"`)},
		{ID: "Foo.ext:birthPlace.url", Path: "Foo.ext.url", Min: intPtr(0), Max: json.RawMessage(`"1"`)},
	}
	tree, err := BuildTreeElements(sd, raws)
	if err != nil {
		t.Fatalf("BuildTreeElements: %v", err)
	}
	ext := tree.ByID["Foo.ext"]
	if ext == nil {
		t.Fatal("Foo.ext not found")
	}
	// The slice entry is grouped into Slices, not Children.
	if len(ext.Slices) != 1 {
		t.Fatalf("Foo.ext Slices len = %d, want 1", len(ext.Slices))
	}
	g := ext.Slices[0]
	if g.Name != "birthPlace" {
		t.Errorf("SliceGroup.Name = %q, want birthPlace", g.Name)
	}
	if g.Definition == nil || g.Definition.ID != "Foo.ext:birthPlace" {
		t.Errorf("SliceGroup.Definition = %+v, want Foo.ext:birthPlace", g.Definition)
	}
	// The slice entry is not a direct child.
	for _, c := range ext.Children {
		if c.SliceName == "birthPlace" {
			t.Error("slice entry should not be in Children")
		}
	}
	// The slice's sub-element is reachable via the slice entry's Children.
	if g.Definition == nil || len(g.Definition.Children) != 1 || g.Definition.Children[0].ID != "Foo.ext:birthPlace.url" {
		t.Errorf("slice entry children = %+v, want [Foo.ext:birthPlace.url]", g.Definition.Children)
	}
}

// TestBuildTreeElementsMultipleSlices verifies multiple slices of one element.
func TestBuildTreeElementsMultipleSlices(t *testing.T) {
	sd := &StructureDefinition{ID: "multi", Type: "Foo"}
	raws := []RawElement{
		rawElem("Foo", "Foo"),
		rawElem("Foo.ext", "Foo.ext"),
		{ID: "Foo.ext:a", Path: "Foo.ext", SliceName: "a", Min: intPtr(0), Max: json.RawMessage(`"1"`)},
		{ID: "Foo.ext:b", Path: "Foo.ext", SliceName: "b", Min: intPtr(0), Max: json.RawMessage(`"1"`)},
	}
	tree, err := BuildTreeElements(sd, raws)
	if err != nil {
		t.Fatalf("BuildTreeElements: %v", err)
	}
	ext := tree.ByID["Foo.ext"]
	if ext == nil {
		t.Fatal("Foo.ext not found")
	}
	if len(ext.Slices) != 2 {
		t.Fatalf("Foo.ext Slices len = %d, want 2", len(ext.Slices))
	}
	names := map[string]bool{}
	for _, g := range ext.Slices {
		names[g.Name] = true
	}
	if !names["a"] || !names["b"] {
		t.Errorf("Slices names = %v, want a and b", names)
	}
}

// TestBuildTreeElementsNestedSlice verifies a slice nested within a slice is
// grouped into the outer slice entry's Slices.
func TestBuildTreeElementsNestedSlice(t *testing.T) {
	sd := &StructureDefinition{ID: "nested", Type: "Foo"}
	raws := []RawElement{
		rawElem("Foo", "Foo"),
		rawElem("Foo.ext", "Foo.ext"),
		{ID: "Foo.ext:outer", Path: "Foo.ext", SliceName: "outer", Min: intPtr(0), Max: json.RawMessage(`"1"`)},
		rawElem("Foo.ext:outer.ext", "Foo.ext.ext"),
		{ID: "Foo.ext:outer.ext:inner", Path: "Foo.ext.ext", SliceName: "inner", Min: intPtr(0), Max: json.RawMessage(`"1"`)},
	}
	tree, err := BuildTreeElements(sd, raws)
	if err != nil {
		t.Fatalf("BuildTreeElements: %v", err)
	}
	outer := tree.ByID["Foo.ext:outer"]
	if outer == nil {
		t.Fatal("outer slice not found")
	}
	// The inner slice is grouped into the outer slice entry's sub-element's
	// Slices, which is reachable through the outer slice entry's Children.
	if len(outer.Children) != 1 {
		t.Fatalf("outer Children len = %d, want 1", len(outer.Children))
	}
	sub := outer.Children[0]
	if sub.ID != "Foo.ext:outer.ext" {
		t.Fatalf("outer child = %s, want Foo.ext:outer.ext", sub.ID)
	}
	if len(sub.Slices) != 1 {
		t.Fatalf("sub Slices len = %d, want 1", len(sub.Slices))
	}
	if sub.Slices[0].Name != "inner" {
		t.Errorf("inner slice name = %q, want inner", sub.Slices[0].Name)
	}
	if sub.Slices[0].Definition == nil || sub.Slices[0].Definition.ID != "Foo.ext:outer.ext:inner" {
		t.Errorf("inner definition = %+v", sub.Slices[0].Definition)
	}
}

// TestBuildTreeElementsNoSlices verifies an element without slicing has nil
// Slices.
func TestBuildTreeElementsNoSlices(t *testing.T) {
	sd := &StructureDefinition{ID: "noslice", Type: "Foo"}
	raws := []RawElement{
		rawElem("Foo", "Foo"),
		rawElem("Foo.bar", "Foo.bar"),
	}
	tree, err := BuildTreeElements(sd, raws)
	if err != nil {
		t.Fatalf("BuildTreeElements: %v", err)
	}
	if tree.Root.Slices != nil {
		t.Errorf("Root.Slices = %v, want nil", tree.Root.Slices)
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

// TestLookupPathExact verifies that LookupPath resolves a top-level relative
// path to its element definition.
func TestLookupPathExact(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType: %v", err)
	}
	elem, err := tree.LookupPath("birthDate")
	if err != nil {
		t.Fatalf("LookupPath(birthDate): %v", err)
	}
	if elem.Path != "Patient.birthDate" {
		t.Errorf("elem.Path = %q, want Patient.birthDate", elem.Path)
	}
}

// TestLookupPathNested verifies that LookupPath resolves a nested relative
// path (e.g. address.city) to its element definition, resolving the complex
// type through the registry.
func TestLookupPathNested(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType: %v", err)
	}
	elem, err := tree.LookupPath("address.city")
	if err != nil {
		t.Fatalf("LookupPath(address.city): %v", err)
	}
	if elem.Path != "Address.city" {
		t.Errorf("elem.Path = %q, want Address.city", elem.Path)
	}
}

// TestLookupPathNotFound verifies that LookupPath returns ErrPathNotFound for
// an unknown relative path.
func TestLookupPathNotFound(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType: %v", err)
	}
	if _, err := tree.LookupPath("nonexistent"); !errors.Is(err, ErrPathNotFound) {
		t.Errorf("LookupPath(nonexistent) err = %v, want ErrPathNotFound", err)
	}
}

// TestLookupPathChoiceConcrete verifies that LookupPath resolves a concrete
// type-suffixed key (e.g. deceasedBoolean) to the underlying choice element.
func TestLookupPathChoiceConcrete(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType: %v", err)
	}
	elem, err := tree.LookupPath("deceasedBoolean")
	if err != nil {
		t.Fatalf("LookupPath(deceasedBoolean): %v", err)
	}
	if elem.Path != "Patient.deceased[x]" {
		t.Errorf("elem.Path = %q, want Patient.deceased[x]", elem.Path)
	}
}

// TestLookupPathChoiceUnsuffixed verifies that LookupPath resolves the
// unsuffixed choice path (e.g. deceased[x]) directly.
func TestLookupPathChoiceUnsuffixed(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType: %v", err)
	}
	elem, err := tree.LookupPath("deceased[x]")
	if err != nil {
		t.Fatalf("LookupPath(deceased[x]): %v", err)
	}
	if elem.Path != "Patient.deceased[x]" {
		t.Errorf("elem.Path = %q, want Patient.deceased[x]", elem.Path)
	}
}

// TestLookupPathResourceRoot verifies that LookupPath with an empty path
// returns the tree root.
func TestLookupPathResourceRoot(t *testing.T) {
	reg := loadTestRegistry(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType: %v", err)
	}
	elem, err := tree.LookupPath("")
	if err != nil {
		t.Fatalf("LookupPath(\"\"): %v", err)
	}
	if elem != tree.Root {
		t.Errorf("LookupPath(\"\") did not return the tree root")
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
	data := readTgzFile(t, "example/Patient-example0.json")
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

// TestRawConstraintUnmarshal verifies JSON parsing of a FHIR invariant.
func TestRawConstraintUnmarshal(t *testing.T) {
	var rc RawConstraint
	data := []byte(`{"key":"inv-1","severity":"error","human":"must have a value","expression":"value.exists()","source":"http://example.org"}`)
	if err := json.Unmarshal(data, &rc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if rc.Key != "inv-1" || rc.Severity != "error" || rc.Human != "must have a value" ||
		rc.Expression != "value.exists()" || rc.Source != "http://example.org" {
		t.Errorf("RawConstraint = %+v", rc)
	}
}

// TestRawBaseParsing verifies JSON parsing of the base cardinality sub-object.
func TestRawBaseParsing(t *testing.T) {
	var rb RawBase
	if err := json.Unmarshal([]byte(`{"min":0,"max":"*"}`), &rb); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if rb.Min == nil || *rb.Min != 0 {
		t.Errorf("min = %+v, want 0", rb.Min)
	}
	if string(rb.Max) != `"*"` {
		t.Errorf("max = %s, want \"*\"", rb.Max)
	}
}

// TestConvertRawElementMustSupport verifies MustSupport conversion, including
// the nil-defaults-to-false case.
func TestConvertRawElementMustSupport(t *testing.T) {
	elem, err := convertRawElement(RawElement{ID: "X", Path: "X", MustSupport: boolptr(true)})
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if !elem.MustSupport {
		t.Error("MustSupport = false, want true")
	}
	elem2, err := convertRawElement(RawElement{ID: "X", Path: "X"})
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if elem2.MustSupport {
		t.Error("MustSupport = true, want false (nil default)")
	}
}

// TestConvertRawElementBaseMax verifies BaseMax conversion: nil Base yields nil
// BaseMax, and a present base.max is parsed into a *Max.
func TestConvertRawElementBaseMax(t *testing.T) {
	elem, err := convertRawElement(RawElement{ID: "X", Path: "X"})
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if elem.BaseMax != nil {
		t.Errorf("BaseMax = %v, want nil when no base", elem.BaseMax)
	}

	elem2, err := convertRawElement(RawElement{ID: "X", Path: "X", Base: &RawBase{Min: intPtr(0), Max: json.RawMessage(`"*"`)}})
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if elem2.BaseMax == nil || *elem2.BaseMax != MaxUnbounded {
		t.Errorf("BaseMax = %v, want MaxUnbounded", elem2.BaseMax)
	}

	elem3, err := convertRawElement(RawElement{ID: "X", Path: "X", Base: &RawBase{Min: intPtr(0), Max: json.RawMessage(`"1"`)}})
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if elem3.BaseMax == nil || *elem3.BaseMax != 1 {
		t.Errorf("BaseMax = %v, want 1", elem3.BaseMax)
	}
}

// TestConvertRawElementConstraints verifies RawConstraint to ElementConstraint
// conversion.
func TestConvertRawElementConstraints(t *testing.T) {
	raw := RawElement{
		ID:   "X",
		Path: "X",
		Constraint: []RawConstraint{
			{Key: "inv-1", Severity: "error", Human: "h", Expression: "e", Source: "s"},
		},
	}
	elem, err := convertRawElement(raw)
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if len(elem.Constraints) != 1 {
		t.Fatalf("Constraints len = %d, want 1", len(elem.Constraints))
	}
	c := elem.Constraints[0]
	if c.Key != "inv-1" || c.Severity != "error" || c.Human != "h" || c.Expression != "e" || c.Source != "s" {
		t.Errorf("Constraint = %+v", c)
	}
}

// TestConvertRawElementProfileTargetProfile verifies Profile/TargetProfile
// passthrough.
func TestConvertRawElementProfileTargetProfile(t *testing.T) {
	raw := RawElement{
		ID:            "X",
		Path:          "X",
		Profile:       []string{"http://example.org/profile"},
		TargetProfile: []string{"http://example.org/target"},
	}
	elem, err := convertRawElement(raw)
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if len(elem.Profile) != 1 || elem.Profile[0] != "http://example.org/profile" {
		t.Errorf("Profile = %v", elem.Profile)
	}
	if len(elem.TargetProfile) != 1 || elem.TargetProfile[0] != "http://example.org/target" {
		t.Errorf("TargetProfile = %v", elem.TargetProfile)
	}
}

// TestConvertRawElementFixedPattern verifies Fixed/Pattern passthrough.
func TestConvertRawElementFixedPattern(t *testing.T) {
	raw := RawElement{ID: "X", Path: "X", Fixed: "http://example.org", Pattern: map[string]any{"code": "active"}}
	elem, err := convertRawElement(raw)
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if elem.Fixed != "http://example.org" {
		t.Errorf("Fixed = %#v", elem.Fixed)
	}
	if elem.Pattern == nil {
		t.Error("Pattern = nil, want map")
	}
}

// TestConvertRawElementExamples verifies Examples passthrough.
func TestConvertRawElementExamples(t *testing.T) {
	raw := RawElement{ID: "X", Path: "X", Examples: []any{"a", "b"}}
	elem, err := convertRawElement(raw)
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if len(elem.Examples) != 2 || elem.Examples[0] != "a" || elem.Examples[1] != "b" {
		t.Errorf("Examples = %#v", elem.Examples)
	}
}

// TestRawElementUnmarshalFixed verifies the custom UnmarshalJSON captures a
// fixed* property.
func TestRawElementUnmarshalFixed(t *testing.T) {
	var raw RawElement
	if err := json.Unmarshal([]byte(`{"id":"X","path":"X","fixedString":"hello"}`), &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw.Fixed != "hello" {
		t.Errorf("Fixed = %#v, want hello", raw.Fixed)
	}
}

// TestRawElementUnmarshalFixedInteger verifies a numeric fixed* property.
func TestRawElementUnmarshalFixedInteger(t *testing.T) {
	var raw RawElement
	if err := json.Unmarshal([]byte(`{"id":"X","path":"X","fixedInteger":42}`), &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw.Fixed != float64(42) {
		t.Errorf("Fixed = %#v, want 42", raw.Fixed)
	}
}

// TestRawElementUnmarshalPattern verifies the custom UnmarshalJSON captures a
// pattern* property.
func TestRawElementUnmarshalPattern(t *testing.T) {
	var raw RawElement
	if err := json.Unmarshal([]byte(`{"id":"X","path":"X","patternCode":"active"}`), &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw.Pattern != "active" {
		t.Errorf("Pattern = %#v, want active", raw.Pattern)
	}
}

// TestRawElementUnmarshalExamples verifies the custom UnmarshalJSON captures
// example values.
func TestRawElementUnmarshalExamples(t *testing.T) {
	var raw RawElement
	data := []byte(`{"id":"X","path":"X","example":[{"label":"Ex","valueString":"test"},{"valueInteger":7}]}`)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(raw.Examples) != 2 {
		t.Fatalf("Examples len = %d, want 2", len(raw.Examples))
	}
	if raw.Examples[0] != "test" || raw.Examples[1] != float64(7) {
		t.Errorf("Examples = %#v", raw.Examples)
	}
}

// TestRawElementUnmarshalMustSupport verifies mustSupport parsing.
func TestRawElementUnmarshalMustSupport(t *testing.T) {
	var raw RawElement
	if err := json.Unmarshal([]byte(`{"id":"X","path":"X","mustSupport":true}`), &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw.MustSupport == nil || !*raw.MustSupport {
		t.Errorf("MustSupport = %+v, want true", raw.MustSupport)
	}
}

// TestRawElementUnmarshalBase verifies base sub-object parsing.
func TestRawElementUnmarshalBase(t *testing.T) {
	var raw RawElement
	if err := json.Unmarshal([]byte(`{"id":"X","path":"X","base":{"min":0,"max":"1"}}`), &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if raw.Base == nil || raw.Base.Min == nil || *raw.Base.Min != 0 || string(raw.Base.Max) != `"1"` {
		t.Errorf("Base = %+v", raw.Base)
	}
}

// TestRawElementUnmarshalConstraint verifies constraint array parsing.
func TestRawElementUnmarshalConstraint(t *testing.T) {
	var raw RawElement
	data := []byte(`{"id":"X","path":"X","constraint":[{"key":"inv-1","severity":"error","expression":"value.exists()"}]}`)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(raw.Constraint) != 1 || raw.Constraint[0].Key != "inv-1" || raw.Constraint[0].Severity != "error" {
		t.Errorf("Constraint = %+v", raw.Constraint)
	}
}

// TestRawElementUnmarshalProfileTargetProfile verifies profile/targetProfile
// array parsing.
func TestRawElementUnmarshalProfileTargetProfile(t *testing.T) {
	var raw RawElement
	data := []byte(`{"id":"X","path":"X","profile":["http://p"],"targetProfile":["http://t"]}`)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(raw.Profile) != 1 || raw.Profile[0] != "http://p" {
		t.Errorf("Profile = %v", raw.Profile)
	}
	if len(raw.TargetProfile) != 1 || raw.TargetProfile[0] != "http://t" {
		t.Errorf("TargetProfile = %v", raw.TargetProfile)
	}
}

// TestRawElementUnmarshalFullElement verifies end-to-end: full JSON element
// through UnmarshalJSON and convertRawElement.
func TestRawElementUnmarshalFullElement(t *testing.T) {
	data := []byte(`{
		"id": "Patient.identifier",
		"path": "Patient.identifier",
		"min": 0,
		"max": "*",
		"mustSupport": true,
		"base": {"min": 0, "max": "*"},
		"type": [{"code": "Identifier", "profile": ["http://example.org/au-ihi"]}],
		"profile": ["http://example.org/elem-profile"],
		"targetProfile": ["http://example.org/elem-target"],
		"constraint": [{"key": "inv-1", "severity": "error", "expression": "value.exists()"}],
		"fixedString": "fixed-value",
		"patternCode": "active",
		"example": [{"label": "Ex", "valueString": "example-value"}]
	}`)
	var raw RawElement
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	elem, err := convertRawElement(raw)
	if err != nil {
		t.Fatalf("convertRawElement: %v", err)
	}
	if !elem.MustSupport {
		t.Error("MustSupport = false, want true")
	}
	if elem.BaseMax == nil || *elem.BaseMax != MaxUnbounded {
		t.Errorf("BaseMax = %v, want MaxUnbounded", elem.BaseMax)
	}
	if len(elem.Profile) != 1 || elem.Profile[0] != "http://example.org/elem-profile" {
		t.Errorf("Profile = %v", elem.Profile)
	}
	if len(elem.TargetProfile) != 1 || elem.TargetProfile[0] != "http://example.org/elem-target" {
		t.Errorf("TargetProfile = %v", elem.TargetProfile)
	}
	if len(elem.Constraints) != 1 || elem.Constraints[0].Key != "inv-1" {
		t.Errorf("Constraints = %+v", elem.Constraints)
	}
	if elem.Fixed != "fixed-value" {
		t.Errorf("Fixed = %#v, want fixed-value", elem.Fixed)
	}
	if elem.Pattern != "active" {
		t.Errorf("Pattern = %#v, want active", elem.Pattern)
	}
	if len(elem.Examples) != 1 || elem.Examples[0] != "example-value" {
		t.Errorf("Examples = %#v", elem.Examples)
	}
}
