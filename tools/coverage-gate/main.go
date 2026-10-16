// Command coverage-gate enforces code coverage thresholds with explicit critical scope.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type coverageCounts struct {
	allTotal        uint64
	allCovered      uint64
	criticalTotal   uint64
	criticalCovered uint64
	criticalPresent bool
}

func main() {
	profile := flag.String("profile", "coverage.out", "coverage profile")
	min := flag.Float64("min", 80, "minimum total percent")
	criticalMin := flag.Float64("critical-min", 90, "minimum critical percent")
	flag.Parse()

	counts := readCoverage(*profile)
	if counts.allTotal == 0 || !counts.criticalPresent || counts.criticalTotal == 0 {
		fail("coverage not measurable or critical packages missing")
	}
	overall := 100 * float64(counts.allCovered) / float64(counts.allTotal)
	crit := 100 * float64(counts.criticalCovered) / float64(counts.criticalTotal)
	fmt.Printf("Coverage total %.2f%% (min %.2f%%), critical %.2f%% (min %.2f%%)\n", overall, *min, crit, *criticalMin)
	if overall < *min || crit < *criticalMin {
		fail("coverage threshold not met")
	}
}

func readCoverage(profile string) coverageCounts {
	f, err := os.Open(profile)
	if err != nil {
		fail(err.Error())
	}
	defer func() {
		if err := f.Close(); err != nil {
			fail(err.Error())
		}
	}()

	scan := bufio.NewScanner(f)
	if !scan.Scan() || !strings.HasPrefix(scan.Text(), "mode: ") {
		fail("invalid coverage profile")
	}
	var counts coverageCounts
	for scan.Scan() {
		counts.addRow(scan.Text())
	}
	if err := scan.Err(); err != nil {
		fail(err.Error())
	}
	return counts
}

func (c *coverageCounts) addRow(row string) {
	fields := strings.Fields(row)
	if len(fields) != 3 {
		fail("invalid profile row")
	}
	n, e1 := strconv.ParseUint(fields[1], 10, 64)
	count, e2 := strconv.ParseUint(fields[2], 10, 64)
	if e1 != nil || e2 != nil {
		fail("invalid counter")
	}
	c.allTotal += n
	if count > 0 {
		c.allCovered += n
	}
	if isCriticalPackage(fields[0]) {
		c.criticalPresent = true
		c.criticalTotal += n
		if count > 0 {
			c.criticalCovered += n
		}
	}
}

func isCriticalPackage(path string) bool {
	return strings.Contains(path, "/internal/auth/") ||
		strings.Contains(path, "/internal/dpop/") ||
		strings.Contains(path, "/internal/access/")
}

func fail(reason string) {
	fmt.Fprintln(os.Stderr, "coverage gate:", reason)
	os.Exit(1)
}
