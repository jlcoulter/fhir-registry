# Contributing to fhir-registry

Thanks for your interest in contributing! This guide gets you set up in under
10 minutes.

## Prerequisites

- Go 1.27 or later (see `go.mod`)
- `git`

## Getting started

```sh
git clone https://github.com/jlcoulter/fhir-registry
cd fhir-registry
go build ./...
```

## Running tests

```sh
go test ./...
```

The test suite uses the bundled FHIR package fixtures (`au-base.tgz`,
`au-core.tgz`) to exercise loading, tree building, marshaling, and
scoping. No network access is required for the unit tests.

## Project layout

The library is a single package, `fhir`, at the repository root:

- `registry.go` — the `Registry`, element trees, and cardinality helpers
- `client.go` — `PackageClient` for downloading packages from a registry
- `resolver.go` — package loading and dependency resolution
- `scope.go` — `Scope` for narrowing what a `Registry` indexes
- `resources.go` — terminology and conformance resource types
- `marshal.go` — instance normalization against element trees
- `merge.go` — differential-to-snapshot merging
- `errors.go` — sentinel errors

## Coding conventions

- Every exported symbol has a doc comment that starts with its name and
  explains *why* and *when* to use it, not just *what* it does.
- New behavior is covered by tests. Prefer table-driven tests, and add
  `ExampleXxx` functions for any new public API so it renders on pkg.go.dev.
- Run `gofmt` and `go vet ./...` before submitting.

## Submitting a change

1. Create a branch off `main`.
2. Make your change, adding tests and updating the README if the public API
   changes.
3. Run `go test ./...` and `go vet ./...`.
4. Open a pull request describing the change and its motivation.

## CI

This project uses GitHub Actions for continuous integration. Every push and
pull request runs:

- `go build ./...`
- `go vet ./...`
- `go test ./...` with a coverage gate (80%)
- `gofmt -l .` (format check)

Releases are automated via Release Please. When a PR is merged with a
conventional-commit message, Release Please opens a release PR. Merging that
PR triggers a tagged release with an auto-generated changelog from git-cliff.
