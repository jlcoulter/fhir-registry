package fhir

import "errors"

// Sentinel errors returned by the library. Callers can distinguish failure
// classes with errors.Is, e.g. errors.Is(err, fhir.ErrPackageNotFound).
var (
	// ErrDefinitionNotFound is returned when a StructureDefinition cannot be
	// found for a canonical URL or base type name.
	ErrDefinitionNotFound = errors.New("fhir: structure definition not found")

	// ErrBaseDefinition is returned when a profile's base definition cannot be
	// resolved from the registry.
	ErrBaseDefinition = errors.New("fhir: cannot resolve base definition")

	// ErrIncompleteDefinition is returned when a StructureDefinition has
	// neither a usable snapshot nor a differential.
	ErrIncompleteDefinition = errors.New("fhir: structure definition has no snapshot or differential")

	// ErrPackageNotFound is returned when a package directory or tarball
	// cannot be located or read.
	ErrPackageNotFound = errors.New("fhir: package not found")

	// ErrVersionNotFound is returned when no available version matches a
	// version reference.
	ErrVersionNotFound = errors.New("fhir: version not found")

	// ErrNetwork is returned when a registry or tarball request fails.
	ErrNetwork = errors.New("fhir: network error")

	// ErrParseFailure is returned when a FHIR resource cannot be parsed.
	ErrParseFailure = errors.New("fhir: parse failure")

	// ErrPathNotFound is returned when an element path cannot be resolved
	// against an element tree.
	ErrPathNotFound = errors.New("fhir: element path not found")
)
