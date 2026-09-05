# fhir-registry

A Go library for loading, indexing, and reasoning about FHIR structure definitions. It is the single source of truth for FHIR structure knowledge, designed to handle arbitrary implementation guides.

## Features

- **Registry** -  concurrency-safe index of StructureDefinitions by canonical URL and base type, with cached element trees.
- **Element trees** - build recursively-linked trees from snapshots or differentials, with slice handling and path/ID lookup.
- **Differential merging** - `MergeDifferential` overlays a profile's differential onto its base snapshot.
- **Package loading** - load FHIR packages from a directory or `.tgz`, and resolve full dependency chains from a registry server with caching.
- **Marshal** - normalise instance resources against type trees (array/scalar wrapping, choice elements, cardinality checks).
- **Cardinality helpers** - `IsMulti`, `IsRequired`, `Cardinality`, `PrimaryTypeCode`, `IsChoice`, `ChoiceName`.

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

## Key types

- `ElementDefinition` - structural definition with typed `Max` cardinality, `Children` tree, and `Slices` grouping.
- `ElementTree` - `SD`, `Root`, `ByPath`, `ByID`.
- `Max` - int type where `MaxUnbounded` represents `"*"`.
- `StructureDefinition`, `ElementType`, `Binding`, `Slicing`, `Discriminator`, `ElementConstraint`.

## Errors

Sentinel errors (`ErrDefinitionNotFound`, `ErrBaseDefinition`, `ErrIncompleteDefinition`, `ErrPackageNotFound`, `ErrVersionNotFound`, `ErrNetwork`, `ErrParseFailure`) are returned wrapped and usable with `errors.Is`.
