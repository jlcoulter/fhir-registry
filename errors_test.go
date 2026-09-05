package fhir

import (
	"errors"
	"testing"
)

// TestErrorSentinels verifies that the exported sentinel errors are distinct
// and usable with errors.Is.
func TestErrorSentinels(t *testing.T) {
	sentinels := []error{
		ErrDefinitionNotFound,
		ErrBaseDefinition,
		ErrIncompleteDefinition,
		ErrPackageNotFound,
		ErrVersionNotFound,
		ErrNetwork,
		ErrParseFailure,
	}
	seen := make(map[string]bool)
	for _, s := range sentinels {
		if s == nil {
			t.Error("sentinel error is nil")
		}
		if seen[s.Error()] {
			t.Errorf("duplicate sentinel error message: %q", s.Error())
		}
		seen[s.Error()] = true
	}
}

// TestErrorWrapping verifies that wrapped errors still match their sentinel
// via errors.Is.
func TestErrorWrapping(t *testing.T) {
	wrapped := errors.Join(ErrDefinitionNotFound, errors.New("detail"))
	if !errors.Is(wrapped, ErrDefinitionNotFound) {
		t.Error("errors.Is(wrapped, ErrDefinitionNotFound) = false, want true")
	}
	if errors.Is(wrapped, ErrNetwork) {
		t.Error("errors.Is(wrapped, ErrNetwork) = true, want false")
	}
}

// TestLoadPackageNotFound verifies that loading a missing directory surfaces
// ErrPackageNotFound.
func TestLoadPackageNotFound(t *testing.T) {
	reg := NewRegistry()
	err := reg.LoadPackage("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing package dir")
	}
	if !errors.Is(err, ErrPackageNotFound) {
		t.Errorf("errors.Is(err, ErrPackageNotFound) = false, got %v", err)
	}
}

// TestTreeDefinitionNotFound verifies that Tree on an unknown URL surfaces
// ErrDefinitionNotFound.
func TestTreeDefinitionNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Tree("http://example.org/StructureDefinition/Unknown")
	if err == nil {
		t.Fatal("expected error for unknown definition")
	}
	if !errors.Is(err, ErrDefinitionNotFound) {
		t.Errorf("errors.Is(err, ErrDefinitionNotFound) = false, got %v", err)
	}
}
