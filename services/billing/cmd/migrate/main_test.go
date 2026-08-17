package main

import "testing"

func TestValidateSteps(t *testing.T) {
	t.Parallel()
	if err := validateSteps(0); err != nil {
		t.Fatalf("validateSteps(0) returned %v", err)
	}
	if err := validateSteps(1); err != nil {
		t.Fatalf("validateSteps(1) returned %v", err)
	}
	if err := validateSteps(-1); err == nil {
		t.Fatal("validateSteps(-1) returned nil")
	}
}
