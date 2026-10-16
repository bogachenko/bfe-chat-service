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

func main() {
	profile := flag.String("profile", "coverage.out", "coverage profile")
	min := flag.Float64("min", 80, "minimum total percent")
	criticalMin := flag.Float64("critical-min", 90, "minimum critical percent")
	flag.Parse()

	f, err := os.Open(*profile)
	if err != nil {
		fail(err.Error())
	}
	defer func() {
		if err := f.Close(); err != nil {
			fail(err.Error())
		}
	}()

	var allTotal, allCovered, criticalTotal, criticalCovered uint64
	var criticalPresent bool
	scan := bufio.NewScanner(f)
	if !scan.Scan() || !strings.HasPrefix(scan.Text(), "mode: ") {
		fail("invalid coverage profile")
	}
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 3 {
			fail("invalid profile row")
		}
		n, e1 := strconv.ParseUint(fields[1], 10, 64)
		count, e2 := strconv.ParseUint(fields[2], 10, 64)
		if e1 != nil || e2 != nil {
			fail("invalid counter")
		}
		allTotal += n
		if count > 0 {
			allCovered += n
		}
		critical := strings.Contains(fields[0], "/internal/auth/") ||
			strings.Contains(fields[0], "/internal/dpop/") ||
			strings.Contains(fields[0], "/internal/access/")
		if critical {
			criticalPresent = true
			criticalTotal += n
			if count > 0 {
				criticalCovered += n
			}
		}
	}
	if err := scan.Err(); err != nil {
		fail(err.Error())
	}
	if allTotal == 0 || !criticalPresent || criticalTotal == 0 {
		fail("coverage not measurable or critical packages missing")
	}
	overall := 100 * float64(allCovered) / float64(allTotal)
	crit := 100 * float64(criticalCovered) / float64(criticalTotal)
	fmt.Printf("Coverage total %.2f%% (min %.2f%%), critical %.2f%% (min %.2f%%)\n", overall, *min, crit, *criticalMin)
	if overall < *min || crit < *criticalMin {
		fail("coverage threshold not met")
	}
}

func fail(reason string) {
	fmt.Fprintln(os.Stderr, "coverage gate:", reason)
	os.Exit(1)
}
