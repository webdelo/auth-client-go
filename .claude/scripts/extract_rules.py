#!/usr/bin/env python3
"""Rule extraction system for Claude Code workflows.

Two-tier output: cheatsheet (one-liners) + full text for matched rules.
Supports task-scoped namespacing for parallel orchestrators.

Usage modes:

  1. Analyze task (hook / Phase A):
     python3 .claude/scripts/extract_rules.py analyze task_management/plans/TASK-066-PRP.md
     python3 .claude/scripts/extract_rules.py analyze task_management/plans/TASK-066-PRP.md \\
       --output-dir .temp/rules/TASK-066/

  2. Select rules by ID (Phase B — model picks from cheatsheet):
     python3 .claude/scripts/extract_rules.py select --rules ES-0051,CS-0050,UT-0001
     python3 .claude/scripts/extract_rules.py select --rules ES-0051 \\
       --output-dir .temp/rules/TASK-066/

  3. Generate cheatsheet (one-time, regenerate when guides change):
     python3 .claude/scripts/extract_rules.py cheatsheet
     python3 .claude/scripts/extract_rules.py cheatsheet --output .claude/guides/rules_cheatsheet.md

  3b. Generate libs cheatsheet:
     python3 .claude/scripts/extract_rules.py libs-cheatsheet
     python3 .claude/scripts/extract_rules.py libs-cheatsheet --output .claude/guides/libs_cheatsheet.md

  4. Legacy: extract specific rules from specific files:
     python3 .claude/scripts/extract_rules.py \\
       -f .claude/guides/golang/effective_standards.md -r ES-0051,ES-0060

  5. Legacy: stdin JSON:
     echo '{"path/to/file.md": ["ES-0051"]}' | python3 .claude/scripts/extract_rules.py

  6. Legacy: programmatic (ctx_execute):
     exec(open(".claude/scripts/extract_rules.py").read())
     extract_rules({".claude/guides/golang/effective_standards.md": ["ES-0051"]})
"""

import re
import sys
import os
import json
import argparse
from pathlib import Path

# ============================================================
# CONFIGURATION
# ============================================================

# Guide files with their rule ID prefixes
GUIDE_FILES = {
    "ES": ".claude/guides/golang/effective_standards.md",
    "CS": ".claude/guides/golang/coding_style.md",
    "UT": ".claude/guides/golang/unit_tests_standards.md",
    "EC": ".claude/guides/golang/effective_concurrency.md",
}

# Section names for cheatsheet output
SECTION_NAMES = {
    "ES": "Golang Standards",
    "CS": "Coding Style",
    "UT": "Unit Tests",
    "EC": "Concurrency",
}

# Library guides directory (auto-discovered from frontmatter)
LIBS_DIR = ".claude/guides/libs"

# Patterns guides directory (auto-discovered from frontmatter)
PATTERNS_DIR = ".claude/guides/golang/patterns"

# Fallback: hardcoded lib files (used only if frontmatter is missing)
LIB_FILES_FALLBACK = {
    "TFY": ".claude/guides/libs/testify.md",
    "MKR": ".claude/guides/libs/mockery.md",
    "GNK": ".claude/guides/libs/gonkey.md",
    "UFX": ".claude/guides/libs/uber_fx.md",
    "CNC": ".claude/guides/libs/concurrency_patterns.md",
    "SCR": ".claude/guides/libs/scratch_usage.md",
    "FVO": ".claude/guides/libs/value_objects.md",
    "CLG": ".claude/guides/libs/ctxlogger.md",
}

# Fallback: hardcoded pattern files (used only if frontmatter is missing)
PATTERNS_FILES_FALLBACK = {
    "FM": ".claude/guides/golang/patterns/factory_method.md",
}

# Cache for discovered lib metadata
_lib_meta_cache: dict | None = None

# Cache for discovered patterns metadata
_patterns_meta_cache: dict | None = None


def _parse_frontmatter(file_path: str) -> dict | None:
    """Parse YAML frontmatter from a markdown file.

    Returns dict with keys: prefix, name, packages, triggers, description.
    Returns None if no frontmatter found.
    """
    path = Path(file_path)
    if not path.exists():
        return None

    text = path.read_text(encoding="utf-8")
    if not text.startswith("---"):
        return None

    end = text.find("---", 3)
    if end == -1:
        return None

    try:
        import yaml
        meta = yaml.safe_load(text[3:end])
        if isinstance(meta, dict) and "prefix" in meta:
            meta["_file_path"] = str(file_path)
            return meta
    except Exception:
        pass
    return None


