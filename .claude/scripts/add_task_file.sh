#!/bin/bash
# Wrapper для добавления ссылок на файлы в tasks_list.yaml
# Использование: add_task_file.sh <task_id> <file_type> <file_path>

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 "$SCRIPT_DIR/req_status.py" add-file "$@"
