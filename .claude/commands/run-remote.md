---
description: Execute commands on remote server wdtrade@65.109.64.175 via SSH
argument-hint: <task description or commands to execute>
---

# Run Remote

Arguments: $ARGUMENTS

All commands execute on remote server `wdtrade@65.109.64.175` via SSH.

## 1. Rules

- All Bash commands wrapped in `ssh wdtrade@65.109.64.175 "..."`.
- Interactive commands (vim, htop, tmux attach) NOT supported. Non-interactive only.
- Default timeout: 30s. For long operations: 120s.

## 2. Command Format

Single command:
```bash
ssh wdtrade@65.109.64.175 "<command>"
```

Multi-line:
```bash
ssh wdtrade@65.109.64.175 bash -c '
  command1
  command2
'
```

## 3. Task

Execute the user's request: $ARGUMENTS
