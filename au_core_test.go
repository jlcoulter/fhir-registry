package fhir

import (
	"context"
	"os"
	"testing"
	"time"
)

// loadAuCoreAndAuBase loads both the au-core and au-base packages into a single
// registry, mirroring the parent dependency chain (au-core depends on au-base).
func loadAuCoreAndAuBase(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := reg.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	return reg
}

// ---------------------------------------------------------------------------
// Cross-package loading
// ---------------------------------------------------------------------------

func TestLoadAuCoreWithAuBase_StructureDefinitions(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	// au-core profiles.
	for _, url := range []string{
		"http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient",
		"http://hl7.org.au/fhir/core/StructureDefinition/au-core-bloodpressure",
		"http://hl7.org.au/fhir/core/StructureDefinition/au-core-allergyintolerance",
	} {
		if _, ok := reg.Definition(url); !ok {
			t.Errorf("au-core SD %s not indexed", url)
		}
	}
	// au-base profiles (the dependency).
	for _, url := range []string{
		"http://hl7.org.au/fhir/StructureDefinition/au-patient",
		"http://hl7.org.au/fhir/StructureDefinition/au-organization",
	} {
		if _, ok := reg.Definition(url); !ok {
			t.Errorf("au-base SD %s not indexed", url)
		}
	}
	// Base FHIR SDs are NOT present (R4 core package not loaded).
	if _, ok := reg.Definition("http://hl7.org/fhir/StructureDefinition/Patient"); ok {
		t.Error("base Patient SD should not be present without R4 core")
	}
	// DefinitionsForType returns both the au-base and au-core Patient profiles.
	defs := reg.DefinitionsForType("Patient")
	var hasBase, hasCore bool
	for _, d := range defs {
		switch d.URL {
		case "http://hl7.org.au/fhir/StructureDefinition/au-patient":
			hasBase = true
		case "http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient":
			hasCore = true
		}
	}
	if !hasBase || !hasCore {
		t.Errorf("DefinitionsForType(Patient) missing base=%v core=%v: %d defs", hasBase, hasCore, len(defs))
	}
}

func TestLoadAuCoreWithAuBase_Terminology(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	// ValueSets come from au-base (au-core ships none).
	for _, url := range []string{
		"http://terminology.hl7.org.au/ValueSet/accession-number-type",
		"http://terminology.hl7.org.au/ValueSet/contact-purpose",
	} {
		if _, ok := reg.ValueSet(url); !ok {
			t.Errorf("ValueSet %s not indexed", url)
		}
	}
	// CodeSystems come from au-base.
	for _, url := range []string{
		"http://terminology.hl7.org.au/CodeSystem/contact-purpose",
		"http://terminology.hl7.org.au/CodeSystem/medication-type",
	} {
		if _, ok := reg.CodeSystem(url); !ok {
			t.Errorf("CodeSystem %s not indexed", url)
		}
	}
}

func TestLoadAuCoreWithAuBase_SearchParameters(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	// au-base SPs.
	if _, ok := reg.SearchParameter("Patient", "indigenous-status"); !ok {
		t.Error("au-base indigenous-status SP not indexed")
	}
	// au-core SPs (multi-base and single-base).
	if _, ok := reg.SearchParameter("AllergyIntolerance", "patient"); !ok {
		t.Error("au-core clinical-patient SP not indexed for AllergyIntolerance")
	}
	if _, ok := reg.SearchParameter("PractitionerRole", "practitioner"); !ok {
		t.Error("au-core practitionerrole-practitioner SP not indexed")
	}
}

