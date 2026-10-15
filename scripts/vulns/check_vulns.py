#!/usr/bin/env python3
"""Gate one dependency ecosystem's, or the built image's, known vulnerabilities against the
exception register.

What this asserts is docs/CI.md § *Dependency vulnerabilities*, § *Image vulnerabilities* and
§ *The exception register*. Standard library only, like every other plain-python check in the tree.

`--scope go` runs `go -C <go-dir> tool govulncheck -json ./...`: a finding whose trace reaches a
symbol fails, and a finding reported only at package or module level (the vulnerable code is present
but never called) prints as informational — the one reachability allowance CI.md's Go paragraph
states. `--scope npm` runs `npm --prefix <npm-dir> audit --json` over the whole tree, no
`--audit-level`: every advisory fails, at any severity. `--scope image` runs Trivy, from the pinned
image below, against the tag named by `--image`, scanning the local Docker daemon's copy at every
severity: every reported vulnerability fails, an empty or missing `FixedVersion` changes nothing, and
nothing in the image is first-party, so a register entry is always eligible relief.

Pass/fail is decided from the parsed JSON alone; no scanner's own exit status is read; `-json` mode is
documented to exit 0 whatever govulncheck finds, and this script holds every scope to the one rule
rather than trusting an exit code no scanner here can promise.

The register (`--register`, default `.vulnerability-exceptions.json` at the repository root) is a
JSON list; each entry names exactly five fields — `advisory`, `scope`, `no_fix_because`,
`no_alternative_because`, `review_by` — documented in docs/CI.md § *The exception register*. An
entry's `advisory` matches a finding by that finding's primary id or any alias (a GO- id's GHSA/CVE
aliases; an npm advisory's GHSA id, read from its `via[]` entry's advisory URL; a Trivy finding's own
`VulnerabilityID`, which carries no alias). Only entries whose own `scope` equals the scope being
checked are examined by a given run — the other scopes' entries are that scope's own run to validate —
so a full validation of the whole register needs every `--scope` value run. An entry whose `scope` is
missing or is not one of `go`, `npm` or `image` cannot be routed to any run, so it fails all of them
rather than falling through unexamined. Within its scope: every entry must be well-formed and current
(today <= review_by <=
today+90 days, UTC); an entry matching nothing this run's scanner reported is an orphan; an entry
matching a finding in the scanned project's own package (a Go package path under this module, or the
npm project's own package name) is refused regardless of currency, because first-party code has no
exception path; likewise a Go standard-library finding, which the register never covers at all
(owner ruling, ticket #264) — the remedy is always the Go toolchain bump. The complete finding list
is always printed, suppressed findings included.

Usage: check_vulns.py --scope go|npm|image [--go-dir DIR] [--npm-dir DIR] [--image TAG]
[--register FILE]

What this has been run against, in both directions: ../cases/check-vulns-go.md,
../cases/check-vulns-npm.md and ../cases/check-vulns-image.md
"""

import argparse
import json
import re
import subprocess
import sys
from datetime import date, timedelta
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent

REGISTER_FIELDS = ("advisory", "scope", "no_fix_because", "no_alternative_because", "review_by")
VALID_SCOPES = ("go", "npm", "image")
REVIEW_WINDOW_DAYS = 90

TRIVY_IMAGE = "aquasec/trivy:0.75.0@sha256:af6acf9a6b85dfe389a1941505c0ce9efef52a4719635e1a962f022a3d855daa"

GHSA_IN_URL = re.compile(r"(GHSA-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4})")


def die(message):
    print(f"check-vulns: {message}", file=sys.stderr)
    sys.exit(1)


def load_register(path):
    """The raw JSON list at `path`. A register that cannot be read or does not parse as a JSON list
    is a failure, never an empty register — a check that could not ask the question must not report
    the clean answer."""
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as error:
        die(f"register {path} could not be read: {error}")
    try:
        data = json.loads(text)
    except json.JSONDecodeError as error:
        die(f"register {path} does not parse as JSON: {error}")
    if not isinstance(data, list):
        die(f"register {path} must be a JSON list at the top level, got {type(data).__name__}")
    return data


