#!/bin/bash
# Hook: UserPromptSubmit
# Автоматически извлекает релевантные правила из task/PRP файлов
# и инъецирует их в контекст модели как system-reminder.
#
# Triggers on: specific command title patterns in expanded prompt.
# Commands: execute-prp, execute-bug-fix, execute-it-bug-fix, review-code,
#           review-code-it, review-tests, generate-prp, generate-devops-prp,
#           generate-adr, generate-devops-req, generate-task, run-parallel, run-tasks.
#
# Reads stdin (expanded prompt после skill substitution),
# ищет путь к task/PRP/report файлу, запускает extract_rules.py.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
EXTRACT_SCRIPT="$SCRIPT_DIR/extract_rules.py"

# Read the expanded prompt from stdin
input=$(cat)

# Skip if extract_rules.py doesn't exist
[[ -f "$EXTRACT_SCRIPT" ]] || exit 0

# Command title patterns that require rules injection.
RULES_COMMANDS=(
  "# Execute PRP"
  "# Execute Bug Fix"
  "# Execute IT Bug Fix"
  "# Code Review"
  "# Integration Testing Review"
  "# Review Tests"
  "# Generate PRP"
  "# Generate DevOps PRP"
  "# Generate ADR"
  "# Generate DevOps REQ"
  "# Generate TASK"
  "# Run Parallel"
  "# Run Tasks"
  "# Execute PRP (Worktree)"
  "# Execute Bug Fix (Worktree)"
  "# Execute IT Bug Fix (Worktree)"
  "# Code Review (Worktree)"
  "# Integration Testing Review (Worktree)"
  "# Review Tests (Worktree)"
  "# Execute QA Integration Testing (Worktree)"
  "# Aggregate Users Code Review (Worktree)"
  "# Run Tasks (Worktree)"
)

# Check if the expanded prompt contains a command that needs rules
needs_rules=false
for cmd in "${RULES_COMMANDS[@]}"; do
  if echo "$input" | grep -qF "$cmd"; then
    needs_rules=true
    break
  fi
done

$needs_rules || exit 0

# Try to find a task/PRP/report file reference in the prompt
# Priority: explicit file paths in the expanded prompt
task_file=""

# Pattern 1: "XXX file: path" headers from skill expansion
# Matches: PRP File, PRP file, Report file, Bug report file, Task/Plan file
task_file=$(echo "$input" | grep -oiP '(?:PRP|Report|Bug report|Task/Plan)\s+file[:\s]*\K\S+\.(?:md|yaml)' | head -1) || true

# Pattern 2: Direct path to task_management/ files
if [[ -z "$task_file" ]]; then
    task_file=$(echo "$input" | grep -oP 'task_management/(?:plans|tasks|reviews|debug_reports|bug_reports)/\S+\.(?:md|yaml)' | head -1) || true
fi

# Pattern 3: .memory_bank/agent_task_results/ reports
if [[ -z "$task_file" ]]; then
    task_file=$(echo "$input" | grep -oP '\.memory_bank/agent_task_results/\S+\.(?:md|yaml)' | head -1) || true
fi

# No task file found — skip silently
[[ -z "$task_file" ]] && exit 0

# Resolve relative path from project root
if [[ ! "$task_file" = /* ]]; then
    task_file="$PROJECT_ROOT/$task_file"
fi

# File must exist
[[ -f "$task_file" ]] || exit 0

# Extract TASK_ID for output directory namespacing
task_id=$(echo "$(basename "$task_file")" | grep -oP '(?:REQ-\d+-)?TASK-\d+' | head -1) || true
if [[ -z "$task_id" ]]; then
    task_id="session-$$"
fi

output_dir="$PROJECT_ROOT/.temp/rules/$task_id"

# Save current task_id for session-scoped cleanup (used by rules_cleanup_hook.sh)
mkdir -p "$PROJECT_ROOT/.temp/rules"
echo "$task_id" >> "$PROJECT_ROOT/.temp/rules/.active_task_ids"

# Run extract_rules.py — stdout goes into model context as system-reminder
cd "$PROJECT_ROOT"
python3 "$EXTRACT_SCRIPT" analyze "$task_file" --output-dir "$output_dir" 2>/dev/null
