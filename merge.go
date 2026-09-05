package fhir

import (
	"sort"
	"strings"
)

// MergeDifferential applies a differential element list onto a base snapshot
// element list, producing the merged element list for a derived profile.
//
// The result preserves all base elements, with differential constraints
// overlaid onto their matching base element, plus any new elements the
// differential introduces (slices, new children, added extensions).
//
// This implements the practical subset of the FHIR differential-to-snapshot
// algorithm that is relevant for cardinality-correct marshaling:
//
//   - min/max overrides
//   - type narrowing (replacement)
//   - binding replacement
//   - short/definition/comment overrides
//   - isModifier/isSummary overrides
//   - slice introduction and new element insertion
//   - slicing metadata
//
// An explicit min: 0 in the differential relaxes a required base element to
// optional; an absent min leaves the base minimum untouched.
func MergeDifferential(base, differential []RawElement) []RawElement {
	// Work on a shallow copy so callers' slices are not mutated.
	out := make([]RawElement, 0, len(base)+len(differential))
	for _, b := range base {
		out = append(out, b)
	}

	// Index base elements by id for overlay lookup.
	byID := make(map[string]int, len(out))
	for i := range out {
		if out[i].ID != "" {
			byID[out[i].ID] = i
		}
	}

	// Apply differential elements: overlay onto matching base, else append.
	for _, diff := range differential {
		idx, ok := byID[diff.ID]
		if ok {
			out[idx] = overlay(out[idx], diff)
			continue
		}
		// New element: insert. Record id for subsequent overlays.
		out = append(out, diff)
		byID[diff.ID] = len(out) - 1
	}

	// Sort into a deterministic parent-before-child path order so the tree
	// builder links parents correctly regardless of insertion order.
	sort.SliceStable(out, func(i, j int) bool {
		return elementOrderLess(out[i].Path, out[j].Path)
	})
	return out
}

// elementOrderLess orders paths so that a path is always before its children
// (a child path equals the parent path plus "." plus more).
func elementOrderLess(a, b string) bool {
	if a == b {
		return false
	}
	// Compare segment by segment; a path that is a prefix (by ".") sorts first.
	aseg := strings.Split(a, ".")
	bseg := strings.Split(b, ".")
	n := len(aseg)
	if len(bseg) < n {
		n = len(bseg)
	}
	for i := 0; i < n; i++ {
		if aseg[i] != bseg[i] {
			return aseg[i] < bseg[i]
		}
	}
	// One is a prefix of the other; the shorter (ancestor) comes first.
	return len(aseg) < len(bseg)
}

// overlay applies non-empty differential fields onto a base element.
func overlay(base, diff RawElement) RawElement {
	// Structural fields are preserved from base.
	base.Short = or(base.Short, diff.Short)
	base.Definition = or(base.Definition, diff.Definition)
	base.Comment = or(base.Comment, diff.Comment)
	base.MeaningWhenMissing = or(base.MeaningWhenMissing, diff.MeaningWhenMissing)

	// Cardinality.
	if len(diff.Max) > 0 {
		base.Max = diff.Max
	}
	if diff.Min != nil {
		base.Min = diff.Min
	}

	// Type narrowing: replace the whole list when the differential specifies types.
	if len(diff.Types) > 0 {
		base.Types = diff.Types
	}

	// Binding replacement.
	if diff.Binding != nil {
		base.Binding = diff.Binding
	}

	// Modifier / summary flags. A nil differential value (field absent)
	// leaves the base flag untouched; an explicit value, including false,
	// overrides it.
	if diff.IsModifier != nil {
		base.IsModifier = diff.IsModifier
	}
	if diff.IsSummary != nil {
		base.IsSummary = diff.IsSummary
	}

	// Conditions and content references.
	if len(diff.Condition) > 0 {
		base.Condition = diff.Condition
	}
	if diff.ContentReference != "" {
		base.ContentReference = diff.ContentReference
	}

	// Slicing metadata.
	if diff.Slicing != nil {
		base.Slicing = diff.Slicing
	}

	// SliceName must match the element identity; keep whichever is set.
	if base.SliceName == "" {
		base.SliceName = diff.SliceName
	}

	return base
}

func or(a, b string) string {
	if b != "" {
		return b
	}
	return a
}
