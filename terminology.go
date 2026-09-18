package fhir

import "strings"

// ResolvedCoding is a single terminology coding that can be used as a generated
// value. It is the registry's contract for "give me a real code for this
// binding", so value synthesis (fhir-generator) never has to reason about
// ValueSet/CodeSystem expansion itself.
type ResolvedCoding struct {
	System  string
	Code    string
	Display string
}

// placeholderCodingCodes are codes representing null/placeholder values that a
// conformant instance must never carry (e.g. the v2-0203 "XX" null code).
var placeholderCodingCodes = map[string]bool{
	"XX": true, "UNK": true, "UN": true, "NULL": true, "NIL": true,
	"N/A": true, "NA": true, "NI": true, "OTH": true, "OT": true,
}

// placeholderDomains are example/placeholder URL domains that must never appear
// in a conformant instance, even when they appear in an IG's own example data.
var placeholderDomains = []string{
	"example.org", "acme.com", "example.com", "example.net", "example.edu",
	"hl7.org/fhir/StructureDefinition/0", "urn:oid:1.2.3.4.5",
}

// IsMeaningfulCoding reports whether a code should be used as a generated
// value: non-empty, not a placeholder/null code, and not a v3 abstract/group
// code (which begin with an underscore and are not valid instance values).
func IsMeaningfulCoding(code, display string) bool {
	if strings.TrimSpace(code) == "" {
		return false
	}
	trimmed := strings.ToUpper(strings.TrimSpace(code))
	if placeholderCodingCodes[trimmed] {
		return false
	}
	// v3 code systems mark abstract/group concepts with a leading underscore.
	if strings.HasPrefix(trimmed, "_") {
		return false
	}
	return true
}

// IsPlaceholderURL reports whether a URL is an example/placeholder domain that
// must not be emitted into a conformant instance.
func IsPlaceholderURL(url string) bool {
	if url == "" {
		return true
	}
	for _, d := range placeholderDomains {
		if strings.Contains(url, d) {
			return true
		}
	}
	return false
}