def _discover_libs() -> dict[str, dict]:
    """Auto-discover lib guides from LIBS_DIR by scanning frontmatter.

    Returns {prefix: {name, packages, triggers, description, _file_path}}.
    Falls back to LIB_FILES_FALLBACK for files without frontmatter.
    """
    global _lib_meta_cache
    if _lib_meta_cache is not None:
        return _lib_meta_cache

    result = {}
    libs_path = Path(LIBS_DIR)

    if libs_path.is_dir():
        for md_file in sorted(libs_path.glob("*.md")):
            meta = _parse_frontmatter(str(md_file))
            if meta and "prefix" in meta:
                result[meta["prefix"]] = meta

    # Fallback: add any missing entries from hardcoded list
    for prefix, file_path in LIB_FILES_FALLBACK.items():
        if prefix not in result and Path(file_path).exists():
            result[prefix] = {
                "prefix": prefix,
                "name": prefix,
                "packages": [],
                "triggers": [],
                "description": "",
                "_file_path": file_path,
            }

    _lib_meta_cache = result
    return result


def _get_lib_files() -> dict[str, str]:
    """Get {prefix: file_path} for all discovered libs."""
    return {prefix: meta["_file_path"] for prefix, meta in _discover_libs().items()}


def _get_lib_names() -> dict[str, str]:
    """Get {prefix: name} for all discovered libs."""
    return {prefix: meta.get("name", prefix) for prefix, meta in _discover_libs().items()}


def _discover_patterns() -> dict[str, dict]:
    """Auto-discover pattern guides from PATTERNS_DIR by scanning frontmatter.

    Returns {prefix: {name, packages, triggers, description, _file_path}}.
    Falls back to PATTERNS_FILES_FALLBACK for files without frontmatter.
    """
    global _patterns_meta_cache
    if _patterns_meta_cache is not None:
        return _patterns_meta_cache

    result = {}
    patterns_path = Path(PATTERNS_DIR)

    if patterns_path.is_dir():
        for md_file in sorted(patterns_path.glob("*.md")):
            meta = _parse_frontmatter(str(md_file))
            if meta and "prefix" in meta:
                result[meta["prefix"]] = meta

    # Fallback: add any missing entries from hardcoded list
    for prefix, file_path in PATTERNS_FILES_FALLBACK.items():
        if prefix not in result and Path(file_path).exists():
            result[prefix] = {
                "prefix": prefix,
                "name": prefix,
                "packages": [],
                "triggers": [],
                "description": "",
                "_file_path": file_path,
            }

    _patterns_meta_cache = result
    return result


def _get_patterns_files() -> dict[str, str]:
    """Get {prefix: file_path} for all discovered patterns."""
    return {prefix: meta["_file_path"] for prefix, meta in _discover_patterns().items()}


def _get_patterns_names() -> dict[str, str]:
    """Get {prefix: name} for all discovered patterns."""
    return {prefix: meta.get("name", prefix) for prefix, meta in _discover_patterns().items()}

# Tech signal patterns → rule matching categories
# Each pattern maps to a list of actions:
#   "EC"              → load ALL rules with this prefix
#   "ES-005"          → load rules starting with ES-005 (ES-0050..ES-0057)
#   "ES-0051"         → load this exact rule
TECH_SIGNALS = {
    # Concurrency primitives
    r"errgroup|atomic\b|sync\.(?:Mutex|RWMutex|WaitGroup|Once)"
    r"|goroutine|go\s+func|chan\s|<-\s|select\s*\{"
    r"|concurrency|параллел": ["EC", "CNC"],

    # PostgreSQL (DBA guides have no ES-style IDs; load relevant ES rules for repo code)
    r"pgx\b|pgxpool|postgres(?:ql)?|LEFT\s+JOIN|INNER\s+JOIN"
    r"|INSERT\s+INTO|CREATE\s+TABLE|ALTER\s+TABLE|migration|миграци": [
        "ES-0051", "ES-0055", "ES-0056",  # interfaces at consumer
        "ES-0060", "ES-0061", "ES-0062",  # error handling
        "ES-0040", "ES-0041",             # return value + named returns
        "ES-0042", "ES-0043",             # defer for cleanup
        "ES-0035",                        # defensive copying
    ],

    # ClickHouse (same — load relevant ES rules for repo code)
    r"clickhouse|OHLCV|MergeTree|Replacing\s*MergeTree": [
        "ES-0051", "ES-0055", "ES-0056",
        "ES-0060", "ES-0061",
        "ES-0040", "ES-0042",
        "ES-0035",
    ],

    # Testing
    r"mockery|\.EXPECT\(\)|\.On\(\s*\"|testify"
    r"|assert\.\w|require\.\w|_test\.go|table.driven": ["UT", "TFY", "MKR"],

    # Uber FX DI
    r"fx\.(?:Provide|Module|New|As|Invoke)|fxm_|fxs_|fxh_"
    r"|uber[./]fx|dependency\s+injection|DI\b": [
        "ES-0051", "ES-0055", "ES-0056",  # interfaces at consumer
        "ES-0140", "ES-0141", "ES-0142", "ES-0143",  # constructors
        "UFX",  # uber fx lib guide
    ],

    # HTTP/gRPC handlers
    r"ogen\b|grpc\.|protobuf|\.proto\b|handler\b|endpoint\b"
    r"|middleware|interceptor|Content-Type|Content-Disposition": [
        "ES-0051", "ES-0055", "ES-0056",  # interfaces at consumer
        "ES-0060", "ES-0061",             # error handling
        "ES-0040", "ES-0044",             # return values, max 3 returns
        "CS-0050",                        # each entity in separate package
    ],

    # Interface patterns
    r"interface\s*\{|type\s+\w+\s+interface\b": [
        "ES-0050", "ES-0051", "ES-0052", "ES-0053",
        "ES-0054", "ES-0055", "ES-0056", "ES-0057",
    ],

    # Error handling
    r"fmt\.Errorf|errors\.(?:New|Is|As|Join)|%w|sentinel": [
        "ES-0060", "ES-0061", "ES-0062", "ES-0063", "ES-0064", "ES-0065",
    ],

    # Constructor / DI patterns
    r"func\s+New\w+\(|Dependencies\s+struct|noop|nil.check": [
        "ES-0140", "ES-0141", "ES-0142", "ES-0143",
    ],

    # Gonkey integration tests
    r"gonkey|yaml\s+test|fixture|integration.test": ["GNK"],

    # Scratch infrastructure
    r"prometheus|metrics|health.?check|sentry|ctx.logger|middleware|logging"
    r"|scratch|scratch\.yaml": ["SCR"],

    # Context logger
    r"ctxlogger|ctx.logger|WithField|zap\.String|zap\.Error|zap\.Int"
    r"|log\.Info\(ctx|log\.Warn\(ctx|log\.Error\(ctx": ["CLG"],

    # Financial value objects
    r"AssetQuantity|Percent|Quantity|financial.values"
    r"|Coins|Units|TickSize|BaseSize": ["FVO"],
}

