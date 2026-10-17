// Command coverage-gate enforces QR-08 production statement coverage.
// Scope is explicit: cmd/bfe-chat-service and all eight internal packages.
// qa/driveauthprobe and tools/coverage-gate are verified separately.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const module = "github.com/bogachenko/bfe-chat-service/"

var production = []string{
	"cmd/bfe-chat-service",
	"internal/authentication",
	"internal/assignments",
	"internal/authorization",
	"internal/chat",
	"internal/conversations",
	"internal/documentcontext",
	"internal/realtime",
	"internal/storage",
}

var critical = []string{"internal/authentication", "internal/authorization"}

var block = regexp.MustCompile(`^(.+):[0-9]+\.[0-9]+,[0-9]+\.[0-9]+$`)

type counts struct{ total, hit uint64 }

func (c counts) percent() float64 {
	if c.total == 0 {
		return 0
	}
	return 100 * float64(c.hit) / float64(c.total)
}

func main() {
	profile := flag.String("profile", "coverage.out", "production coverage profile")
	min := flag.Float64("min", 80, "minimum production percent")
	criticalMin := flag.Float64("critical-min", 90, "minimum percent for each critical package")
	flag.Parse()

	f, err := os.Open(*profile)
	if err != nil {
		fail(err)
	}
	defer func() { _ = f.Close() }()
	byPackage, err := parseProfile(f)
	if err != nil {
		fail(err)
	}
	if err := evaluate(byPackage, *min, *criticalMin, os.Stdout); err != nil {
		fail(err)
	}
}

func parseProfile(r io.Reader) (map[string]counts, error) {
	scan := bufio.NewScanner(r)
	if !scan.Scan() {
		return nil, errors.New("empty coverage profile")
	}
	switch scan.Text() {
	case "mode: set", "mode: count", "mode: atomic":
	default:
		return nil, errors.New("invalid coverage profile header")
	}
	byPackage := make(map[string]counts)
	known := make(map[string]bool, len(production))
	for _, pkg := range production {
		known[pkg] = true
	}
	rows := 0
	seen := make(map[string]uint64)
	covered := make(map[string]bool)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 3 {
			return nil, errors.New("invalid profile row")
		}
		match := block.FindStringSubmatch(fields[0])
		if match == nil || !strings.HasPrefix(match[1], module) {
			return nil, errors.New("invalid production file path")
		}
		path := strings.TrimPrefix(match[1], module)
		last := strings.LastIndexByte(path, '/')
		if last < 0 || !strings.HasSuffix(path, ".go") || !known[path[:last]] {
			return nil, fmt.Errorf("unexpected production package: %s", path)
		}
		n, errN := strconv.ParseUint(fields[1], 10, 64)
		hits, errHits := strconv.ParseUint(fields[2], 10, 64)
		if errN != nil || errHits != nil {
			return nil, errors.New("invalid statement counter")
		}
		// -coverpkg produces one profile entry per test binary. Identical
		// source blocks must be counted once, covered if any run hit them.
		key := fields[0]
		prior, exists := seen[key]
		if exists {
			if prior != n {
				return nil, errors.New("inconsistent repeated coverage block")
			}
			if hits > 0 {
				c := byPackage[path[:last]]
				if !covered[key] {
					c.hit += n
					covered[key] = true
					byPackage[path[:last]] = c
				}
			}
			continue
		}
		seen[key] = n
		pkg := path[:last]
		c := byPackage[pkg]
		c.total += n
		if hits > 0 {
			c.hit += n
			covered[key] = true
		}
		byPackage[pkg] = c
		rows++
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, errors.New("empty coverage profile")
	}
	return byPackage, nil
}

func evaluate(byPackage map[string]counts, minimum, criticalMinimum float64, out io.Writer) error {
	total := counts{}
	for _, pkg := range production {
		c := byPackage[pkg]
		if c.total == 0 {
			return fmt.Errorf("missing production package: %s", pkg)
		}
		total.total += c.total
		total.hit += c.hit
		if _, err := fmt.Fprintf(out, "%s: %.2f%%\n", pkg, c.percent()); err != nil {
			return fmt.Errorf("write package coverage: %w", err)
		}
	}
	if _, err := fmt.Fprintf(out, "Production coverage: %.2f%% (minimum %.2f%%)\n", total.percent(), minimum); err != nil {
		return fmt.Errorf("write production coverage: %w", err)
	}
	if total.percent() < minimum {
		return errors.New("production coverage below threshold")
	}
	for _, pkg := range critical {
		c := byPackage[pkg]
		if _, err := fmt.Fprintf(out, "Critical %s: %.2f%% (minimum %.2f%%)\n", pkg, c.percent(), criticalMinimum); err != nil {
			return fmt.Errorf("write critical coverage: %w", err)
		}
		if c.percent() < criticalMinimum {
			return fmt.Errorf("critical package below threshold: %s", pkg)
		}
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "coverage gate:", err)
	os.Exit(1)
}