func TestLoadAuCoreWithAuBase_CapabilityStatements(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	css := reg.CapabilityStatements()
	if len(css) != 2 {
		t.Fatalf("CapabilityStatements len = %d, want 2", len(css))
	}
	var responder, requester *CapabilityStatement
	for _, cs := range css {
		switch cs.URL {
		case "http://hl7.org.au/fhir/core/CapabilityStatement/au-core-responder":
			responder = cs
		case "http://hl7.org.au/fhir/core/CapabilityStatement/au-core-requester":
			requester = cs
		}
	}
	if responder == nil {
		t.Fatal("responder CS not found")
	}
	if responder.Status != "active" {
		t.Errorf("responder status = %q, want active", responder.Status)
	}
	if len(responder.Rest) != 1 || responder.Rest[0].Mode != "server" {
		t.Errorf("responder rest mode = %+v, want server", responder.Rest)
	}
	if requester == nil {
		t.Fatal("requester CS not found")
	}
	if len(requester.Rest) != 1 || requester.Rest[0].Mode != "client" {
		t.Errorf("requester rest mode = %+v, want client", requester.Rest)
	}
}

func TestLoadAuCoreWithAuBase_GenericResources(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	// Patient examples from both packages.
	patients := reg.ResourcesForType("Patient")
	if len(patients) == 0 {
		t.Fatal("no Patient resources indexed")
	}
	// At least one Patient example carries the au-core-patient profile.
	var sawCoreProfile bool
	for _, p := range patients {
		for _, u := range p.ProfileURLs {
			if u == "http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient" {
				sawCoreProfile = true
			}
		}
	}
	if !sawCoreProfile {
		t.Error("no Patient example with au-core-patient profile found")
	}
	// Observation examples from au-core.
	if len(reg.ResourcesForType("Observation")) == 0 {
		t.Error("no Observation resources indexed")
	}
	// ImplementationGuide from au-base.
	if len(reg.ResourcesForType("ImplementationGuide")) == 0 {
		t.Error("no ImplementationGuide resources indexed")
	}
	// AllResources is sorted by resource type.
	all := reg.AllResources()
	for i := 1; i < len(all); i++ {
		if all[i-1].ResourceType > all[i].ResourceType {
			t.Errorf("AllResources not sorted: %s before %s", all[i-1].ResourceType, all[i].ResourceType)
		}
	}
}

func TestLoadAuCoreWithAuBase_DependencyChain(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	core, ok := reg.Definition("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient")
	if !ok {
		t.Fatal("au-core-patient not indexed")
	}
	if core.BaseDefinition != "http://hl7.org.au/fhir/StructureDefinition/au-patient" {
		t.Errorf("au-core-patient baseDefinition = %q, want au-patient", core.BaseDefinition)
	}
	base, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient")
	if !ok {
		t.Fatal("au-patient not indexed")
	}
	if base.BaseDefinition != "http://hl7.org/fhir/StructureDefinition/Patient" {
		t.Errorf("au-patient baseDefinition = %q, want R4 Patient", base.BaseDefinition)
	}
}

// ---------------------------------------------------------------------------
// Tree and type resolution
// ---------------------------------------------------------------------------

func TestLoadAuCoreWithAuBase_Tree(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	tree, err := reg.Tree("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient")
	if err != nil {
		t.Fatalf("Tree(au-core-patient): %v", err)
	}
	if tree.Root == nil {
		t.Fatal("tree.Root is nil")
	}
	if tree.Root.Path != "Patient" {
		t.Errorf("root path = %q, want Patient", tree.Root.Path)
	}
	if len(tree.ByPath) == 0 {
		t.Error("tree.ByPath is empty")
	}
	if _, ok := tree.ByPath["Patient.name"]; !ok {
		t.Error("tree.ByPath missing Patient.name")
	}
}

