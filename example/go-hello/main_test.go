package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestGreet(t *testing.T) {
	got := greet("pvm")
	if !strings.Contains(got, "pvm") {
		t.Fatalf("greet() = %q", got)
	}
	if !strings.Contains(got, runtime.GOOS) {
		t.Fatalf("greet() should include platform: %q", got)
	}
}
