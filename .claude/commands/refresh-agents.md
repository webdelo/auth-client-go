---
description: Update agent catalog .claude/AGENTS.yaml from system definitions
---

# Refresh Agents

Update `.claude/AGENTS.yaml` metadata from system agent definitions, preserving manual triggers.

## 1. Rules

| Field | Source | Action |
|-------|--------|--------|
| name, purpose, model, tools | System definitions | Update |
| background | Rules (see below) | Update |
| category, triggers, when_to_use, mandatory_for | Manual config | Preserve |

Background = false ONLY for: bash-command-runner, git-commit-manager, git-pr-manager.
All others: background = true.

## 2. Read Current AGENTS.yaml

1. Read `.claude/AGENTS.yaml`.
2. Save category structure, all triggers, when_to_use, mandatory_for.

## 3. Get System Definitions

1. Read agent definitions from Tool descriptions (available in system prompt).
2. Extract: subagent_type, purpose, model, tools.

## 4. Update AGENTS.yaml

1. For each existing agent: update name, purpose, model, tools, background. Preserve triggers and other manual fields.
2. New agents (in system, not in AGENTS.yaml): add with `# TODO: Define triggers`.
3. Removed agents (in AGENTS.yaml, not in system): add `# DEPRECATED` comment. Do NOT delete.

## 5. Verify Consistency

1. Check agents referenced in workflows exist.
2. Check agents in rules_index.yaml match.
3. Validate YAML: `python3 -c "import yaml; yaml.safe_load(open('.claude/AGENTS.yaml'))"`.

## 6. Save

1. Write updated `.claude/AGENTS.yaml`.
2. Update `generation.last_updated`.

## 7. Completion Checklist

- [ ] Current AGENTS.yaml read.
- [ ] System definitions extracted.
- [ ] Agents updated (name, purpose, model, tools, background).
- [ ] Triggers preserved for all existing agents.
- [ ] New agents added with TODO comment.
- [ ] Deprecated agents marked.
- [ ] Workflow consistency checked.
- [ ] YAML validated.
- [ ] File saved.
