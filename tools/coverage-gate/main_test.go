package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func profile(covered bool) string {
	var out strings.Builder
	out.WriteString("mode: atomic\n")
	for _, pkg := range production {
		hit := 1
		if !covered {
			hit = 0
		}
		fmt.Fprintf(&out, "%s%s/example.go:1.1,2.1 1 %d\n", module, pkg, hit)
	}
	return out.String()
}

func TestCoverageGateValid(t *testing.T) {
	got, err := parseProfile(strings.NewReader(profile(true)))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(got, 80, 90, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageGateEmptyProfile(t *testing.T) {
	for _, data := range []string{"", "mode: atomic\n"} {
		if _, err := parseProfile(strings.NewReader(data)); err == nil {
			t.Fatal("empty profile accepted")
		}
	}
}

func TestCoverageGateMissingCriticalPackage(t *testing.T) {
	data := strings.ReplaceAll(profile(true), module+"internal/authorization/example.go:1.1,2.1 1 1\n", "")
	byPackage, err := parseProfile(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(byPackage, 80, 90, &bytes.Buffer{}); err == nil {
		t.Fatal("missing critical package accepted")
	}
}

func TestCoverageGateInsufficientCoverage(t *testing.T) {
	byPackage, err := parseProfile(strings.NewReader(profile(false)))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(byPackage, 80, 90, &bytes.Buffer{}); err == nil {
		t.Fatal("zero coverage accepted")
	}
	data := strings.ReplaceAll(profile(true), module+"internal/authentication/example.go:1.1,2.1 1 1", module+"internal/authentication/example.go:1.1,2.1 1 0")
	byPackage, err = parseProfile(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(byPackage, 80, 90, &bytes.Buffer{}); err == nil {
		t.Fatal("critical package under 90% accepted")
	}
}

func TestCoverageGateMalformedProfile(t *testing.T) {
	for _, data := range []string{
		"not a header\n",
		"mode: atomic\ninvalid row\n",
		"mode: atomic\n" + module + "internal/authentication/a.go:1.1,2.1 nope 1\n",
		"mode: atomic\n" + module + "tools/coverage-gate/main.go:1.1,2.1 1 1\n",
		"mode: atomic\n" + module + "internal/authentication/a.go:1.1,2.1 0 1\n",
	} {
		if _, err := parseProfile(strings.NewReader(data)); err == nil {
			t.Fatalf("malformed profile accepted: %q", data)
		}
	}
}
