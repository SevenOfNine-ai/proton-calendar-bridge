package provider

import (
	"errors"
	"testing"
)

func TestNotSupportedError(t *testing.T) {
	err := NotSupportedError{Operation: "x"}
	if err.Error() == "" {
		t.Fatal("expected error text")
	}
	if !errors.Is(err, ErrNotSupported) {
		t.Fatal("expected unwrap to ErrNotSupported")
	}
	if (NotSupportedError{}).Error() != ErrNotSupported.Error() {
		t.Fatal("expected default message")
	}
}
