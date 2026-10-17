#!/usr/bin/env python3
"""Staged coverage reporting; missing critical modules remain blocking."""
import argparse
from pathlib import Path

def read(path):
    lines = Path(path).read_text().splitlines()
    if not lines or not lines[0].startswith("mode: "):
        raise ValueError("invalid coverage profile")
    rows = []
    for line in lines[1:]:
        items = line.split()
        if len(items) != 3:
            raise ValueError("invalid profile row")
        rows.append((items[0].split(":", 1)[0], int(items[1]), int(items[2]) > 0))
    if not rows:
        raise ValueError("empty profile")
    return rows

def percent(rows):
    size = sum(n for _, n, _ in rows)
    if size == 0:
        raise ValueError("no executable statements")
    return 100.0 * sum(n for _, n, hit in rows if hit) / size

def main():
    p = argparse.ArgumentParser()
    p.add_argument("--module-profile", required=True)
    p.add_argument("--project-profile", required=True)
    p.add_argument("--output", default="coverage-report.md")
    a = p.parse_args()
    module = [r for r in read(a.module_profile) if "/internal/storage/" in r[0]]
    project = [r for r in read(a.project_profile) if "/cmd/bfe-chat-service/" in r[0] or "/internal/" in r[0]]
    module_pc, project_pc = percent(module), percent(project)
    critical = []
    for name in ("authentication", "authorization"):
        folder = Path("internal") / name
        relevant = [r for r in project if "/internal/" + name + "/" in r[0]]
        if not folder.is_dir() or not list(folder.glob("*.go")):
            status = "NOT IMPLEMENTED"
        elif not relevant:
            status = "NOT MEASURABLE"
        else:
            pc = percent(relevant)
            status = "%.2f%% %s" % (pc, "PASS" if pc >= 90 else "FAIL")
        critical.append((name, status))
    module_pass = module_pc >= 80
    cmd_present = any("/cmd/bfe-chat-service/" in r[0] for r in project)
    project_pass = cmd_present and project_pc >= 80 and all(status.endswith("PASS") for _, status in critical)
    lines = [
        "# Staged Coverage Report",
        "",
        "Source: Go tests with -tags=integration (unit + integration).",
        "Statement coverage does not prove semantic coverage.",
        "",
        "## Module Coverage — DP-M07 (internal/storage)",
        "- Statements: %.2f%%; threshold: 80%%." % module_pc,
        "- Module result: " + ("PASS" if module_pass else "FAIL"),
        "",
        "## Project Coverage — Production Quality Gate",
        "- Statements: %.2f%%; threshold: 80%%." % project_pc,
        "- Critical modules require at least 90%.",
    ]
    lines.extend("- internal/%s: %s" % value for value in critical)
    lines.extend([
        "- cmd/bfe-chat-service: " + ("INCLUDED" if cmd_present else "MISSING"),
        "- Production Quality Gate: " + ("PASS" if project_pass else "BLOCKED"),
        "",
        "Missing critical packages or cmd entrypoint are BLOCKING, never considered verified.",
        "Production quality thresholds are unchanged and blocking.",
        "",
    ])
    output = "\n".join(lines)
    Path(a.output).write_text(output)
    print(output)
    if not module_pass or not project_pass:
        raise SystemExit(1)

if __name__ == "__main__":
    main()
