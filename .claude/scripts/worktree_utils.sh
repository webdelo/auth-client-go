#!/bin/bash
# Shared utility functions for worktree management.
# Sourced by inject_worktree_hook.sh and other worktree scripts.

# Extract TASK-ID from a file path.
# Supports: REQ-001-TASK-082-PRP.md, TASK-082-CODE-REVIEW.yaml, etc.
# Usage: extract_task_id "path/to/REQ-001-TASK-082-PRP.md"
extract_task_id() {
    local filepath="$1"
    local basename
    basename=$(basename "$filepath")
    echo "$basename" | grep -oP '(?:REQ-\d+-)?TASK-\d+' | head -1
}

# Find jira_id for a TASK-ID in tasks_list.yaml.
# Usage: find_jira_id "$PROJECT_ROOT" "REQ-001-TASK-082"
find_jira_id() {
    local project_root="$1"
    local task_id="$2"
    local tasks_file="$project_root/.memory_bank/tasks_list.yaml"

    [[ -f "$tasks_file" ]] || return 1

    # Extract jira_id from the task entry in YAML
    python3 -c "
import yaml, sys
with open('$tasks_file') as f:
    data = yaml.safe_load(f)
for req in data.get('requirements', []):
    for task in req.get('tasks', []):
        if task.get('id', '') == '$task_id':
            jira_id = task.get('jira_id', '')
            if jira_id:
                print(jira_id)
                sys.exit(0)
sys.exit(1)
" 2>/dev/null
}

# Find existing worktree directory for a TASK-ID.
# Returns absolute path or empty string.
# Usage: find_worktree "$PROJECT_ROOT" "REQ-001-TASK-082"
find_worktree() {
    local project_root="$1"
    local task_id="$2"
    local worktree_dir="$project_root/.worktrees/$task_id"

    # Direct lookup by TASK-ID
    if [[ -d "$worktree_dir" ]]; then
        echo "$worktree_dir"
        return 0
    fi

    # Fallback: search git worktree list for matching branch
    local jira_id
    jira_id=$(find_jira_id "$project_root" "$task_id") || true
    if [[ -n "$jira_id" ]]; then
        local branch="feature/$jira_id"
        local wt_path
        wt_path=$(git -C "$project_root" worktree list --porcelain 2>/dev/null \
            | awk -v branch="$branch" '
                /^worktree / { path=$2 }
                /^branch / { if ($2 == "refs/heads/" branch) print path }
            ' | head -1)
        if [[ -n "$wt_path" ]]; then
            echo "$wt_path"
            return 0
        fi
    fi

    return 1
}

# Create a new worktree for a TASK-ID.
# Usage: create_worktree "$PROJECT_ROOT" "REQ-001-TASK-082" "feature/FB-120"
create_worktree() {
    local project_root="$1"
    local task_id="$2"
    local branch="$3"
    local worktree_dir="$project_root/.worktrees/$task_id"

    mkdir -p "$project_root/.worktrees"

    # Ensure .worktrees/ is in .gitignore
    local gitignore="$project_root/.gitignore"
    if [[ -f "$gitignore" ]]; then
        if ! grep -qxF '/.worktrees/' "$gitignore"; then
            printf '\n/.worktrees/\n' >> "$gitignore"
        fi
    else
        echo '/.worktrees/' > "$gitignore"
    fi

    # Check if branch already exists
    if git -C "$project_root" show-ref --verify --quiet "refs/heads/$branch" 2>/dev/null; then
        # Branch exists — checkout existing branch into worktree
        if ! git -C "$project_root" worktree add "$worktree_dir" "$branch" >/dev/null 2>&1; then
            return 1
        fi
    else
        # Create new branch
        if ! git -C "$project_root" worktree add -b "$branch" "$worktree_dir" >/dev/null 2>&1; then
            return 1
        fi
    fi

    # Create metadata file
    cat > "$worktree_dir/.worktree-meta.yaml" <<METAEOF
task_id: "$task_id"
branch: "$branch"
created_at: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
METAEOF

    echo "$worktree_dir"
}

# Write active worktree state to .temp/active_worktree.env
# Usage: write_worktree_state "$PROJECT_ROOT" "TASK-ID" "/path/to/worktree" "feature/FB-120"
write_worktree_state() {
    local project_root="$1"
    local task_id="$2"
    local worktree_dir="$3"
    local branch="$4"

    mkdir -p "$project_root/.temp"
    cat > "$project_root/.temp/active_worktree.env" <<ENVEOF
WORKTREE_TASK_ID=$task_id
WORKTREE_DIR=$worktree_dir
PROJECT_ROOT=$project_root
WORKTREE_BRANCH=$branch
ENVEOF
}

# Read branch from worktree metadata or git.
# Usage: get_worktree_branch "/path/to/worktree"
get_worktree_branch() {
    local worktree_dir="$1"
    local meta_file="$worktree_dir/.worktree-meta.yaml"

    if [[ -f "$meta_file" ]]; then
        grep -oP '^branch:\s*"?\K[^"]+' "$meta_file" | head -1
        return 0
    fi

    # Fallback: ask git
    git -C "$worktree_dir" branch --show-current 2>/dev/null
}