# Task type detection
TASK_TYPE_PATTERNS = {
    "coding":      [r"-PRP\.md$", r"реализ", r"implement", r"создать", r"create"],
    "review":      [r"CODE-REVIEW", r"code.review", r"ревью", r"review"],
    "bug_fix":     [r"BUG-REPORT", r"DEBUG-REPORT", r"bug", r"баг", r"исправ"],
    "refactoring": [r"рефакторинг", r"refactor", r"удалить", r"remove", r"rename", r"переименов"],
}


# ============================================================
# REGEX PATTERNS
# ============================================================

# Matches rule headers: ### ES-0051: Title
RULE_HEADER_RE = re.compile(r"^(#{2,4})\s+([A-Z][\w-]+-\d+)[:\s]")

# Known rule prefixes (filter out false positives like FB-107, REQ-001, ADR-005)
# NOTE: evaluated lazily via function because lib prefixes are discovered at runtime
def _known_rule_prefixes() -> frozenset[str]:
    return frozenset(GUIDE_FILES.keys()) | frozenset(_get_lib_files().keys()) | frozenset(_get_patterns_files().keys())

# Matches rule IDs in text: ES-0051, CS-0112, UT-0001
RULE_ID_IN_TEXT = re.compile(r"\b([A-Z]{2,3}-\d{3,4})\b")


# ============================================================
# RULE PARSING (core — unchanged from v1)
# ============================================================

def parse_rules(file_path: str) -> dict[str, tuple[int, int, str]]:
    """Parse a guide file and return {rule_id: (start_line, end_line, title)}.

    start_line is 0-based inclusive, end_line is exclusive.
    """
    path = Path(file_path)
    if not path.exists():
        print(f"# ERROR: file not found: {file_path}", file=sys.stderr)
        return {}

    lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
    positions: list[tuple[str, int, str, int]] = []  # (rule_id, start, title, header_level)

    for i, line in enumerate(lines):
        m = RULE_HEADER_RE.match(line)
        if m:
            header_level = len(m.group(1))
            rule_id = m.group(2)
            title = line.strip().lstrip("#").strip()
            positions.append((rule_id, i, title, header_level))

    rules = {}
    for idx, (rule_id, start, title, level) in enumerate(positions):
        # End = next rule of same or higher level, or end of file
        end = len(lines)
        for next_id, next_start, _, next_level in positions[idx + 1:]:
            if next_level <= level:
                end = next_start
                break
        rules[rule_id] = (start, end, title)

    return rules


# ============================================================
# RULE DATA ACCESS
# ============================================================

# Cache: {file_path: {rule_id: (start, end, title)}}
_rules_cache: dict[str, dict[str, tuple[int, int, str]]] = {}
# Cache: {file_path: list[str]}  (file lines)
_lines_cache: dict[str, list[str]] = {}


