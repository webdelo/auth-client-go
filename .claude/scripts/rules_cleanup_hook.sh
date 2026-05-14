#!/bin/bash
# Hook: UserPromptSubmit
# Удаляет временные файлы правил ТОЛЬКО текущей сессии при вызове /clear.
# Правила создаются хуком inject_rules_hook.sh в .temp/rules/<task_id>/.
# Маркер .active_task_ids хранит task_id'ы текущей сессии.
#
# Привязан к UserPromptSubmit вместо Stop, потому что Stop срабатывает
# после КАЖДОГО хода ассистента, что ломает multi-turn workflows.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RULES_DIR="$PROJECT_ROOT/.temp/rules"
MARKER="$RULES_DIR/.active_task_ids"

# Read the prompt from stdin
input=$(cat)

# Only cleanup on /clear command
echo "$input" | grep -qiP '^\s*/clear\s*$' || exit 0

# No marker — nothing to clean
[[ -f "$MARKER" ]] || exit 0

# Delete only directories for task_ids registered in this session
while IFS= read -r task_id; do
    [[ -z "$task_id" ]] && continue
    target="$RULES_DIR/$task_id"
    [[ -d "$target" ]] && rm -rf "${target:?}"
done < "$MARKER"

# Remove the marker itself
rm -f "$MARKER"

exit 0
