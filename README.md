# fhir-registry

A Go library for loading, indexing, and reasoning about FHIR structure definitions. It is the single source of truth for FHIR structure knowledge, designed to handle arbitrary implementation guides.

## Features

- **Registry** -  concurrency-safe index of StructureDefinitions by canonical URL and base type, with cached element trees.
- **Element trees** - build recursively-linked trees from snapshots or differentials, with slice handling and path/ID lookup.
- **Differential merging** - `MergeDifferential` overlays a profile's differential onto its base snapshot.
- **Package loading** - load FHIR packages from a directory or `.tgz`, and resolve full dependency chains from a registry server with caching.
- **Marshal** - normalise instance resources against type trees (array/scalar wrapping, choice elements, cardinality checks).
- **Cardinality helpers** - `IsMulti`, `IsRequired`, `Cardinality`, `PrimaryTypeCode`, `IsChoice`, `ChoiceName`.
- **Terminology & conformance resources** - index ValueSets, CodeSystems, CapabilityStatements, and SearchParameters from packages, plus any other resource type as an opaque `Resource`.

## Usage

```go
reg := fhir.NewRegistry()
if err := reg.LoadPackage("package"); err != nil {
    log.Fatal(err)
}

tree, err := reg.Tree("http://hl7.org/fhir/StructureDefinition/Patient")
if err != nil {
    log.Fatal(err)
}
// tree.Root, tree.ByPath, tree.ByID
```

Load a package with dependencies:

```go
client, _ := fhir.NewPackageClient()
ctx := context.Background()
if err := reg.LoadPackageWithDeps(ctx, "package", client); err != nil {
    log.Fatal(err)
}
```

Normalise an instance:

```go
out, report, err := reg.Marshal("Patient", instance)
```

## Terminology & conformance resources

Loading a package indexes more than StructureDefinitions. ValueSets and
CodeSystems are keyed by canonical URL, SearchParameters by `(base, code)`,
CapabilityStatements are collected, and any other resource type is stored as
an opaque `Resource` keyed by its `resourceType`.

```go
// ValueSet / CodeSystem lookup by canonical URL.
vs, ok := reg.ValueSet("http://terminology.hl7.org.au/ValueSet/accession-number-type")
cs, ok := reg.CodeSystem("http://terminology.hl7.org.au/CodeSystem/contact-purpose")

// SearchParameter lookup by (base resource type, code).
sp, ok := reg.SearchParameter("Patient", "indigenous-status")

// CapabilityStatements and SearchParameters are iterable.
for _, cs := range reg.CapabilityStatements() { /* ... */ }
for _, sp := range reg.SearchParameters() { /* ... */ }

// Any other resource is kept opaque, with meta.profile extracted.
for _, res := range reg.ResourcesForType("Patient") {
    // res.ResourceType, res.ProfileURLs, res.Raw (full decoded JSON)
}
all := reg.AllResources() // sorted by resource type
```

Each type also has a matching `Add*` method for programmatic registration
(`AddValueSet`, `AddCodeSystem`, `AddCapabilityStatement`, `AddSearchParameter`,
`AddResource`).

### Semantics

- **Last write wins** for URL-keyed lookups (`ValueSet`, `CodeSystem`) and for
  `SearchParameter(base, code)` when two parameters share the same key. The
  superseded entries remain in the iterable collections.
- **Empty canonical URLs** are skipped for ValueSet and CodeSystem.
- **Malformed JSON** is silently ignored, consistent with StructureDefinition
  handling, so a bad file does not abort a package load.
- **`AllResources()`** returns resources ordered by `resourceType` for
  deterministic iteration.

## Key types

- `ElementDefinition` - structural definition with typed `Max` cardinality, `Children` tree, and `Slices` grouping.
- `ElementTree` - `SD`, `Root`, `ByPath`, `ByID`.
- `Max` - int type where `MaxUnbounded` represents `"*"`.
- `StructureDefinition`, `ElementType`, `Binding`, `Slicing`, `Discriminator`, `ElementConstraint`.
- `ValueSet`, `CodeSystem`, `CapabilityStatement`, `SearchParameter` - typed terminology/conformance resources.
- `Resource` - opaque generic resource (`ResourceType`, `ProfileURLs`, `Raw`).

## Errors

Sentinel errors (`ErrDefinitionNotFound`, `ErrBaseDefinition`, `ErrIncompleteDefinition`, `ErrPackageNotFound`, `ErrVersionNotFound`, `ErrNetwork`, `ErrParseFailure`) are returned wrapped and usable with `errors.Is`.