def _get_parsed(file_path: str) -> dict[str, tuple[int, int, str]]:
    """Get parsed rules with caching."""
    if file_path not in _rules_cache:
        _rules_cache[file_path] = parse_rules(file_path)
    return _rules_cache[file_path]


def _get_lines(file_path: str) -> list[str]:
    """Get file lines with caching."""
    if file_path not in _lines_cache:
        path = Path(file_path)
        if path.exists():
            _lines_cache[file_path] = path.read_text(encoding="utf-8").splitlines(keepends=True)
        else:
            _lines_cache[file_path] = []
    return _lines_cache[file_path]


def get_all_titles() -> dict[str, str]:
    """Get {rule_id: title_oneliner} for ALL rules across ALL guide files."""
    titles = {}
    for prefix, file_path in GUIDE_FILES.items():
        rules = _get_parsed(file_path)
        for rule_id, (start, end, title) in rules.items():
            titles[rule_id] = title
    return titles


def get_full_text(rule_ids: set[str]) -> dict[str, str]:
    """Get {rule_id: full_text_with_examples} for specified rules."""
    result = {}

    # Group by prefix → file (check GUIDE_FILES, LIB_FILES, and PATTERNS_FILES)
    by_file: dict[str, set[str]] = {}
    all_files = {**GUIDE_FILES, **_get_lib_files(), **_get_patterns_files()}
    for rid in rule_ids:
        prefix = rid.split("-")[0]
        if prefix in all_files:
            by_file.setdefault(all_files[prefix], set()).add(rid)

    for file_path, ids in by_file.items():
        rules = _get_parsed(file_path)
        lines = _get_lines(file_path)
        for rid in ids:
            if rid in rules:
                start, end, _ = rules[rid]
                result[rid] = "".join(lines[start:end]).rstrip()

    return result


def get_all_ids_with_prefix(prefix: str) -> set[str]:
    """Get all rule IDs that start with a prefix (e.g., 'EC' → all EC-* IDs)."""
    all_files = {**GUIDE_FILES, **_get_lib_files(), **_get_patterns_files()}
    if prefix not in all_files:
        return set()
    rules = _get_parsed(all_files[prefix])
    return set(rules.keys())


# ============================================================
# SIGNAL EXTRACTION FROM TASK/PRP FILES
# ============================================================

def extract_task_id(filepath: str) -> str:
    """TASK-066-PRP.md → TASK-066, REQ-001-TASK-074.md → REQ-001-TASK-074."""
    m = re.search(r"((?:REQ-\d+-)?TASK-\d+)", Path(filepath).name)
    return m.group(1) if m else f"session-{os.getpid()}"


def detect_task_type(filepath: str, text: str) -> str:
    """Detect task type from filename and content."""
    combined = f"{Path(filepath).name} {text[:500]}".lower()

    for task_type, patterns in TASK_TYPE_PATTERNS.items():
        for pat in patterns:
            if re.search(pat, combined, re.IGNORECASE):
                return task_type

    return "coding"


def extract_signals(text: str) -> tuple[set[str], set[str]]:
    """Extract signals from task text.

    Returns:
        explicit_ids: Rule IDs mentioned directly (CS-0112, ES-0051...)
        matched_ids:  Rule IDs matched via tech signals
    """
    # 1. Explicit Rule IDs in text (filter to known prefixes only)
    known = _known_rule_prefixes()
    explicit_ids = {
        rid for rid in RULE_ID_IN_TEXT.findall(text)
        if rid.split("-")[0] in known
    }

    # 2. Tech signal matching → rule IDs/prefixes
    matched_ids: set[str] = set()

    for pattern, targets in TECH_SIGNALS.items():
        if not re.search(pattern, text, re.IGNORECASE):
            continue
        for target in targets:
            if re.match(r"^[A-Z]{2,3}$", target):
                # Full prefix → all rules with this prefix
                matched_ids.update(get_all_ids_with_prefix(target))
            elif re.match(r"^[A-Z]{2,3}-\d{3}", target):
                if len(target.split("-")[1]) <= 3:
                    # Partial prefix like "ES-005" → all ES-005x rules
                    prefix = target.split("-")[0]
                    for rid in get_all_ids_with_prefix(prefix):
                        if rid.startswith(target):
                            matched_ids.add(rid)
                else:
                    # Exact rule ID
                    matched_ids.add(target)

    return explicit_ids, matched_ids


# ============================================================
# OUTPUT FORMATTERS
# ============================================================

