package fhir

import (
	"os"
	"testing"
)

// TestLoadPackageTgzIndexesValueSets verifies that loading a real FHIR package
// indexes its ValueSets by canonical URL.
func TestLoadPackageTgzIndexesValueSets(t *testing.T) {
	reg := loadTestRegistry(t)
	vs, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type")
	if !ok || vs == nil {
		t.Fatal("ValueSet not indexed from tgz")
	}
	if vs.Status != "active" {
		t.Errorf("status = %q, want active", vs.Status)
	}
	if vs.Compose == nil || len(vs.Compose.Include) != 1 {
		t.Fatalf("compose.include = %+v", vs.Compose)
	}
	inc := vs.Compose.Include[0]
	if inc.System != "http://terminology.hl7.org/CodeSystem/v2-0203" {
		t.Errorf("include system = %q", inc.System)
	}
	if len(inc.Concept) != 2 || inc.Concept[0].Code != "ACSN" {
		t.Errorf("include concepts = %+v", inc.Concept)
	}
}

// TestLoadPackageTgzIndexesCodeSystems verifies that loading a real FHIR
// package indexes its CodeSystems by canonical URL, including concepts.
func TestLoadPackageTgzIndexesCodeSystems(t *testing.T) {
	reg := loadTestRegistry(t)
	cs, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/contact-purpose")
	if !ok || cs == nil {
		t.Fatal("CodeSystem not indexed from tgz")
	}
	if cs.Status != "active" {
		t.Errorf("status = %q, want active", cs.Status)
	}
	if len(cs.Concepts) == 0 {
		t.Fatal("CodeSystem has no concepts")
	}
	if cs.Concepts[0].Code == "" {
		t.Errorf("first concept code empty: %+v", cs.Concepts[0])
	}
}

// TestLoadPackageTgzIndexesSearchParameters verifies that loading a real FHIR
// package indexes its SearchParameters by (base, code).
func TestLoadPackageTgzIndexesSearchParameters(t *testing.T) {
	reg := loadTestRegistry(t)
	sp, ok := reg.SearchParameter("Patient", "indigenous-status")
	if !ok || sp == nil {
		t.Fatal("SearchParameter not indexed from tgz")
	}
	if sp.Type != "token" {
		t.Errorf("type = %q, want token", sp.Type)
	}
	if sp.URL != "http://hl7.org.au/fhir/SearchParameter/indigenous-status" {
		t.Errorf("url = %q", sp.URL)
	}
}

// TestLoadPackageTgzIndexesGenericResources verifies that loading a real FHIR
// package indexes non-specialised resources (e.g. ImplementationGuide) as
// generic Resources.
func TestLoadPackageTgzIndexesGenericResources(t *testing.T) {
	reg := loadTestRegistry(t)
	resources := reg.ResourcesForType("ImplementationGuide")
	if len(resources) != 1 {
		t.Fatalf("ResourcesForType(ImplementationGuide) len = %d, want 1", len(resources))
	}
	if resources[0].ResourceType != "ImplementationGuide" {
		t.Errorf("ResourceType = %q", resources[0].ResourceType)
	}
	if resources[0].Raw["id"] != "hl7.fhir.au.base" {
		t.Errorf("Raw id = %v", resources[0].Raw["id"])
	}
}

// TestLoadPackageTgzIndexesPatientResources verifies that example instance
// resources (in the example/ subdirectory) are indexed as generic Resources
// with ProfileURLs extracted from meta.profile.
func TestLoadPackageTgzIndexesPatientResources(t *testing.T) {
	reg := loadTestRegistry(t)
	resources := reg.ResourcesForType("Patient")
	if len(resources) == 0 {
		t.Fatal("no Patient resources indexed from tgz")
	}
	for _, res := range resources {
		if res.ResourceType != "Patient" {
			t.Errorf("ResourceType = %q, want Patient", res.ResourceType)
		}
		if res.Raw["id"] == "" {
			t.Errorf("Raw id empty: %+v", res.Raw)
		}
	}
}

// TestLoadPackageDirectoryIndexesNewTypes verifies that LoadPackage (directory
// form) indexes ValueSets, CodeSystems, and SearchParameters from a real
// package layout.
func TestLoadPackageDirectoryIndexesNewTypes(t *testing.T) {
	if _, err := os.Stat("au-base.tgz"); err != nil {
		t.Skip("au-base.tgz not present")
	}
	pkgDir := t.TempDir()
	f, err := os.Open("au-base.tgz")
	if err != nil {
		t.Fatalf("open au-base.tgz: %v", err)
	}
	if err := extractTGZToDir(f, pkgDir); err != nil {
		f.Close()
		t.Fatalf("extractTGZToDir: %v", err)
	}
	f.Close()

	reg := NewRegistry()
	if err := reg.LoadPackage(pkgDir); err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type"); !ok {
		t.Error("ValueSet not indexed from directory")
	}
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/contact-purpose"); !ok {
		t.Error("CodeSystem not indexed from directory")
	}
	if _, ok := reg.SearchParameter("Patient", "indigenous-status"); !ok {
		t.Error("SearchParameter not indexed from directory")
	}
}
