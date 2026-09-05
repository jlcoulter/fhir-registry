# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and
this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `Registry` — concurrency-safe index of `StructureDefinition`s by canonical
  URL and base type, with cached element trees.
- Element trees — recursively-linked trees built from snapshots or
  differentials, with slice handling and path/ID lookup.
- `MergeDifferential` — overlays a profile's differential onto its base
  snapshot.
- Package loading — load FHIR packages from a directory or `.tgz`, and resolve
  full dependency chains from a registry server with caching via
  `PackageClient`.
- `Marshal` — normalizes instance resources against type trees (array/scalar
  wrapping, choice elements, cardinality checks).
- Cardinality helpers — `IsMulti`, `IsRequired`, `Cardinality`,
  `PrimaryTypeCode`, `IsChoice`, `ChoiceName`.
- Terminology and conformance resources — index `ValueSet`s, `CodeSystem`s,
  `CapabilityStatement`s, and `SearchParameter`s from packages, plus any other
  resource type as an opaque `Resource`.
- Scoped registry — `Scope` narrows which resources a `Registry` indexes,
  e.g. to only what a package's `CapabilityStatement` declares as supported.
- Sentinel errors — `ErrDefinitionNotFound`, `ErrBaseDefinition`,
  `ErrIncompleteDefinition`, `ErrPackageNotFound`, `ErrVersionNotFound`,
  `ErrNetwork`, `ErrParseFailure`.