def format_cheatsheet(mark_ids: set[str] | None = None) -> str:
    """Generate one-liner cheatsheet of ALL rules.

    Args:
        mark_ids: if provided, mark these rules with ⚡ (loaded in full)
    """
    mark = mark_ids or set()
    output = ["# Rules Cheatsheet", ""]

    for prefix in ["ES", "CS", "UT", "EC"]:
        file_path = GUIDE_FILES.get(prefix)
        if not file_path:
            continue
        rules = _get_parsed(file_path)
        if not rules:
            continue

        section_name = SECTION_NAMES.get(prefix, prefix)
        output.append(f"## {section_name} ({prefix}, {len(rules)} rules)")
        for rule_id in sorted(rules.keys(), key=lambda x: int(x.split("-")[1])):
            _, _, title = rules[rule_id]
            marker = " ⚡" if rule_id in mark else ""
            output.append(f"- {title}{marker}")
        output.append("")

    return "\n".join(output)


def format_full_rules(rules_text: dict[str, str], header: str = "") -> str:
    """Format full rule texts with optional header."""
    if not rules_text:
        return ""

    output = []
    if header:
        output.append(header)
        output.append("")

    for rid in sorted(rules_text.keys(), key=lambda x: (x.split("-")[0], int(x.split("-")[1]))):
        output.append(rules_text[rid])
        output.append("")

    return "\n".join(output)


# ============================================================
# SCOPE FILE WRITER
# ============================================================

def write_scope_files(
    output_dir: Path,
    cheatsheet: str,
    full_rules: dict[str, str],
):
    """Write scope-specific files for agents."""
    output_dir.mkdir(parents=True, exist_ok=True)

    # cheatsheet.md
    (output_dir / "cheatsheet.md").write_text(cheatsheet, encoding="utf-8")

    # full_rules.md — all detailed rules combined
    full_content = format_full_rules(
        full_rules,
        f"# All Extracted Rules ({len(full_rules)} rules)"
    )
    (output_dir / "full_rules.md").write_text(full_content, encoding="utf-8")

    # coding_scope.md — for code-writer (ES + CS + EC)
    coding = {k: v for k, v in full_rules.items() if k.startswith(("ES-", "CS-", "EC-"))}
    if coding:
        content = format_full_rules(coding, f"# Coding Rules ({len(coding)} rules)")
        (output_dir / "coding_scope.md").write_text(content, encoding="utf-8")

    # test_scope.md — for unit-test-writer (UT)
    testing = {k: v for k, v in full_rules.items() if k.startswith("UT-")}
    if testing:
        content = format_full_rules(testing, f"# Test Rules ({len(testing)} rules)")
        (output_dir / "test_scope.md").write_text(content, encoding="utf-8")

    # review_scope.md — for code-reviewer (all)
    if full_rules:
        content = format_full_rules(full_rules, f"# Review Rules ({len(full_rules)} rules)")
        (output_dir / "review_scope.md").write_text(content, encoding="utf-8")


# ============================================================
# COMMAND: analyze
# ============================================================

