#!/bin/bash
# Hook: PreToolUse (matcher: Agent)
# Injects worktree path reminder before each Agent tool call.
# Only fires when an active worktree session exists (.temp/active_worktree.env).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

STATE_FILE="$PROJECT_ROOT/.temp/active_worktree.env"

# No active worktree — skip silently
[[ -f "$STATE_FILE" ]] || exit 0

# Read state
source "$STATE_FILE"

# Validate worktree still exists
[[ -d "$WORKTREE_DIR" ]] || exit 0

cat <<EOF
WORKTREE ACTIVE: Agents must work in $WORKTREE_DIR.
- Include "cd $WORKTREE_DIR" in agent prompts for code operations.
- Source code: $WORKTREE_DIR/internal/...
- Guides/scripts: $PROJECT_ROOT/.claude/...
- Task files: $PROJECT_ROOT/task_management/...
- Reports: $PROJECT_ROOT/.memory_bank/agent_task_results/...
EOF
