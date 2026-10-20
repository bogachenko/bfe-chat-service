// Independent QA regression tests for QR-08 production coverage scope.
// These tests deliberately fail until internal/issue25 is included in coverage-gate.
package main

import (
    "strings"
    "testing"
)

func TestIndependentIssue25IsProduction(t *testing.T) {
    if !knownProductionPackage("internal/issue25") {
        t.Fatal("QR-08: internal/issue25 is production Go code but coverage-gate excludes it")
    }
}

func TestIndependentIssue25CoverageProfileAccepted(t *testing.T) {
    row := module + "internal/issue25/logic.go:1.1,1.6 1 1"
    if _, _, _, _, err := parseCoverageRow(row); err != nil {
        t.Fatalf("QR-08: coverage-gate rejects Issue #25 production coverage: %v", err)
    }
}

func TestIndependentUnknownProductionMustNotBeSilentlySkipped(t *testing.T) {
    profile := "mode: atomic\n" +
        module + "internal/issue25/logic.go:1.1,1.6 1 0\n"
    if _, err := parseProfile(strings.NewReader(profile)); err != nil {
        t.Fatalf("QR-08: production profile fails because Issue #25 is excluded: %v", err)
    }
}