def cmd_analyze(task_file: str, output_dir: str | None = None) -> None:
    """Analyze task file → cheatsheet + signal-matched full rules + scope files."""
    if not Path(task_file).exists():
        print(f"ERROR: file not found: {task_file}", file=sys.stderr)
        sys.exit(1)

    text = Path(task_file).read_text(encoding="utf-8")
    task_id = extract_task_id(task_file)
    task_type = detect_task_type(task_file, text)

    # Extract signals
    explicit_ids, signal_matched_ids = extract_signals(text)
    all_needed = explicit_ids | signal_matched_ids

    # Get full text
    full_rules = get_full_text(all_needed)

    # Generate cheatsheet with markers
    cheatsheet = format_cheatsheet(mark_ids=set(full_rules.keys()))

    # Count signal categories for summary
    signal_cats = set()
    for pattern, targets in TECH_SIGNALS.items():
        if re.search(pattern, text, re.IGNORECASE):
            for t in targets:
                if re.match(r"^[A-Z]{2,3}$", t):
                    signal_cats.add(t)
                else:
                    signal_cats.add(t.split("-")[0])

    # ── Print stdout (goes into model context) ──

    print(f"# ⚡ Rules Context for {task_id}")
    print(f"# Task type: {task_type} | Signals: {', '.join(sorted(signal_cats)) or 'none'}")
    print(f"# Explicit IDs: {', '.join(sorted(explicit_ids)) or 'none'}")
    print(f"# Signal-matched: {len(signal_matched_ids)} rules | Total full: {len(full_rules)} rules")
    print()

    # Tier 1: cheatsheet
    print(cheatsheet)

    # Tier 2: full rules
    if full_rules:
        print(f"# {'═' * 60}")
        print(f"# DETAILED RULES ({len(full_rules)} rules with examples)")
        print(f"# Violations = code review failure.")
        print(f"# {'═' * 60}")
        print()
        print(format_full_rules(full_rules))

    # ── Write scope files ──

    if output_dir:
        out = Path(output_dir)
        write_scope_files(out, cheatsheet, full_rules)
        # Write metadata
        meta = {
            "task_id": task_id,
            "task_type": task_type,
            "task_file": task_file,
            "explicit_ids": sorted(explicit_ids),
            "signal_matched_count": len(signal_matched_ids),
            "total_full_rules": len(full_rules),
            "signal_categories": sorted(signal_cats),
        }
        (out / "metadata.json").write_text(
            json.dumps(meta, indent=2, ensure_ascii=False), encoding="utf-8"
        )

        # Verify files were written
        expected = ["cheatsheet.md", "full_rules.md", "metadata.json"]
        if any(k.startswith(("ES-", "CS-", "EC-")) for k in full_rules):
            expected.append("coding_scope.md")
        if any(k.startswith("UT-") for k in full_rules):
            expected.append("test_scope.md")
        if full_rules:
            expected.append("review_scope.md")

        written = [f for f in expected if (out / f).exists()]
        missing = [f for f in expected if not (out / f).exists()]

        print(f"\n# {'═' * 60}")
        print(f"# SCOPE FILES WRITTEN to {output_dir}/")
        print(f"# Created: {', '.join(written)}")
        if missing:
            print(f"# ERROR — missing: {', '.join(missing)}")
        print(f"# Total rules extracted: {len(full_rules)}")
        print(f"# {'═' * 60}")
        print(f"\n# Scope files written to {output_dir}/", file=sys.stderr)
    else:
        # Phase B instructions — only when no output-dir (interactive/stdout mode)
        print(f"# {'═' * 60}")
        print(f"# TO CREATE SCOPE FILES:")
        print(f"# Re-run with --output-dir to write scope files:")
        print(f"# python3 .claude/scripts/extract_rules.py analyze {task_file} \\")
        print(f"#   --output-dir .temp/rules/{task_id}/")
        print(f"#")
        print(f"# Or select specific rules manually:")
        print(f"# python3 .claude/scripts/extract_rules.py select \\")
        print(f"#   --rules ES-XXXX,CS-XXXX \\")
        print(f"#   --output-dir .temp/rules/{task_id}/")
        print(f"# {'═' * 60}")


# ============================================================
# COMMAND: select
# ============================================================

def cmd_select(rules_arg: str, output_dir: str | None = None) -> None:
    """Select specific rules by ID → print full text + append to scope files."""
    rule_ids = {r.strip() for r in rules_arg.split(",") if r.strip()}

    if not rule_ids:
        print("ERROR: no rule IDs specified", file=sys.stderr)
        sys.exit(1)

    rules_text = get_full_text(rule_ids)

    not_found = rule_ids - set(rules_text.keys())
    if not_found:
        print(f"# WARN: not found: {', '.join(sorted(not_found))}")

    # Print to stdout
    for rid in sorted(rules_text, key=lambda x: (x.split("-")[0], int(x.split("-")[1]))):
        print(rules_text[rid])
        print()

    print(f"# --- Selected {len(rules_text)} rules ({sum(t.count(chr(10)) + 1 for t in rules_text.values())} lines) ---")

    # Append to scope files
    if output_dir:
        out = Path(output_dir)
        if not out.exists():
            print(f"# WARN: {output_dir} does not exist, creating", file=sys.stderr)
            out.mkdir(parents=True, exist_ok=True)

        appendix = "\n# --- Model-selected rules (Phase B) ---\n\n"
        for scope_name, filter_fn in [
            ("coding_scope.md", lambda k: k.startswith(("ES-", "CS-", "EC-"))),
            ("test_scope.md", lambda k: k.startswith("UT-")),
            ("review_scope.md", lambda k: True),
            ("full_rules.md", lambda k: True),
        ]:
            relevant = {k: v for k, v in rules_text.items() if filter_fn(k)}
            if not relevant:
                continue

            scope_path = out / scope_name
            with open(scope_path, "a", encoding="utf-8") as f:
                f.write(appendix)
                for rid in sorted(relevant):
                    f.write(relevant[rid])
                    f.write("\n\n")

        print(f"# Appended to scope files in {output_dir}/", file=sys.stderr)


# ============================================================
# COMMAND: cheatsheet
# ============================================================

def cmd_cheatsheet(output_file: str | None = None) -> None:
    """Generate full cheatsheet of all rules."""
    cs = format_cheatsheet()

    if output_file:
        Path(output_file).write_text(cs, encoding="utf-8")
        # Summary to stdout
        titles = get_all_titles()
        print(f"Cheatsheet: {len(titles)} rules written to {output_file}")
    else:
        print(cs)


# ============================================================
# COMMAND: libs-cheatsheet
# ============================================================