def validate_entry(entry, index):
    """(well_formed, problems) for one raw register entry. A structurally broken entry (wrong type,
    missing or extra field) is reported and never examined further — reaching the value checks below
    would report on a field that only conditionally exists."""
    label = f"register entry #{index}"
    if not isinstance(entry, dict):
        return False, [f"{label} is not a JSON object"]

    keys = set(entry.keys())
    wanted = set(REGISTER_FIELDS)
    if keys != wanted:
        problems = []
        missing = sorted(wanted - keys)
        extra = sorted(keys - wanted)
        if missing:
            problems.append(f"{label} is missing field(s): {', '.join(missing)}")
        if extra:
            problems.append(f"{label} carries unexpected field(s): {', '.join(extra)}")
        return False, problems

    label = f"{label} ('{entry['advisory']}')" if isinstance(entry["advisory"], str) else label
    problems = []
    for field in ("advisory", "no_fix_because", "no_alternative_because"):
        if not isinstance(entry[field], str) or not entry[field].strip():
            problems.append(f"{label}: '{field}' must be a non-empty string")
    if not isinstance(entry["scope"], str) or entry["scope"] not in VALID_SCOPES:
        problems.append(f"{label}: 'scope' must be one of {VALID_SCOPES}, got {entry['scope']!r}")
    if not isinstance(entry["review_by"], str):
        problems.append(f"{label}: 'review_by' must be a string")
    else:
        try:
            date.fromisoformat(entry["review_by"])
        except ValueError:
            problems.append(f"{label}: 'review_by' ({entry['review_by']!r}) is not an ISO 8601 date")

    return (len(problems) == 0), problems


def currency_problem(entry, today):
    """None, or the reason `entry` (already well-formed) is not a current exception today."""
    review_by = date.fromisoformat(entry["review_by"])
    label = f"register entry ('{entry['advisory']}')"
    if review_by < today:
        return f"{label}: review_by {review_by} has passed (today is {today})"
    limit = today + timedelta(days=REVIEW_WINDOW_DAYS)
    if review_by > limit:
        return f"{label}: review_by {review_by} is more than {REVIEW_WINDOW_DAYS} days out (limit {limit})"
    return None


def matches(entry, primary_id, aliases):
    return entry["advisory"] == primary_id or entry["advisory"] in aliases


def readable_scope(raw):
    """The entry's own `scope` field, or None if `raw` carries nothing of that shape to read (not an
    object, no `scope` key, or a `scope` that is not a string) — read defensively, and read *before*
    `validate_entry`, so an entry can be routed to the scope that owns it without first assuming it
    is well-formed at all."""
    if isinstance(raw, dict) and isinstance(raw.get("scope"), str):
        return raw["scope"]
    return None


def read_go_module(go_dir):
    text = (go_dir / "go.mod").read_text(encoding="utf-8")
    match = re.search(r"(?m)^module\s+(\S+)", text)
    if not match:
        die(f"{go_dir / 'go.mod'} declares no module path")
    return match.group(1)


def govulncheck_objects(go_dir):
    """Every top-level JSON object govulncheck's `-json` output concatenates. The stream is not
    JSON Lines — each message is pretty-printed and may span many lines — so it is decoded by
    repeated `raw_decode` rather than split on newlines."""
    result = subprocess.run(
        ["go", "-C", str(go_dir), "tool", "govulncheck", "-json", "./..."],
        capture_output=True,
        text=True,
    )
    decoder = json.JSONDecoder()
    text = result.stdout
    index = 0
    objects = []
    while index < len(text):
        while index < len(text) and text[index].isspace():
            index += 1
        if index >= len(text):
            break
        try:
            obj, end = decoder.raw_decode(text, index)
        except json.JSONDecodeError as error:
            die(
                f"govulncheck -json output does not parse at offset {index}: {error}\n"
                f"stderr:\n{result.stderr}"
            )
        objects.append(obj)
        index = end
    if not any("config" in obj for obj in objects):
        # No `config` message at all (a missing tool directive, a build failure) means the run never
        # got far enough to scan anything, which must not read as an empty, clean population.
        die(f"govulncheck produced no scan output:\nstderr:\n{result.stderr}")
    return objects