func TestLoadAuCoreWithAuBase_TreeForType(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	tree, err := reg.TreeForType("Patient")
	if err != nil {
		t.Fatalf("TreeForType(Patient): %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Patient" {
		t.Errorf("TreeForType(Patient) root = %+v, want Patient", tree.Root)
	}
}

func TestLoadAuCoreWithAuBase_ResolveType(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	// Explicit profile resolves to the au-core tree.
	tree, ok := reg.ResolveType("Patient", []string{"http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient"})
	if !ok {
		t.Fatal("ResolveType with au-core-patient failed")
	}
	if tree.SD.URL != "http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient" {
		t.Errorf("ResolveType SD = %q, want au-core-patient", tree.SD.URL)
	}
	// No profiles and no R4 base: with multiple Patient defs and no base
	// definition, ResolveType cannot disambiguate and returns false.
	if _, ok := reg.ResolveType("Patient", nil); ok {
		t.Error("ResolveType with no profiles should fail (no R4 base, multiple defs)")
	}
}

func TestLoadAuCoreWithAuBase_Marshal(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	in := map[string]any{
		"resourceType": "Patient",
		"name": []any{
			map[string]any{"family": "Smith", "given": []any{"Jane"}},
		},
	}
	out, rep, err := reg.Marshal("Patient", in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if out == nil {
		t.Fatal("Marshal returned nil output")
	}
	if out["resourceType"] != "Patient" {
		t.Errorf("resourceType = %v, want Patient", out["resourceType"])
	}
	if rep == nil {
		t.Fatal("Marshal returned nil report")
	}
}

// ---------------------------------------------------------------------------
// Scope tests with au-core
// ---------------------------------------------------------------------------

func TestLoadAuCoreScopeFromCapabilityStatement(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	var responder *CapabilityStatement
	for _, cs := range reg.CapabilityStatements() {
		if cs.URL == "http://hl7.org.au/fhir/core/CapabilityStatement/au-core-responder" {
			responder = cs
		}
	}
	if responder == nil {
		t.Fatal("responder CS not found")
	}
	s := ScopeFromCapabilityStatement(responder)
	if s == nil {
		t.Fatal("ScopeFromCapabilityStatement returned nil")
	}
	// 24 resource types from the CS.
	if len(s.ResourceTypes) != 24 {
		t.Errorf("ResourceTypes len = %d, want 24", len(s.ResourceTypes))
	}
	if !s.AllowsResourceType("Patient") || !s.AllowsResourceType("Observation") {
		t.Error("Patient/Observation should be in scope")
	}
	if s.AllowsResourceType("Address") {
		t.Error("Address should not be in scope")
	}
	// au-core profiles from CS profile fields.
	if !s.AllowsProfile("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient") {
		t.Error("au-core-patient profile should be in scope")
	}
	// Referenced policies.
	if s.ValueSets != ScopeReferenced || s.CodeSystems != ScopeReferenced {
		t.Errorf("ValueSets/CodeSystems = %q/%q, want ScopeReferenced", s.ValueSets, s.CodeSystems)
	}
	if s.SearchParams != ScopeReferenced || s.GenericResources != ScopeReferenced {
		t.Errorf("SearchParams/GenericResources = %q/%q, want ScopeReferenced", s.SearchParams, s.GenericResources)
	}
}

func TestLoadAuCoreWithScopeFromCS_FiltersSDs(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	var responder *CapabilityStatement
	for _, cs := range reg.CapabilityStatements() {
		if cs.URL == "http://hl7.org.au/fhir/core/CapabilityStatement/au-core-responder" {
			responder = cs
		}
	}
	scoped := NewRegistry()
	scoped.Scope = ScopeFromCapabilityStatement(responder)
	if err := scoped.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := scoped.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	scoped.Resolve()
	// In-scope: au-core-patient (profile in CS), au-patient (base for Patient).
	if _, ok := scoped.Definition("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient"); !ok {
		t.Error("au-core-patient should be indexed")
	}
	if _, ok := scoped.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient should be indexed")
	}
	// Out-of-scope: Address is not in the CS's 24 types.
	if _, ok := scoped.Definition("http://hl7.org.au/fhir/StructureDefinition/au-address"); ok {
		t.Error("au-address should be filtered out (Address not in scope)")
	}
}

func TestLoadAuCoreWithScopeReferencedValueSets(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Organization").WithValueSets(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := reg.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	reg.Resolve()
	// Organization SDs bind to contact-purpose.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/contact-purpose"); !ok {
		t.Error("contact-purpose ValueSet should be indexed (referenced by Organization SD)")
	}
	// accession-number-type is not referenced by any Organization SD.
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type"); ok {
		t.Error("accession-number-type ValueSet should be filtered out")
	}
}