def format_libs_cheatsheet() -> str:
    """Generate block-format libs reference from frontmatter metadata."""
    libs = _discover_libs()
    output = [
        "# Libs Reference",
        "",
        "Guides are loaded **fully** by triggers. No per-rule selection needed.",
        "",
    ]

    # Fixed display order: testing → DI → infra → domain
    display_order = ["TFY", "MKR", "GNK", "UFX", "CNC", "SCR", "FVO", "CLG"]
    ordered = [p for p in display_order if p in libs] + [p for p in libs if p not in display_order]
    for prefix in ordered:
        meta = libs[prefix]
        file_path = meta["_file_path"]
        rules = _get_parsed(file_path)
        count = len(rules) if rules else 0
        name = meta.get("name", prefix)
        desc = meta.get("description", "")
        packages = meta.get("packages", [])

        output.append(f"### {prefix} — {name} ({count} patterns)")
        if packages:
            pkgs_str = ", ".join(f"`{p}`" for p in packages)
            output.append(f"Packages: {pkgs_str}")
        output.append(f"Guide: `{file_path}`")
        if desc:
            output.append(desc)
        output.append("")

    return "\n".join(output)


def cmd_libs_cheatsheet(output_file: str | None = None) -> None:
    """Generate full cheatsheet of all lib patterns."""
    cs = format_libs_cheatsheet()

    if output_file:
        Path(output_file).write_text(cs, encoding="utf-8")
        total = sum(len(_get_parsed(f)) for f in _get_lib_files().values())
        print(f"Libs cheatsheet: {total} patterns written to {output_file}")
    else:
        print(cs)


# ============================================================
# COMMAND: patterns-cheatsheet
# ============================================================

def format_patterns_cheatsheet() -> str:
    """Generate block-format patterns reference from frontmatter metadata."""
    patterns = _discover_patterns()
    output = [
        "# Patterns Reference",
        "",
        "Guides are loaded **fully** by triggers. No per-rule selection needed.",
        "",
    ]

    # Fixed display order (add new patterns here)
    display_order = ["FM"]
    ordered = [p for p in display_order if p in patterns] + [p for p in patterns if p not in display_order]
    for prefix in ordered:
        meta = patterns[prefix]
        file_path = meta["_file_path"]
        rules = _get_parsed(file_path)
        count = len(rules) if rules else 0
        name = meta.get("name", prefix)
        desc = meta.get("description", "")
        packages = meta.get("packages", [])

        output.append(f"### {prefix} — {name} ({count} patterns)")
        if packages:
            pkgs_str = ", ".join(f"`{p}`" for p in packages)
            output.append(f"Packages: {pkgs_str}")
        output.append(f"Guide: `{file_path}`")
        if desc:
            output.append(desc)
        output.append("")

    return "\n".join(output)


def cmd_patterns_cheatsheet(output_file: str | None = None) -> None:
    """Generate full cheatsheet of all pattern guides."""
    cs = format_patterns_cheatsheet()

    if output_file:
        Path(output_file).write_text(cs, encoding="utf-8")
        total = sum(len(_get_parsed(f)) for f in _get_patterns_files().values())
        print(f"Patterns cheatsheet: {total} patterns written to {output_file}")
    else:
        print(cs)


# ============================================================
# LEGACY: extract_rules() function (backward compatible, for ctx_execute)
# ============================================================

def extract_rules(requests: dict[str, list[str]], *, show_stats: bool = True) -> None:
    """Extract and print requested rules from guide files.

    Args:
        requests: {file_path: [rule_id, ...]} or {file_path: ["*"]} for all rules.
        show_stats: print summary line at the end.
    """
    total_extracted = 0
    total_lines_extracted = 0
    total_lines_in_files = 0
    not_found: list[str] = []

    for file_path, rule_ids in requests.items():
        path = Path(file_path)
        if not path.exists():
            print(f"# ERROR: file not found: {file_path}")
            continue

        all_lines = _get_lines(file_path)
        total_lines_in_files += len(all_lines)
        rules = _get_parsed(file_path)

        # Handle --all / ["*"]
        if rule_ids == ["*"] or rule_ids == "*":
            rule_ids = list(rules.keys())

        print(f"# === {path.name} ===")

        for rule_id in rule_ids:
            if rule_id not in rules:
                not_found.append(f"{rule_id} (in {path.name})")
                continue

            start, end, title = rules[rule_id]
            content = "".join(all_lines[start:end]).rstrip()
            print(content)
            print()
            total_extracted += 1
            total_lines_extracted += end - start

    if not_found:
        print(f"# WARN: rules not found: {', '.join(not_found)}")

    if show_stats:
        pct = (
            round(100 * total_lines_extracted / total_lines_in_files)
            if total_lines_in_files > 0
            else 0
        )
        print(
            f"# --- Extracted {total_extracted} rules, "
            f"{total_lines_extracted} lines "
            f"({pct}% of {total_lines_in_files} total) ---"
        )


