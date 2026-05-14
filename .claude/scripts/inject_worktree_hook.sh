#!/bin/bash
# Hook: UserPromptSubmit
# Automatically creates or discovers git worktrees for -worktree commands.
# Injects WORKTREE_DIR and PROJECT_ROOT into model context as system-reminder.
#
# Two modes:
#   CREATE — for execute-prp-worktree: creates new worktree if not exists
#   DISCOVER — for review/bug-fix: finds existing worktree by TASK-ID
#
# Reads stdin (expanded prompt after skill substitution).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source shared utilities
source "$SCRIPT_DIR/worktree_utils.sh"

# Read the expanded prompt from stdin
input=$(cat)

# Commands that CREATE worktrees (new task execution)
# Includes both expanded skill headers and raw skill command names
CREATE_COMMANDS=(
    "# Execute PRP (Worktree)"
    "# Run Tasks (Worktree)"
    "execute-prp-worktree"
    "run-tasks-worktree"
)

# Commands that DISCOVER existing worktrees
DISCOVER_COMMANDS=(
    "# Execute Bug Fix (Worktree)"
    "# Execute IT Bug Fix (Worktree)"
    "# Execute QA Integration Testing (Worktree)"
    "# Code Review (Worktree)"
    "# Integration Testing Review (Worktree)"
    "# Review Tests (Worktree)"
    "# Generate Commit (Worktree)"
    "# Generate Pull Request (Worktree)"
    "# Aggregate Users Code Review (Worktree)"
    "execute-bug-fix-worktree"
    "execute-it-bug-fix-worktree"
    "execute-qa-it-worktree"
    "review-code-worktree"
    "review-code-it-worktree"
    "review-tests-worktree"
    "generate-commit-worktree"
    "generate-pull-request-worktree"
    "aggregate-users-code-review-worktree"
)

# Determine mode
mode=""
for cmd in "${CREATE_COMMANDS[@]}"; do
    if echo "$input" | grep -qF "$cmd"; then
        mode="create"
        break
    fi
done

if [[ -z "$mode" ]]; then
    for cmd in "${DISCOVER_COMMANDS[@]}"; do
        if echo "$input" | grep -qF "$cmd"; then
            mode="discover"
            break
        fi
    done
fi

# Not a worktree command — skip
[[ -z "$mode" ]] && exit 0

# Extract task file path from the prompt (same patterns as inject_rules_hook.sh)
task_file=""
task_file=$(echo "$input" | grep -oiP '(?:PRP|Report|Bug report|Task/Plan)\s+file[:\s]*\K\S+\.(?:md|yaml)' | head -1) || true
if [[ -z "$task_file" ]]; then
    task_file=$(echo "$input" | grep -oP 'task_management/(?:plans|tasks|reviews|debug_reports|bug_reports)/\S+\.(?:md|yaml)' | head -1) || true
fi
if [[ -z "$task_file" ]]; then
    task_file=$(echo "$input" | grep -oP '\.memory_bank/agent_task_results/\S+\.(?:md|yaml)' | head -1) || true
fi

# Extract TASK-ID
task_id=""
if [[ -n "$task_file" ]]; then
    task_id=$(extract_task_id "$task_file")
fi

# Also try to extract TASK-ID directly from the prompt arguments
if [[ -z "$task_id" ]]; then
    task_id=$(echo "$input" | grep -oP '(?:REQ-\d+-)?TASK-\d+' | head -1) || true
fi

# No TASK-ID found — cannot proceed
if [[ -z "$task_id" ]]; then
    echo "# WORKTREE ERROR (auto-injected)"
    echo ""
    echo "Could not extract TASK-ID from command arguments."
    echo "Provide a file path containing TASK-ID (e.g., task_management/plans/REQ-001-TASK-082-PRP.md)."
    exit 0
fi

worktree_dir=""
branch=""

if [[ "$mode" == "create" ]]; then
    # Try to find existing worktree first
    worktree_dir=$(find_worktree "$PROJECT_ROOT" "$task_id") || true

    if [[ -n "$worktree_dir" ]]; then
        branch=$(get_worktree_branch "$worktree_dir")
    else
        # Determine branch name
        jira_id=$(find_jira_id "$PROJECT_ROOT" "$task_id") || true
        if [[ -n "$jira_id" ]]; then
            branch="feature/$jira_id"
        else
            branch="feature/$task_id"
        fi

        # Create worktree
        worktree_dir=$(create_worktree "$PROJECT_ROOT" "$task_id" "$branch") || true
        if [[ -z "$worktree_dir" ]]; then
            echo "# WORKTREE ERROR (auto-injected)"
            echo ""
            echo "Failed to create worktree for $task_id with branch $branch."
            echo "Check: git worktree list, existing branches, disk space."
            exit 0
        fi
    fi

elif [[ "$mode" == "discover" ]]; then
    worktree_dir=$(find_worktree "$PROJECT_ROOT" "$task_id") || true

    if [[ -z "$worktree_dir" ]]; then
        echo "# WORKTREE ERROR (auto-injected)"
        echo ""
        echo "Worktree for $task_id not found."
        echo "Expected: $PROJECT_ROOT/.worktrees/$task_id/"
        echo "Use /execute-prp-worktree to create one first."
        exit 0
    fi

    branch=$(get_worktree_branch "$worktree_dir")
fi

# Write state for PreToolUse agent hook
write_worktree_state "$PROJECT_ROOT" "$task_id" "$worktree_dir" "$branch"

# Output context for the model
cat <<EOF
# WORKTREE CONTEXT (auto-injected)

Active worktree for task $task_id:
- WORKTREE_DIR: $worktree_dir
- PROJECT_ROOT: $PROJECT_ROOT
- Branch: $branch
- Mode: $mode

## CRITICAL — Set Working Directory
**Your FIRST Bash action MUST be: \`cd $worktree_dir\`**
This sets the persistent working directory for the entire session.
After this, all \`make build\`, \`make test\`, \`make lint\` commands will automatically
execute from the worktree root — no need to prefix each command with \`cd\`.

## Path Rules
- \`make\`, \`go build\`, \`go test\`, \`make lint\`: run from WORKTREE_DIR (set via \`cd\` above)
- \`python3 .claude/scripts/...\`: use PROJECT_ROOT absolute paths (\`$PROJECT_ROOT/.claude/scripts/...\`)
- Agent prompts: include WORKTREE_DIR path and instruction to \`cd $worktree_dir\` before code operations
- Read/Edit/Write *.go files: use WORKTREE_DIR paths (\`$worktree_dir/internal/...\`)
- task_management/, .memory_bank/, .temp/: use PROJECT_ROOT paths (\`$PROJECT_ROOT/task_management/...\`)
- Guide/rule files: \`$PROJECT_ROOT/.claude/guides/...\`, \`$PROJECT_ROOT/.temp/rules/...\`
EOF