func TestLoadAuCoreWithScopeReferencedCodeSystems(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Organization").WithValueSets(ScopeReferenced).WithCodeSystems(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := reg.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	reg.Resolve()
	// contact-purpose CodeSystem is referenced by the contact-purpose ValueSet.
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/contact-purpose"); !ok {
		t.Error("contact-purpose CodeSystem should be indexed")
	}
	// medication-type is not referenced by any in-scope ValueSet.
	if _, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/medication-type"); ok {
		t.Error("medication-type CodeSystem should be filtered out")
	}
}

func TestLoadAuCoreWithScopeReferencedSearchParams(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("AllergyIntolerance", "PractitionerRole").WithSearchParams(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := reg.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	// au-core clinical-patient SP has AllergyIntolerance in its base list.
	if _, ok := reg.SearchParameter("AllergyIntolerance", "patient"); !ok {
		t.Error("clinical-patient SP should be indexed (base includes AllergyIntolerance)")
	}
	// au-core practitionerrole-practitioner SP has PractitionerRole in base.
	if _, ok := reg.SearchParameter("PractitionerRole", "practitioner"); !ok {
		t.Error("practitionerrole-practitioner SP should be indexed")
	}
	// au-base indigenous-status SP has Patient in base, which is out of scope.
	if _, ok := reg.SearchParameter("Patient", "indigenous-status"); ok {
		t.Error("indigenous-status SP should be filtered out (base Patient out of scope)")
	}
	// discharge-disposition has base Encounter, which is out of scope.
	if _, ok := reg.SearchParameter("Encounter", "discharge-disposition"); ok {
		t.Error("discharge-disposition SP should be filtered out (base Encounter out of scope)")
	}
}

func TestLoadAuCoreWithScopeReferencedGenericResources(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Patient", "Observation").WithGenericResources(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := reg.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	if len(reg.ResourcesForType("Patient")) == 0 {
		t.Error("Patient examples should be indexed")
	}
	if len(reg.ResourcesForType("Observation")) == 0 {
		t.Error("Observation examples should be indexed")
	}
	// MedicationRequest is out of scope.
	if len(reg.ResourcesForType("MedicationRequest")) != 0 {
		t.Error("MedicationRequest examples should be filtered out")
	}
}

func TestLoadAuCoreWithScopeNoneCapabilityStatements(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithCapabilityStatements(ScopeNone)
	if err := reg.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	if len(reg.CapabilityStatements()) != 0 {
		t.Error("CapabilityStatements should be filtered out with ScopeNone")
	}
}

func TestLoadAuCoreWithScopeFromCS_ResolveTerminology(t *testing.T) {
	reg := loadAuCoreAndAuBase(t)
	var responder *CapabilityStatement
	for _, cs := range reg.CapabilityStatements() {
		if cs.URL == "http://hl7.org.au/fhir/core/CapabilityStatement/au-core-responder" {
			responder = cs
		}
	}
	scoped := NewRegistry()
	scoped.Scope = ScopeFromCapabilityStatement(responder)
	if err := scoped.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	if err := scoped.LoadPackageTgz("au-core.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-core: %v", err)
	}
	scoped.Resolve()
	// contact-purpose is referenced by Organization SDs (Organization in scope).
	if _, ok := scoped.ValueSet("http://terminology.hl7.org.au/ValueSet/contact-purpose"); !ok {
		t.Error("contact-purpose ValueSet should be indexed after Resolve")
	}
	// accession-number-type is not referenced by any in-scope SD.
	if _, ok := scoped.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type"); ok {
		t.Error("accession-number-type ValueSet should be filtered out")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestLoadAuCoreResolveIdempotent(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Organization").WithValueSets(ScopeReferenced).WithCodeSystems(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	reg.Resolve()
	reg.Resolve()
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/contact-purpose"); !ok {
		t.Error("contact-purpose ValueSet should be indexed after Resolve")
	}
	if len(reg.pendingValueSets) != 0 || len(reg.pendingCodeSystems) != 0 {
		t.Error("pending buffers should be cleared after Resolve")
	}
}