def list_rules(file_path: str) -> None:
    """Print a compact list of all rule IDs and titles in a file."""
    rules = _get_parsed(file_path)
    if not rules:
        print(f"# No rules found in {file_path}")
        return

    print(f"# {Path(file_path).name}: {len(rules)} rules")
    for rule_id, (start, end, title) in rules.items():
        print(f"  {rule_id}: {title}  ({end - start} lines)")


# ============================================================
# MAIN
# ============================================================

def main():
    parser = argparse.ArgumentParser(
        description="Rule extraction system for Claude Code workflows",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Analyze task → cheatsheet + signal-matched rules + scope files
  %(prog)s analyze task_management/plans/TASK-066-PRP.md --output-dir .temp/rules/TASK-066/

  # Model selects additional rules from cheatsheet
  %(prog)s select --rules ES-0051,CS-0050,UT-0001 --output-dir .temp/rules/TASK-066/

  # Generate cheatsheet (regenerate when guides change)
  %(prog)s cheatsheet --output .claude/guides/rules_cheatsheet.md

  # Legacy: extract specific rules
  %(prog)s -f .claude/guides/golang/effective_standards.md -r ES-0051,ES-0060
        """,
    )
    subparsers = parser.add_subparsers(dest="command")

    # --- analyze ---
    p_analyze = subparsers.add_parser(
        "analyze", help="Analyze task file → extract relevant rules"
    )
    p_analyze.add_argument("task_file", help="Path to PRP/TASK/Report file")
    p_analyze.add_argument(
        "--output-dir",
        help="Directory for scope files (e.g., .temp/rules/TASK-066/)",
    )

    # --- select ---
    p_select = subparsers.add_parser(
        "select", help="Load specific rules by ID (Phase B)"
    )
    p_select.add_argument(
        "--rules", required=True, help="Comma-separated rule IDs (ES-0051,UT-0001)"
    )
    p_select.add_argument(
        "--output-dir", help="Append rules to scope files in this directory"
    )

    # --- cheatsheet ---
    p_cheatsheet = subparsers.add_parser(
        "cheatsheet", help="Generate one-liner cheatsheet of all rules"
    )
    p_cheatsheet.add_argument("--output", help="Output file (default: stdout)")

    # --- libs-cheatsheet ---
    p_libs = subparsers.add_parser(
        "libs-cheatsheet", help="Generate one-liner cheatsheet of all lib patterns"
    )
    p_libs.add_argument("--output", help="Output file (default: stdout)")

    # --- patterns-cheatsheet ---
    p_patterns = subparsers.add_parser(
        "patterns-cheatsheet", help="Generate one-liner cheatsheet of all pattern guides"
    )
    p_patterns.add_argument("--output", help="Output file (default: stdout)")

    # --- Legacy arguments ---
    parser.add_argument(
        "-f", "--file", action="append", dest="files",
        help="(Legacy) Guide file path (repeatable)"
    )
    parser.add_argument(
        "-r", "--rules", action="append", dest="rule_lists",
        help="(Legacy) Comma-separated rule IDs for preceding -f"
    )
    parser.add_argument("--all", action="store_true", help="(Legacy) Extract all rules")
    parser.add_argument("--list", action="store_true", help="(Legacy) List rule IDs only")
    parser.add_argument("--json", action="store_true", help="(Legacy) Read JSON from stdin")

    args = parser.parse_args()

    # ── New commands ──

    if args.command == "analyze":
        cmd_analyze(args.task_file, args.output_dir)
        return

    if args.command == "select":
        cmd_select(args.rules, args.output_dir)
        return

    if args.command == "cheatsheet":
        cmd_cheatsheet(args.output)
        return

    if args.command == "libs-cheatsheet":
        cmd_libs_cheatsheet(args.output)
        return

    if args.command == "patterns-cheatsheet":
        cmd_patterns_cheatsheet(args.output)
        return

    # ── Legacy modes ──

    # stdin JSON
    if args.json or (not args.files and args.command is None and not sys.stdin.isatty()):
        data = json.load(sys.stdin)
        extract_rules(data)
        return

    # --list
    if args.list and args.files:
        for f in args.files:
            list_rules(f)
        return

    # -f/-r pairs
    if args.files:
        requests = {}
        files = args.files or []
        rule_lists = args.rule_lists or []

        for i, f in enumerate(files):
            if args.all:
                requests[f] = ["*"]
            elif i < len(rule_lists):
                requests[f] = [r.strip() for r in rule_lists[i].split(",")]
            else:
                requests[f] = ["*"]

        extract_rules(requests)
        return

    parser.print_help()


if __name__ == "__main__":
    # Support both direct execution and programmatic use
    if len(sys.argv) > 1:
        main()
    elif not sys.stdin.isatty():
        # Piped JSON input
        data = json.load(sys.stdin)
        extract_rules(data)
