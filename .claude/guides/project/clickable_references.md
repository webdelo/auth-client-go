# Clickable File References Policy

> All file references must be clickable — in Claude Code CLI and in Markdown renderers (GitHub, IDE).

## Two Formats by Context

| Context | Format | Example |
|---------|--------|---------|
| **Claude Code CLI** (inline text) | `path:line` | `domain/entities/order.go:142` |
| **Markdown documents** (.md files) | `[text](path)` | `[order.go](../../domain/entities/order.go)` |

## Format 1: CLI References (`path:line`)

Use for code references in Claude Code CLI output:

| ✅ Correct | ❌ Incorrect |
|-----------|-------------|
| `domain/entities/order.go:142` | `domain/entities/order.go` |
| `config/loader.go:25` | `config/loader.go (line 25)` |

**Rules**:
1. Always include line number — CLI only makes links clickable with `:line`
2. Use `:1` for general references when specific line not needed
3. Use actual line numbers from Grep when referencing specific code

## Format 2: Markdown Links (`[text](path)`)

Use for references **inside .md files** (PRP, TASK, REQ, CODE-REVIEW, etc.):

| ✅ Correct | ❌ Incorrect |
|-----------|-------------|
| `[REQ-002](../tasks/REQ-002.md)` | `task_management/tasks/REQ-002.md:1` |
| `[Golden Signals Analysis](../session_analysis/2026-01-28-analysis.md)` | `"path/to/file.md:1"` in YAML block |
| `[prometheus.yml](../../tools/kafka/configs/prometheus/prometheus.yml)` | plain path without link |

**Rules**:
1. Use **relative paths** from the current file location
2. Link text should be **descriptive** (file name or short description)
3. For cross-directory links, use `../` to navigate up

### Examples in PRP/TASK files

```markdown
## References

- **Parent Request**: [REQ-002](../tasks/REQ-002.md)
- **Parent Task**: [REQ-002-TASK-005](../tasks/REQ-002-TASK-005.md)
- **Analysis**: [Golden Signals Analysis](../session_analysis/2026-01-28-REQ-002-TASK-005-GOLDEN-SIGNALS-ANALYSIS.md)
- **Code Research**: [prometheus-grafana research](../../.memory_bank/agent_task_results/2026-01-28_code-research_prometheus-grafana.yaml)

## Files to Modify

- [prometheus.yml](../../tools/kafka/configs/prometheus/prometheus.yml) — add alert rules
- [docker-compose.yaml](../../docker-compose.yaml) — add volume mount
```

### Inline code references (still use `path:line`)

When referencing specific code locations in explanatory text, use CLI format:

```markdown
The bug is located in `domain/services/registry/registry.go:88` where the mutex is not released.
```

## Summary

| File Type | Link Format |
|-----------|-------------|
| `.md` document links | `[text](relative/path.md)` |
| Code location references | `path/file.go:line` |
| YAML frontmatter | Plain paths (not clickable) |