def go_findings(go_dir):
    """One record per OSV id govulncheck reported: {osv, aliases, first_party, reachable}."""
    objects = govulncheck_objects(go_dir)
    module = read_go_module(go_dir)

    osvs = {}
    for obj in objects:
        if "osv" in obj:
            osvs[obj["osv"]["id"]] = obj["osv"]

    by_osv = {}
    for obj in objects:
        finding = obj.get("finding")
        if finding is None:
            continue
        by_osv.setdefault(finding["osv"], []).append(finding)

    records = []
    for osv_id, findings in by_osv.items():
        osv = osvs.get(osv_id, {})
        aliases = osv.get("aliases", [])
        packages = [
            affected.get("package", {}).get("name", "")
            for affected in osv.get("affected", [])
        ]
        first_party = any(name == module or name.startswith(module + "/") for name in packages)
        # docs/CI.md § *Dependency vulnerabilities*: govulncheck's own JSON names the standard
        # library's package exactly "stdlib".
        stdlib = "stdlib" in packages
        reachable = any(len(finding.get("trace", [])) > 1 for finding in findings)
        records.append(
            {
                "primary_id": osv_id,
                "aliases": aliases,
                "package": ", ".join(packages) or "(unknown)",
                "severity": None,
                "first_party": first_party,
                "stdlib": stdlib,
                "reachable": reachable,
            }
        )
    return records


def read_npm_package_name(npm_dir):
    data = json.loads((npm_dir / "package.json").read_text(encoding="utf-8"))
    name = data.get("name")
    if not isinstance(name, str) or not name:
        die(f"{npm_dir / 'package.json'} names no package")
    return name


def npm_findings(npm_dir):
    """One record per (package, advisory-url) `npm audit --json` reports. A `via[]` entry that is a
    string names another affected package on the dependency chain rather than an advisory, and is
    skipped: only a `via[]` object carries an advisory of its own."""
    result = subprocess.run(
        ["npm", "--prefix", str(npm_dir), "audit", "--json"],
        capture_output=True,
        text=True,
    )
    try:
        report = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        die(f"npm audit --json output does not parse: {error}\nstderr:\n{result.stderr}")

    package_name = read_npm_package_name(npm_dir)

    records = []
    for name, vuln in report.get("vulnerabilities", {}).items():
        for via in vuln.get("via", []):
            if not isinstance(via, dict):
                continue
            url = via.get("url", "")
            ghsa = GHSA_IN_URL.search(url)
            if not ghsa:
                die(f"npm audit finding for '{name}' carries no GHSA id in its advisory url: {url!r}")
            records.append(
                {
                    "primary_id": ghsa.group(1),
                    "aliases": [],
                    "package": name,
                    "severity": via.get("severity", "(unknown)"),
                    "first_party": name == package_name,
                    "reachable": None,
                }
            )
    return records


def image_findings(tag):
    """One record per `Results[].Vulnerabilities[]` entry Trivy reports against `tag`: {primary_id,
    aliases, package, severity, first_party, reachable}. `first_party` is always False and
    `reachable` always True — nothing in a built image is this project's own source, and Trivy carries
    no reachability concept, so every finding is held to the same rule as npm's. An empty or missing
    `FixedVersion` is not read: an unfixed finding fails like any other, and only a register entry is
    relief."""
    result = subprocess.run(
        [
            "docker",
            "run",
            "--rm",
            "-v",
            "/var/run/docker.sock:/var/run/docker.sock",
            TRIVY_IMAGE,
            "image",
            "--format",
            "json",
            "--exit-code",
            "0",
            "--quiet",
            tag,
        ],
        capture_output=True,
        text=True,
    )
    try:
        report = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        die(f"trivy image --format json output does not parse: {error}\nstderr:\n{result.stderr}")

    records = []
    for entry in report.get("Results") or []:
        for vuln in entry.get("Vulnerabilities") or []:
            records.append(
                {
                    "primary_id": vuln["VulnerabilityID"],
                    "aliases": [],
                    "package": f"{vuln['PkgName']} ({vuln.get('InstalledVersion', '(unknown)')})",
                    "severity": vuln.get("Severity", "(unknown)"),
                    "first_party": False,
                    "reachable": True,
                }
            )
    return records


def no_exception_reason(finding):
    """Why no register entry may ever cover `finding`, or None if a current, matching entry
    legitimately could: first-party or Go standard-library, per docs/CI.md § *The exception
    register*."""
    if finding["first_party"]:
        return "first-party — no entry may cover it"
    if finding.get("stdlib"):
        return "a Go standard-library finding — the exception register never covers those"
    return None


