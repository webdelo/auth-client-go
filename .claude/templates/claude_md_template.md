# CLAUDE.md Template (Optimized)
# Используется командой /refresh_memory_bank для генерации компактного CLAUDE.md
# Детали вынесены в отдельные guides для ленивой загрузки
# Секции с {{PLACEHOLDER}} заполняются автоматически

---

# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

{{PROJECT_OVERVIEW}}

## Quick Reference

| Resource | Path |
|----------|------|
| **Memory Bank** | |
| Tech Stack | `.memory_bank/tech_stack.yaml:1` |
| Project Map | `.memory_bank/project_map.yaml:1` |
| External Libs | `.memory_bank/external_libs.yaml:1` |
| Dev Commands | `.memory_bank/dev_commands.yaml:1` |
| Tasks List | `.memory_bank/tasks_list.yaml:1` |
| **Guides** | |
| Task Management | `.claude/guides/project/task_management_guide.md:1` |
| File Organization | `.claude/guides/project/file_organization.md:1` |
| Clickable References | `.claude/guides/project/clickable_references.md:1` |
| Rules Index | `.claude/guides/rules_index.yaml:1` |
| Agent Catalog | `.claude/AGENTS.yaml:1` |

## Essential Commands

```bash
# Docker
make dev-up-env          # Start infrastructure
make dev-up-app          # Start services
make dev-down            # Stop all
make dev-logs SERVICE=x  # View logs

# Tests (ALWAYS use make, never raw go commands)
make test                # Run all tests
make test-race           # With race detector

# Build (ALWAYS use make, never raw go build)
make build-local-service SERVICE=trading
```

> **⚠️ Build/Test Policy:**
> 1. **ALWAYS** use `make` commands from `.memory_bank/dev_commands.yaml:1`
> 2. If no suitable `make` command exists → use `-o .temp/` flag explicitly
> 3. **FORBIDDEN:** `go build` or `go test -c` without output flag — pollutes project root

## Architecture

{{ARCHITECTURE}}

## Logging

```go
log.General().Info("message")
log.Custom("channel").Error("message")
```

## Key Dependencies

{{KEY_DEPENDENCIES}}

---

## ⚠️ CRITICAL POLICIES

### 1. Mandatory Task Execution

**ALL items are MANDATORY** — PRP points, DOD criteria, CRITICAL/WARNING/SUGGESTIONS from reviews, bugs from reports.

- ❌ **FORBIDDEN:** Decide "out of scope" or "not critical" without user approval
- ❌ **FORBIDDEN:** Skip SUGGESTIONS as "optional"
- ✅ **REQUIRED:** Execute ALL items or explicitly ask user for permission to skip

### 2. Rules Loading

Before ANY coding task:
1. Read `.claude/guides/rules_index.yaml:1`
2. Find triggers matching task (golang, redis, postgres...)
3. Load relevant guides
4. Apply rules

### 3. Agent Usage

- Use `run_in_background: true` for all agents
- See `.claude/AGENTS.yaml:1` for agent catalog
- See `.claude/guides/rules_index.yaml:29` for orchestrator vs agent decision

### 4. Git Commits

- ❌ **FORBIDDEN:** Any AI attribution (Co-Authored-By, Claude, Anthropic)
- ✅ **REQUIRED:** Commits authored exclusively by user
- Enforced by `commit-msg` hook

### 5. File References

**Two formats by context:**

| Context | Format | Example |
|---------|--------|---------|
| CLI output | `path:line` | `domain/services/trader.go:142` |
| MD files | `[text](path)` or plain path | `[trader.go](domain/services/trader.go)` |

**Rules:**
- CLI: Always include line number (`:142` or `:1` for general ref)
- MD files: Use markdown links or plain paths, **never** `path:1`

See `.claude/guides/project/clickable_references.md:1`

### 6. File Organization

- Reports → `task_management/reviews/` or `.memory_bank/agent_task_results/`
- Results → `task_management/results/`
- Temp files → `.temp/`
- ❌ Never create files in project root

See `.claude/guides/project/file_organization.md:1`
