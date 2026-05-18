---
description: Refresh project memory bank and CLAUDE.md
---

# Refresh Memory Bank

## 1. Analyze Project

1. Read `go.mod`, `README.md`, current `CLAUDE.md`.
2. Analyze structure: `ls -la`, directory layout.

## 2. Generate Memory Bank Files

| File | Template | Source |
|------|----------|--------|
| `.memory_bank/tech_stack.yaml` | `.claude/templates/tech_stack_template.yaml` | go.mod, architecture |
| `.memory_bank/project_map.yaml` | `.claude/templates/project_map_template.yaml` | directory structure |
| `.memory_bank/external_libs.yaml` | `.claude/templates/external_libs_template.yaml` | go.mod (direct deps) |
| `.memory_bank/dev_commands.yaml` | `.claude/templates/dev_commands_template.yaml` | Makefile (.PHONY) |

1. For each: read template, fill with data, save.
2. `tasks_list.yaml` — do NOT overwrite if exists (working document).

## 3. Scratch Commands

1. Check `scratch.yaml` exists.
2. If yes: check `which scratch`, read `.claude/guides/libs/scratch_usage.md`, add `scratch:` category to `dev_commands.yaml`.
3. If no: remove `scratch:` category from `dev_commands.yaml`.

## 4. Update rules_index.yaml

1. Scan `.claude/` for new/deleted files.
2. Update `statistics` section.
3. Add new files / remove dead references.

## 5. Generate CLAUDE.md

1. Template: `.claude/templates/claude_md_template.md`.
2. Fill placeholders from project_map.yaml, tech_stack.yaml, external_libs.yaml.
3. Target size: ~130 lines, ~4KB.

## 6. Verify Directory Structure

```bash
mkdir -p .memory_bank/agent_task_results
mkdir -p task_management/{results,reviews,session_analysis}
mkdir -p .temp
grep "\.temp/" .gitignore || echo ".temp/" >> .gitignore
rm -rf .temp/*
ls -la | grep -E "TEST_|WORK_|REPORT|\.tmp$|\.test$"
```

Remove any stray files found in project root.

## 7. Setup MCP Dependencies

```bash
bash .claude/scripts/setup-mcp.sh
```

## 8. Final Verification

All files exist:
- `.memory_bank/tech_stack.yaml`
- `.memory_bank/project_map.yaml`
- `.memory_bank/external_libs.yaml`
- `.memory_bank/dev_commands.yaml`
- `.memory_bank/tasks_list.yaml`
- `.claude/guides/rules_index.yaml`
- `.claude/AGENTS.yaml`
- `CLAUDE.md` with Quick Reference table, ~130 lines.

## 9. Completion Checklist

- [ ] Project analyzed (go.mod, structure).
- [ ] All 4 memory bank files generated from templates.
- [ ] tasks_list.yaml preserved (not overwritten).
- [ ] Scratch commands populated (if scratch.yaml exists).
- [ ] rules_index.yaml updated.
- [ ] CLAUDE.md generated (~130 lines).
- [ ] Directory structure verified.
- [ ] MCP dependencies set up.
- [ ] All files exist.
