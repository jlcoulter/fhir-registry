package fhir

import (
	"sync"
	"testing"
)

// TestConcurrentAddAndReadNewTypes exercises concurrent mutation and reads of
// the new resource indexes. Run with -race to detect data races.
func TestConcurrentAddAndReadNewTypes(t *testing.T) {
	reg := NewRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vs := &ValueSet{URL: "http://example.org/ValueSet/vs" + string(rune('a'+i)), Status: "active"}
			reg.AddValueSet(vs)
			cs := &CodeSystem{URL: "http://example.org/CodeSystem/cs" + string(rune('a'+i)), Status: "draft"}
			reg.AddCodeSystem(cs)
			sp := &SearchParameter{URL: "http://example.org/SearchParameter/sp" + string(rune('a'+i)), Code: "code" + string(rune('a'+i)), Base: []string{"Patient"}}
			reg.AddSearchParameter(sp)
			reg.AddResource(&Resource{ResourceType: "Patient", Raw: map[string]any{"id": string(rune('a' + i))}})
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = reg.ValueSet("http://example.org/ValueSet/vs" + string(rune('a'+i)))
			_, _ = reg.CodeSystem("http://example.org/CodeSystem/cs" + string(rune('a'+i)))
			_, _ = reg.SearchParameter("Patient", "code"+string(rune('a'+i)))
			_ = reg.ResourcesForType("Patient")
			_ = reg.AllResources()
			_ = reg.CapabilityStatements()
			_ = reg.SearchParameters()
		}(i)
	}
	wg.Wait()
}

// TestAllResourcesDeterminism verifies that AllResources returns resources in
// a deterministic order (sorted by resource type), regardless of insertion
// order.
func TestAllResourcesDeterminism(t *testing.T) {
	reg := NewRegistry()
	// Insert in non-sorted order.
	reg.AddResource(&Resource{ResourceType: "Observation", Raw: map[string]any{"id": "o1"}})
	reg.AddResource(&Resource{ResourceType: "Patient", Raw: map[string]any{"id": "p1"}})
	reg.AddResource(&Resource{ResourceType: "Encounter", Raw: map[string]any{"id": "e1"}})

	all := reg.AllResources()
	if len(all) != 3 {
		t.Fatalf("AllResources() len = %d, want 3", len(all))
	}
	// Expect sorted by resourceType: Encounter, Observation, Patient.
	want := []string{"Encounter", "Observation", "Patient"}
	for i, w := range want {
		if all[i].ResourceType != w {
			t.Errorf("AllResources()[%d].ResourceType = %q, want %q (sorted order)", i, all[i].ResourceType, w)
		}
	}
}