def run(scope, findings, register_entries, today):
    problems = []

    scoped_entries = []
    for index, raw in enumerate(register_entries):
        # Scope is read before anything else is validated, so an entry can be routed to the run
        # that owns it without first assuming it is well-formed. An entry with no valid `go`/`npm`
        # scope of its own cannot be routed at all, so it is unroutable rather than the other
        # scope's business, and is examined — and can fail — every scope's run instead of neither.
        stored_scope = readable_scope(raw)
        if stored_scope not in VALID_SCOPES:
            _, entry_problems = validate_entry(raw, index)
            problems.extend(entry_problems)
            continue
        if stored_scope != scope:
            continue
        well_formed, entry_problems = validate_entry(raw, index)
        if not well_formed:
            problems.extend(entry_problems)
            continue
        currency = currency_problem(raw, today)
        if currency:
            problems.append(currency)
            continue
        scoped_entries.append(raw)

    accounted_for = set()  # entries that legitimately cover a finding, or were refused for trying to
    # cover one with no exception path (first-party or stdlib) — either way, not an orphan below.
    for finding in findings:
        covering = [
            entry
            for entry in scoped_entries
            if matches(entry, finding["primary_id"], finding["aliases"])
        ]
        blocked = no_exception_reason(finding)
        if blocked and covering:
            for entry in covering:
                problems.append(
                    f"register entry ('{entry['advisory']}') matches a finding with no exception "
                    f"path ({finding['primary_id']}, package {finding['package']}) — {blocked}"
                )
            accounted_for.update(id(entry) for entry in covering)
            covering = []  # such a finding cannot be suppressed by any entry
        else:
            accounted_for.update(id(entry) for entry in covering)

        must_fail = finding["reachable"] if scope == "go" else True
        if must_fail and not covering:
            if scope == "go":
                reason = "its trace reaches a called symbol"
            elif scope == "npm":
                reason = "npm audit reports it"
            else:
                reason = "trivy reports it"
            if blocked:
                reason += f", and it is {blocked}"
            problems.append(f"{finding['primary_id']} ({finding['package']}) is unregistered — {reason}")

    for entry in scoped_entries:
        if id(entry) not in accounted_for:
            problems.append(
                f"register entry ('{entry['advisory']}') is an orphan — no {scope} finding this run "
                "reports matches it"
            )

    report_findings(scope, findings, scoped_entries)

    return problems


def report_findings(scope, findings, scoped_entries):
    print(f"check-vulns ({scope}): {len(findings)} finding(s) reported")
    for finding in sorted(findings, key=lambda f: f["primary_id"]):
        covering = [
            entry["advisory"]
            for entry in scoped_entries
            if matches(entry, finding["primary_id"], finding["aliases"])
        ]
        if scope == "go":
            reach = "reachable" if finding["reachable"] else "informational (not called)"
        else:
            reach = "n/a"
        severity = finding["severity"] or "n/a"
        register_status = f"registered ({', '.join(covering)})" if covering else "unregistered"
        blocked = no_exception_reason(finding)
        if blocked:
            register_status = f"{blocked} — no exception path"
        print(
            f"  {finding['primary_id']}  package={finding['package']}  severity={severity}  "
            f"reachability={reach}  register={register_status}"
        )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scope", required=True, choices=VALID_SCOPES)
    parser.add_argument("--go-dir", default=str(ROOT / "backend"))
    parser.add_argument("--npm-dir", default=str(ROOT / "frontend"))
    parser.add_argument("--image")
    parser.add_argument("--register", default=str(ROOT / ".vulnerability-exceptions.json"))
    args = parser.parse_args()

    if args.scope == "image" and not args.image:
        die("--scope image requires --image <tag>")

    register_entries = load_register(Path(args.register))

    if args.scope == "go":
        findings = go_findings(Path(args.go_dir))
    elif args.scope == "npm":
        findings = npm_findings(Path(args.npm_dir))
    else:
        findings = image_findings(args.image)

    problems = run(args.scope, findings, register_entries, date.today())

    if problems:
        print(f"check-vulns ({args.scope}): {len(problems)} problem(s):", file=sys.stderr)
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        return 1

    print(f"check-vulns ({args.scope}): no unregistered or first-party finding, register clean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