func TestLoadAuCoreAddValueSetScopeReferenced(t *testing.T) {
	reg := NewRegistry()
	reg.Scope = NewScope().WithResourceTypes("Organization").WithValueSets(ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		t.Fatalf("LoadPackageTgz au-base: %v", err)
	}
	// Manually add a ValueSet referenced by the Organization SD.
	reg.AddValueSet(&ValueSet{URL: "http://terminology.hl7.org.au/ValueSet/contact-purpose", Status: "active"})
	reg.Resolve()
	if _, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/contact-purpose"); !ok {
		t.Error("manually added referenced ValueSet should be indexed after Resolve")
	}
}

// ---------------------------------------------------------------------------
// Directory loading
// ---------------------------------------------------------------------------

func TestLoadAuCoreDirectory(t *testing.T) {
	if _, err := os.Stat("au-core.tgz"); err != nil {
		t.Skip("au-core.tgz not present")
	}
	pkgDir := t.TempDir()
	f, err := os.Open("au-core.tgz")
	if err != nil {
		t.Fatalf("open au-core.tgz: %v", err)
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
	if _, ok := reg.Definition("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient"); !ok {
		t.Error("au-core-patient not indexed from directory")
	}
	if len(reg.CapabilityStatements()) != 2 {
		t.Errorf("CapabilityStatements len = %d, want 2", len(reg.CapabilityStatements()))
	}
	if _, ok := reg.SearchParameter("PractitionerRole", "practitioner"); !ok {
		t.Error("practitionerrole-practitioner SP not indexed from directory")
	}
}

// ---------------------------------------------------------------------------
// Network: full dependency chain resolution
// ---------------------------------------------------------------------------

// TestLoadAuCoreWithDeps resolves the full parent dependency chain from the
// registry. au-core depends on au-base (pinned to "current", which the client
// resolves via dist-tags), which depends on R4 core and terminology.
func TestLoadAuCoreWithDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network dependency resolution in -short mode")
	}
	client, err := NewPackageClient()
	if err != nil {
		t.Fatalf("NewPackageClient: %v", err)
	}
	client.CacheDir = t.TempDir() // isolate cache

	pkgDir := t.TempDir()
	f, err := os.Open("au-core.tgz")
	if err != nil {
		t.Fatalf("open au-core.tgz: %v", err)
	}
	if err := extractTGZToDir(f, pkgDir); err != nil {
		f.Close()
		t.Fatalf("extractTGZToDir: %v", err)
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	reg := NewRegistry()
	if err := reg.LoadPackageWithDeps(ctx, pkgDir, client); err != nil {
		t.Fatalf("LoadPackageWithDeps: %v", err)
	}

	// R4 core base definitions resolved from the dependency chain.
	if _, ok := reg.Definition("http://hl7.org/fhir/StructureDefinition/Patient"); !ok {
		t.Error("base Patient definition not resolved from R4 core")
	}
	if _, ok := reg.Definition("http://hl7.org/fhir/StructureDefinition/Address"); !ok {
		t.Error("base Address definition not resolved from R4 core")
	}
	// au-base profiles (resolved via the "current" dependency).
	if _, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient"); !ok {
		t.Error("au-patient not resolved from au-base dependency")
	}
	// au-core profiles.
	if _, ok := reg.Definition("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient"); !ok {
		t.Error("au-core-patient not loaded")
	}
	// Terminology from hl7.terminology.r4.
	if _, ok := reg.ValueSet("http://hl7.org/fhir/ValueSet/administrative-gender"); !ok {
		t.Error("administrative-gender ValueSet not resolved from terminology dependency")
	}
	// Tree building works across the full chain.
	tree, err := reg.Tree("http://hl7.org.au/fhir/core/StructureDefinition/au-core-patient")
	if err != nil {
		t.Fatalf("Tree(au-core-patient): %v", err)
	}
	if tree.Root == nil || tree.Root.Path != "Patient" {
		t.Errorf("au-core-patient tree root = %+v, want Patient", tree.Root)
	}
	// ResolveType with no profiles now resolves to the R4 base Patient.
	if _, ok := reg.ResolveType("Patient", nil); !ok {
		t.Error("ResolveType(Patient, nil) should resolve to R4 base with full chain")
	}
}
