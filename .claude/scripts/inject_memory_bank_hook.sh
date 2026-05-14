#!/bin/bash
# Hook: UserPromptSubmit
# Auto-injects memory bank project context files into the prompt
# when a command that requires project context is detected.
#
# Triggers on: command title patterns in expanded prompt (after skill substitution).
# Commands: execute-prp, execute-bug-fix, execute-it-bug-fix, execute-qa-it,
#           review-code, review-code-it, review-tests,
#           generate-prp, generate-devops-prp, generate-req, generate-devops-req,
#           generate-task, generate-tasks-for-adr, generate-tasks-for-req,
#           generate-commit, generate-pull-request, generate-adr,
#           generate-users-review-response, aggregate-users-code-review,
#           write-gonkey-tests, run-tasks, run-parallel, run-remote.
#
# Output goes into <user-prompt-submit-hook> in the model context (primacy zone).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Read the expanded prompt from stdin
input=$(cat)

# Command title patterns that require memory bank project context.
# These match the "# Title" headers in .claude/commands/*.md files
# after skill expansion replaces the command content into the prompt.
MEMORY_BANK_COMMANDS=(
  "# Execute PRP"
  "# Execute Bug Fix"
  "# Execute IT Bug Fix"
  "# Code Review"
  "# Integration Testing Review"
  "# Review Tests"
  "# Generate PRP"
  "# Generate DevOps PRP"
  "# Generate REQ"
  "# Generate DevOps REQ"
  "# Generate TASK"
  "# Generate Tasks for ADR"
  "# Generate Commit"
  "# Generate Users Review Response"
  "# Aggregate Users Code Review"
  "# Write Gonkey Tests"
  "# Run Tasks"
  "# Run Parallel"
  "# Execute QA Integration Testing"
  "# Generate ADR"
  "# Generate Pull Request"
  "# Generate Tasks for REQ"
  "# Run Remote"
  "# Execute PRP (Worktree)"
  "# Execute Bug Fix (Worktree)"
  "# Execute IT Bug Fix (Worktree)"
  "# Code Review (Worktree)"
  "# Integration Testing Review (Worktree)"
  "# Review Tests (Worktree)"
  "# Execute QA Integration Testing (Worktree)"
  "# Aggregate Users Code Review (Worktree)"
  "# Generate Commit (Worktree)"
  "# Generate Pull Request (Worktree)"
  "# Run Tasks (Worktree)"
)

# Check if the expanded prompt contains a command that needs memory bank
needs_mb=false
for cmd in "${MEMORY_BANK_COMMANDS[@]}"; do
  if echo "$input" | grep -qF "$cmd"; then
    needs_mb=true
    break
  fi
done

$needs_mb || exit 0

# Memory bank files to inject
MEMORY_BANK_FILES=(
  "dev_commands.yaml"
  "external_libs.yaml"
  "project_map.yaml"
  "tech_stack.yaml"
)

echo "# PROJECT CONTEXT (auto-injected)"
echo ""

for file in "${MEMORY_BANK_FILES[@]}"; do
  filepath="$PROJECT_ROOT/.memory_bank/$file"
  if [[ -f "$filepath" ]]; then
    echo "## ${file%.yaml}"
    echo '```yaml'
    cat "$filepath"
    echo '```'
    echo ""
  fi
done
