package main

import (
	"bytes"
	"fmt"
	"errors"
	"io"
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

func TestCoverageGateZeroStatementBlock(t *testing.T) {
	data := profile(true) + module + "internal/authentication/empty.go:1.1,1.1 0 0\n"
	byPackage, err := parseProfile(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(byPackage, 80, 90, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageGateMalformedProfile(t *testing.T) {
	for _, data := range []string{
		"not a header\n",
		"mode: atomic\ninvalid row\n",
		"mode: atomic\n" + module + "internal/authentication/a.go:1.1,2.1 nope 1\n",
		"mode: atomic\n" + module + "tools/coverage-gate/main.go:1.1,2.1 1 1\n",
		"mode: atomic\n" + module + "internal/authentication/a.go:1.1,2.1 invalid 1\n",
	} {
		if _, err := parseProfile(strings.NewReader(data)); err == nil {
			t.Fatalf("malformed profile accepted: %q", data)
		}
	}
}

func TestCoverageGateDeduplicatesBlocksAcrossTestBinaries(t *testing.T) {
	base := profile(true)
	// A second package's test binary includes the same blocks with zero hits.
	var duplicates strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(base), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("bad fixture: %q", line)
		}
		fmt.Fprintf(&duplicates, "%s %s 0\n", fields[0], fields[1])
	}
	withDuplicates := base + duplicates.String()
	counts, err := parseProfile(strings.NewReader(withDuplicates))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(counts, 80, 90, &bytes.Buffer{}); err != nil {
		t.Fatalf("zero-hit copies reduced coverage: %v", err)
	}

	// Reverse order: zero-hit records before the covered copies.
	first := "mode: atomic\n" + duplicates.String() + strings.TrimPrefix(base, "mode: atomic\n")
	counts, err = parseProfile(strings.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if err := evaluate(counts, 80, 90, &bytes.Buffer{}); err != nil {
		t.Fatalf("covered copies did not raise coverage: %v", err)
	}
}

type failingReportWriter struct {
	remaining int
	err       error
}

func (w *failingReportWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, w.err
	}
	w.remaining--
	return len(p), nil
}

func TestCoverageReportWriteErrors(t *testing.T) {
	all, err := parseProfile(strings.NewReader(profile(true)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		successes int
	}{
		{name: "package row", successes: 0},
		{name: "production summary", successes: len(production)},
		{name: "critical row", successes: len(production) + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeErr := errors.New("report sink failed")
			out := &failingReportWriter{remaining: tc.successes, err: writeErr}
			if err := evaluate(all, 80, 90, out); !errors.Is(err, writeErr) {
				t.Fatalf("evaluate error = %v; want wrapped write error", err)
			}
		})
	}
}

var _ io.Writer = (*failingReportWriter)(nil)
