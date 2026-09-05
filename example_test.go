package fhir_test

import (
	"context"
	"fmt"
	"log"

	fhir "github.com/jlcoulter/fhir-registry"
)

// ExampleRegistry demonstrates loading a FHIR package and looking up a
// StructureDefinition by canonical URL.
func ExampleRegistry() {
	reg := fhir.NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		log.Fatal(err)
	}

	sd, ok := reg.Definition("http://hl7.org.au/fhir/StructureDefinition/au-patient")
	if !ok {
		log.Fatal("au-patient not found")
	}
	fmt.Println(sd.Type, sd.Name)
	// Output: Patient AUBasePatient
}

// ExampleRegistry_Tree demonstrates building the element tree for a
// StructureDefinition and walking its children.
func ExampleRegistry_Tree() {
	reg := fhir.NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		log.Fatal(err)
	}

	tree, err := reg.Tree("http://hl7.org.au/fhir/StructureDefinition/au-organization")
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range tree.Root.Children {
		fmt.Println(c.Path, fhir.Cardinality(c))
	}
	// Output:
	// Organization.id 0..1
	// Organization.meta 0..1
	// Organization.implicitRules 0..1
	// Organization.language 0..1
	// Organization.text 0..1
	// Organization.contained 0..*
	// Organization.extension 0..*
	// Organization.modifierExtension 0..*
	// Organization.identifier 0..*
	// Organization.active 0..1
	// Organization.type 0..*
	// Organization.name 0..1
	// Organization.alias 0..*
	// Organization.telecom 0..*
	// Organization.address 0..*
	// Organization.partOf 0..1
	// Organization.contact 0..*
	// Organization.endpoint 0..*
}

// ExampleRegistry_Marshal demonstrates normalizing an instance resource
// against the element tree for a base type.
func ExampleRegistry_Marshal() {
	reg := fhir.NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		log.Fatal(err)
	}

	instance := map[string]any{
		"resourceType": "Organization",
		"name":         "ACME",
		"alias":        "ACME Corp",
	}
	out, report, err := reg.Marshal("Organization", instance)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("name=%v alias=%v items=%d\n", out["name"], out["alias"], len(report.Items))
	// Output: name=ACME alias=[ACME Corp] items=0
}

// ExampleRegistry_LoadPackageWithDeps demonstrates loading a package and
// resolving its full dependency chain from a registry server.
func ExampleRegistry_LoadPackageWithDeps() {
	reg := fhir.NewRegistry()
	client, err := fhir.NewPackageClient()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	if err := reg.LoadPackageWithDeps(ctx, "package", client); err != nil {
		log.Fatal(err)
	}
}

// ExampleScope demonstrates narrowing which resources a Registry indexes.
func ExampleScope() {
	reg := fhir.NewRegistry()
	reg.Scope = fhir.NewScope().
		WithResourceTypes("Patient", "Observation").
		WithSearchParams(fhir.ScopeReferenced)
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		log.Fatal(err)
	}
}

// ExampleScopeFromCapabilityStatement demonstrates deriving a scope from a
// CapabilityStatement.
func ExampleScopeFromCapabilityStatement() {
	reg := fhir.NewRegistry()
	if err := reg.LoadPackageTgz("au-base.tgz"); err != nil {
		log.Fatal(err)
	}
	cs := reg.CapabilityStatements()[0]
	reg.Scope = fhir.ScopeFromCapabilityStatement(cs)
}

// ExampleMergeDifferential demonstrates overlaying a profile's differential
// onto its base snapshot.
func ExampleMergeDifferential() {
	base := []fhir.RawElement{
		{ID: "Patient.name", Path: "Patient.name", Min: intPtr(0), Max: rawMax("1")},
	}
	diff := []fhir.RawElement{
		{ID: "Patient.name", Path: "Patient.name", Min: intPtr(1)},
	}
	merged := fhir.MergeDifferential(base, diff)
	fmt.Println(len(merged), *merged[0].Min)
	// Output: 1 1
}

func intPtr(i int) *int { return &i }

func rawMax(s string) []byte { return []byte(`"` + s + `"`) }
