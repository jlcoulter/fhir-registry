package fhir

import (
	"fmt"
	"strings"
)

// Severity distinguishes report items.
type Severity string

const (
	// SeverityViolation marks a cardinality or structural violation that
	// should be fixed.
	SeverityViolation Severity = "violation"
	// SeverityWarning marks a non-fatal issue, such as an unknown key.
	SeverityWarning Severity = "warning"
)

// ReportItem describes a problem found while marshaling.
type ReportItem struct {
	// Path is the element path (e.g. "Patient.name") the problem relates to.
	Path string
	// Severity is either SeverityViolation or SeverityWarning.
	Severity Severity
	// Message is a human-readable description of the problem.
	Message string
}

// MarshalReport collects diagnostics from a Marshal call.
type MarshalReport struct {
	// Items holds the problems found, in the order they were encountered.
	Items []ReportItem
}

func (r *MarshalReport) add(severity Severity, path, format string, args ...any) {
	r.Items = append(r.Items, ReportItem{
		Path:     path,
		Severity: severity,
		Message:  fmt.Sprintf(format, args...),
	})
}

// Marshal normalizes an instance object against the element tree for a base
// type, resolving complex-type references through the registry for deep
// recursion. Repeating elements (max > 1) are emitted as arrays, single
// elements (max == 1) as scalars, and choice elements use their concrete
// type-suffixed key. Unknown keys are preserved (with a warning), and
// cardinality violations are reported in the returned report.
func (r *Registry) Marshal(resourceType string, instance map[string]any) (map[string]any, *MarshalReport, error) {
	tree, err := r.TreeForType(resourceType)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: type %s", ErrDefinitionNotFound, resourceType)
	}
	rep := &MarshalReport{}
	// resourceType is a special field not present in the element tree; handle
	// it out-of-band so it is not reported as an unknown key.
	rt, hasRT := instance["resourceType"]
	body := make(map[string]any, len(instance))
	for k, v := range instance {
		if k != "resourceType" {
			body[k] = v
		}
	}
	out, err := marshalObject(r, tree.Root, tree, body, rep)
	if err != nil {
		return nil, rep, err
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, rep, fmt.Errorf("root element did not produce an object")
	}
	if hasRT {
		m["resourceType"] = rt
	}
	return m, rep, nil
}

// baseChildren returns the direct child element definitions of a node, one per
// path (slices collapsed to their base element). Complex-type children with no
// in-tree children are resolved through the registry.
func (r *Registry) baseChildren(tree *ElementTree, elem *ElementDefinition) []*ElementDefinition {
	var children []*ElementDefinition
	if len(elem.Children) > 0 {
		children = elem.Children
	} else if t, ok := r.ResolveType(PrimaryTypeCode(elem), profilesOf(elem)); ok {
		children = t.Root.Children
	} else {
		return nil
	}

	// Collapse slices: keep only the first (base, non-slice) element per path.
	seen := make(map[string]bool)
	base := make([]*ElementDefinition, 0, len(children))
	for _, c := range children {
		if seen[c.Path] {
			continue
		}
		seen[c.Path] = true
		base = append(base, c)
	}
	return base
}

func profilesOf(elem *ElementDefinition) []string {
	if len(elem.Types) == 0 {
		return nil
	}
	return elem.Types[0].Profiles
}

// marshalObject processes a JSON object value against an element definition.
func marshalObject(r *Registry, elem *ElementDefinition, tree *ElementTree, obj map[string]any, rep *MarshalReport) (any, error) {
	children := r.baseChildren(tree, elem)

	out := make(map[string]any, len(obj))
	emitted := make(map[string]bool, len(children))
	// seen guards against sibling elements that share a last path segment:
	// the first (direct) child owns the JSON key, so later siblings with the
	// same key must not re-process the value.
	seen := make(map[string]bool, len(children))
	for _, child := range children {
		key := lastSegment(child.Path)
		if seen[key] {
			continue
		}
		seen[key] = true

		if IsChoice(child) {
			for _, ck := range marshalChoice(r, child, tree, obj, out, rep) {
				emitted[ck] = true
			}
			continue
		}

		prim, hasPrim := obj[key]
		under, hasUnder := obj["_"+key]

		if !hasPrim && !hasUnder {
			if child.Min > 0 && !isExtensionElement(child) {
				rep.add(SeverityViolation, child.Path, "required element missing (min=%d)", child.Min)
			}
			continue
		}

		if IsMulti(child) {
			norm, err := marshalArray(r, child, tree, obj, key, rep)
			if err != nil {
				return nil, err
			}
			out[key] = norm
		} else {
			v := prim
			if arr, isArr := prim.([]any); isArr && len(arr) == 1 {
				v = arr[0]
			}
			norm, err := marshalValue(r, child, tree, v, rep)
			if err != nil {
				return nil, err
			}
			if hasPrim {
				out[key] = norm
			}
		}
		if hasPrim {
			emitted[key] = true
		}
		// The "_key" extension companion of a primitive passes through.
		if hasUnder {
			out["_"+key] = under
			emitted["_"+key] = true
		}
	}

	// Preserve keys not covered by the definition.
	for k, v := range obj {
		if emitted[k] {
			continue
		}
		rep.add(SeverityWarning, elem.Path, "unknown key %q passed through", k)
		out[k] = v
	}
	return out, nil
}

// isExtensionElement reports whether an element is an extension container,
// which is never required even when a profile sets min > 0.
func isExtensionElement(elem *ElementDefinition) bool {
	seg := lastSegment(elem.Path)
	return seg == "extension" || seg == "modifierExtension"
}

func marshalArray(r *Registry, elem *ElementDefinition, tree *ElementTree, obj map[string]any, key string, rep *MarshalReport) (any, error) {
	v, present := obj[key]
	if !present {
		return nil, nil
	}
	items, isArr := v.([]any)
	if !isArr {
		items = []any{v}
	}
	norm := make([]any, 0, len(items))
	for _, item := range items {
		n, err := marshalValue(r, elem, tree, item, rep)
		if err != nil {
			return nil, err
		}
		norm = append(norm, n)
	}
	return norm, nil
}

func marshalValue(r *Registry, elem *ElementDefinition, tree *ElementTree, v any, rep *MarshalReport) (any, error) {
	switch val := v.(type) {
	case map[string]any:
		return marshalObject(r, elem, tree, val, rep)
	case []any:
		// Match marshalObject's scalar handling: a single-element array for a
		// single-valued (max == 1) element is unwrapped to its scalar value.
		if !elem.Max.IsUnbounded() {
			if len(val) == 1 {
				return marshalValue(r, elem, tree, val[0], rep)
			}
			// More than one value for a max==1 element: keep the array; the
			// elements are still normalized below.
		}
		norm := make([]any, 0, len(val))
		for _, item := range val {
			n, err := marshalValue(r, elem, tree, item, rep)
			if err != nil {
				return nil, err
			}
			norm = append(norm, n)
		}
		return norm, nil
	default:
		return v, nil
	}
}

func marshalChoice(r *Registry, elem *ElementDefinition, tree *ElementTree, obj, out map[string]any, rep *MarshalReport) []string {
	matched := 0
	var emitted []string
	for _, ty := range elem.Types {
		ck := ChoiceName(elem, ty.Code)
		if v, present := obj[ck]; present {
			norm, err := marshalValue(r, elem, tree, v, rep)
			if err != nil {
				continue
			}
			out[ck] = norm
			emitted = append(emitted, ck)
			matched++
		}
	}
	if matched > 1 {
		rep.add(SeverityViolation, elem.Path, "choice element has multiple concrete values")
	}
	return emitted
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}
	return path
}
