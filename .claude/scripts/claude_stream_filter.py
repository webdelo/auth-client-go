#!/usr/bin/env python3 -u
"""
Stream filter for claude -p --verbose --output-format stream-json
Replaces jq filter with guaranteed unbuffered output (flush=True on every line).

Usage:
  echo "<prompt>" | claude -p --verbose --output-format stream-json ... 2>&1 | python3 -u .claude/scripts/claude_stream_filter.py
"""
import sys
import json
from datetime import datetime


def format_tool(name, inp):
    if name == "Read":
        return "📖 Read: " + inp.get("file_path", "").split("/")[-1]
    elif name == "Write":
        return "✍️  Write: " + inp.get("file_path", "").split("/")[-1]
    elif name == "Edit":
        return "✏️  Edit: " + inp.get("file_path", "").split("/")[-1]
    elif name == "Bash":
        return "💻 Bash: " + inp.get("command", "")[:80]
    elif name == "Grep":
        return "🔍 Grep: " + inp.get("pattern", "")
    elif name == "Glob":
        return "📁 Glob: " + inp.get("pattern", "")
    elif name == "Task":
        return "🤖 Agent [" + inp.get("subagent_type", "agent") + "]: " + inp.get("description", "")
    elif name == "TodoWrite":
        return "📋 TodoWrite"
    elif name == "WebSearch":
        return "🌐 WebSearch: " + inp.get("query", "")
    elif name == "WebFetch":
        return "🔗 WebFetch: " + inp.get("url", "")[:80]
    else:
        return "🔧 " + name


def out(text):
    print(text, flush=True)


for raw in sys.stdin:
    raw = raw.strip()
    if not raw:
        continue
    try:
        ev = json.loads(raw)
    except json.JSONDecodeError:
        continue

    ts = datetime.now().strftime("%H:%M:%S")
    t = ev.get("type", "")

    if t == "assistant":
        for block in ev.get("message", {}).get("content", []):
            bt = block.get("type", "")
            if bt == "text":
                out(f"\n{ts} | 📝\n{block['text']}\n")
            elif bt == "tool_use":
                out(f"{ts} | {format_tool(block['name'], block.get('input', {}))}")

    elif t == "result":
        status = "❌ FAILED" if ev.get("is_error") else "✅ SUCCESS"
        duration = ev.get("duration_ms", 0) // 1000
        cost = ev.get("total_cost_usd", 0)
        turns = ev.get("num_turns", 0)
        out(
            f"\n{'=' * 60}\n"
            f"{ts} | {status} | ⏱️ {duration}s | 💰 ${cost:.4f} | 🔄 {turns} turns\n"
            f"{'=' * 60}"
        )