// stripCanonical removes a "|version" suffix and a "#fragment" from a canonical
// URL so lookups by canonical value match across versioned references.
func stripCanonical(url string) string {
	v := strings.TrimSpace(url)
	if i := strings.Index(v, "|"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "#"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// FirstConcept returns the first meaningful coding in a ValueSet, from its
// expansion if present, otherwise from its compose includes (explicit concepts
// first, then the included code system). It returns ok=false when the ValueSet
// is nil or carries no meaningful code.
func (r *Registry) FirstConcept(vs *ValueSet) (ResolvedCoding, bool) {
	if vs == nil {
		return ResolvedCoding{}, false
	}
	if vs.Expansion != nil {
		if coding, ok := firstExpansionCoding(vs.Expansion.Contains); ok {
			return coding, true
		}
	}
	if vs.Compose != nil {
		for _, include := range vs.Compose.Include {
			for _, concept := range include.Concept {
				if IsMeaningfulCoding(concept.Code, concept.Display) {
					return ResolvedCoding{System: include.System, Code: concept.Code, Display: concept.Display}, true
				}
			}
			if include.System == "" {
				continue
			}
			if cs, ok := r.CodeSystem(stripCanonical(include.System)); ok && cs != nil {
				if concept, ok := firstCodeSystemConcept(cs.Concepts); ok {
					return ResolvedCoding{System: include.System, Code: concept.Code, Display: concept.Display}, true
				}
			}
		}
	}
	return ResolvedCoding{}, false
}

// ResolveBoundCoding resolves a real coding for a value set canonical URL,
// preferring the ValueSet's expansion, then its compose/CodeSystem, then the
// package's example instances (see FirstExampleCoding/ExampleCodingForExtension).
func (r *Registry) ResolveBoundCoding(valueSetURL string) (ResolvedCoding, bool) {
	url := stripCanonical(valueSetURL)
	if url == "" {
		return ResolvedCoding{}, false
	}
	if vs, ok := r.ValueSet(url); ok && vs != nil {
		if coding, ok := r.FirstConcept(vs); ok {
			return coding, true
		}
	}
	return ResolvedCoding{}, false
}

// CodingDisplay returns the canonical CodeSystem display for a code, or "" when
// the CodeSystem or concept is not indexed.
func (r *Registry) CodingDisplay(system, code string) string {
	if system == "" || code == "" {
		return ""
	}
	cs, ok := r.CodeSystem(stripCanonical(system))
	if !ok || cs == nil {
		return ""
	}
	if c := findCodeSystemConceptByCode(cs.Concepts, code); c != nil {
		return c.Display
	}
	return ""
}

// FirstExampleCoding returns a real coding found at the element path within the
// package's example instance resources, preferring an instance whose
// meta.profile matches profileURL, then the first example of the resource type.
// path is the dot-separated element path below the resource root.
func (r *Registry) FirstExampleCoding(resourceType, path, profileURL string) (ResolvedCoding, bool) {
	if resourceType == "" || path == "" {
		return ResolvedCoding{}, false
	}
	instances := r.ResourcesForType(resourceType)
	if len(instances) == 0 {
		return ResolvedCoding{}, false
	}
	profileURL = stripCanonical(profileURL)
	for _, inst := range instances {
		if inst == nil || !hasCanonicalProfile(inst.ProfileURLs, profileURL) {
			continue
		}
		if coding, ok := codingAtPath(inst.Raw, path); ok {
			return r.conformExampleCoding(coding)
		}
	}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if coding, ok := codingAtPath(inst.Raw, path); ok {
			return r.conformExampleCoding(coding)
		}
	}
	return ResolvedCoding{}, false
}

// conformExampleCoding validates a coding extracted from a package example
// against the indexed CodeSystem. When the example's system is indexed:
//   - a code that does not exist in that CodeSystem is rejected (an IG's own
//     example can carry a stale/unknown code the validator rejects), and
//   - the canonical CodeSystem display replaces the example's display, so a
//     stale example display (e.g. "Simplified Profile for HL7 V2.4 REF message"
//     vs canonical "HL7 V2.4 REF message (Level 2)") never ships.
//
// A system-less coding, or a system the registry has not indexed, is returned
// as-is (it cannot be validated, and failing closed on every unindexed system
// would drop too many real examples).
func (r *Registry) conformExampleCoding(coding ResolvedCoding) (ResolvedCoding, bool) {
	if coding.System == "" {
		return coding, true
	}
	cs, ok := r.CodeSystem(stripCanonical(coding.System))
	if !ok || cs == nil {
		return coding, true
	}
	concept := findCodeSystemConceptByCode(cs.Concepts, coding.Code)
	if concept == nil {
		return ResolvedCoding{}, false
	}
	if concept.Display != "" {
		coding.Display = concept.Display
	}
	return coding, true
}

// ExampleCodingForExtension searches every example instance for an extension
// whose url equals extensionURL and returns the first meaningful coding from its
// valueCodeableConcept or valueCoding. The extension URL is globally unique.
func (r *Registry) ExampleCodingForExtension(extensionURL string) (ResolvedCoding, bool) {
	extensionURL = stripCanonical(extensionURL)
	if extensionURL == "" {
		return ResolvedCoding{}, false
	}
	for _, inst := range r.AllResources() {
		if inst == nil || inst.Raw == nil {
			continue
		}
		if coding, ok := findExtensionValueCoding(inst.Raw, extensionURL); ok {
			return coding, true
		}
	}
	return ResolvedCoding{}, false
}

// firstExpansionCoding returns the first meaningful coding in a ValueSet
// expansion, descending into nested contains.
func firstExpansionCoding(entries []ValueSetExpansionContains) (ResolvedCoding, bool) {
	for _, entry := range entries {
		if entry.Code != "" && IsMeaningfulCoding(entry.Code, entry.Display) {
			return ResolvedCoding{System: entry.System, Code: entry.Code, Display: entry.Display}, true
		}
		if coding, ok := firstExpansionCoding(entry.Contains); ok {
			return coding, true
		}
	}
	return ResolvedCoding{}, false
}

// firstCodeSystemConcept returns the first meaningful concept in a CodeSystem,
// descending into nested concepts.
func firstCodeSystemConcept(concepts []CodeSystemConcept) (CodeSystemConcept, bool) {
	for _, concept := range concepts {
		if concept.Code != "" && IsMeaningfulCoding(concept.Code, concept.Display) {
			return concept, true
		}
		if child, ok := firstCodeSystemConcept(concept.Concepts); ok {
			return child, true
		}
	}
	return CodeSystemConcept{}, false
}

// findCodeSystemConceptByCode returns the first CodeSystemConcept matching code,
// walking nested Concepts, or nil when not found.
func findCodeSystemConceptByCode(concepts []CodeSystemConcept, code string) *CodeSystemConcept {
	for i := range concepts {
		if concepts[i].Code == code {
			return &concepts[i]
		}
		if child := findCodeSystemConceptByCode(concepts[i].Concepts, code); child != nil {
			return child
		}
	}
	return nil
}

// hasCanonicalProfile reports whether profileURL (possibly versioned) matches
// any of the resource's declared profile URLs.
func hasCanonicalProfile(profiles []string, profileURL string) bool {
	if profileURL == "" {
		return false
	}
	for _, p := range profiles {
		if stripCanonical(p) == profileURL {
			return true
		}
	}
	return false
}

// codingAtPath walks a raw resource instance to the named element path and
// returns the first meaningful coding found there. It handles both a single
// element and a repeatable element (an array of objects). path is the
// dot-separated path below the resource root, e.g. "communication" or
// "code.coding". The final element is expected to carry a "coding" array.
func codingAtPath(raw map[string]any, path string) (ResolvedCoding, bool) {
	if raw == nil || path == "" {
		return ResolvedCoding{}, false
	}
	segments := strings.Split(path, ".")
	var cur any = raw
	for _, seg := range segments {
		switch typed := cur.(type) {
		case map[string]any:
			next, ok := typed[seg]
			if !ok {
				return ResolvedCoding{}, false
			}
			cur = next
		case []any:
			found := false
			for _, item := range typed {
				if itemMap, ok := item.(map[string]any); ok {
					if next, ok := itemMap[seg]; ok {
						cur = next
						found = true
						break
					}
				}
			}
			if !found {
				return ResolvedCoding{}, false
			}
		default:
			return ResolvedCoding{}, false
		}
	}
	return firstCodingInValue(cur)
}

// firstCodingInValue extracts the first meaningful coding from a value that is
// either a CodeableConcept (map with "coding"), a Coding (map with system+code),
// or an array of those.
func firstCodingInValue(v any) (ResolvedCoding, bool) {
	switch typed := v.(type) {
	case []any:
		for _, item := range typed {
			if coding, ok := firstCodingInValue(item); ok {
				return coding, true
			}
		}
		return ResolvedCoding{}, false
	case map[string]any:
		if codings, ok := typed["coding"].([]any); ok {
			for _, c := range codings {
				if cm, ok := c.(map[string]any); ok {
					if coding, ok := codingFromMap(cm); ok {
						return coding, true
					}
				}
			}
			return ResolvedCoding{}, false
		}
		return codingFromMap(typed)
	default:
		return ResolvedCoding{}, false
	}
}

// codingFromMap converts a raw coding map into a ResolvedCoding, returning
// false when the code is empty or not meaningful.
func codingFromMap(m map[string]any) (ResolvedCoding, bool) {
	code, _ := m["code"].(string)
	if !IsMeaningfulCoding(code, "") {
		return ResolvedCoding{}, false
	}
	system, _ := m["system"].(string)
	display, _ := m["display"].(string)
	return ResolvedCoding{System: system, Code: code, Display: display}, true
}

// findExtensionValueCoding walks a resource instance's extension array (and
// nested extension arrays) looking for an extension whose url equals
// extensionURL, then extracts the first meaningful coding from its
// valueCodeableConcept or valueCoding.
func findExtensionValueCoding(raw map[string]any, extensionURL string) (ResolvedCoding, bool) {
	if raw == nil || extensionURL == "" {
		return ResolvedCoding{}, false
	}
	return findExtensionValueCodingInAny(raw, extensionURL)
}

func findExtensionValueCodingInAny(v any, extensionURL string) (ResolvedCoding, bool) {
	switch typed := v.(type) {
	case []any:
		for _, item := range typed {
			if coding, ok := findExtensionValueCodingInAny(item, extensionURL); ok {
				return coding, true
			}
		}
		return ResolvedCoding{}, false
	case map[string]any:
		if u, ok := typed["url"].(string); ok && stripCanonical(u) == extensionURL {
			if coding, ok := extensionValueCoding(typed); ok {
				return coding, true
			}
		}
		for _, val := range typed {
			if coding, ok := findExtensionValueCodingInAny(val, extensionURL); ok {
				return coding, true
			}
		}
		return ResolvedCoding{}, false
	default:
		return ResolvedCoding{}, false
	}
}

// extensionValueCoding extracts the first meaningful coding from an extension's
// valueCodeableConcept or valueCoding.
func extensionValueCoding(ext map[string]any) (ResolvedCoding, bool) {
	if ext == nil {
		return ResolvedCoding{}, false
	}
	if vcc, ok := ext["valueCodeableConcept"]; ok {
		if coding, ok := firstCodingInValue(vcc); ok {
			return coding, true
		}
	}
	if vc, ok := ext["valueCoding"]; ok {
		if coding, ok := firstCodingInValue(vc); ok {
			return coding, true
		}
	}
	return ResolvedCoding{}, false
}
