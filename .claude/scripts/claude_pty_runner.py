#!/usr/bin/env python3 -u
"""
Запускает claude -p через PTY, читает stream-json в реальном времени,
выводит отфильтрованный текст в stdout (→ Shell details) и лог-файл.

Usage:
  python3 -u .claude/scripts/claude_pty_runner.py <prompt> <log_file>
"""
import sys
import os
import pty
import json
import subprocess
from datetime import datetime

CLAUDE_BIN = "/home/gremlin/.local/bin/claude"


def format_event(ev):
    ts = datetime.now().strftime("%H:%M:%S")
    t = ev.get("type", "")
    if t == "assistant":
        for block in ev.get("message", {}).get("content", []):
            bt = block.get("type", "")
            if bt == "text" and block.get("text", "").strip():
                text = block["text"].strip().replace("\n", " ")[:120]
                return f"{ts} | 📝 {text}"
            elif bt == "tool_use":
                name = block.get("name", "")
                inp = block.get("input", {})
                if name == "Bash":
                    return f"{ts} | 💻 Bash: {inp.get('command', '')[:80]}"
                elif name == "Read":
                    return f"{ts} | 📖 Read: {inp.get('file_path', '').split('/')[-1]}"
                elif name == "Write":
                    return f"{ts} | ✍️  Write: {inp.get('file_path', '').split('/')[-1]}"
                elif name == "Edit":
                    return f"{ts} | ✏️  Edit: {inp.get('file_path', '').split('/')[-1]}"
                else:
                    return f"{ts} | 🔧 {name}"
    elif t == "result":
        status = "✅ SUCCESS" if not ev.get("is_error") else "❌ FAILED"
        duration = ev.get("duration_ms", 0) // 1000
        cost = ev.get("total_cost_usd", 0)
        turns = ev.get("num_turns", 0)
        return f"\n{'='*50}\n{ts} | {status} | ⏱️ {duration}s | 💰 ${cost:.4f} | 🔄 {turns} turns\n{'='*50}"
    return None


def emit(text, log_fh):
    line = (text + "\n").encode()
    os.write(1, line)  # direct write to stdout fd, no Python buffering
    log_fh.write(line)
    log_fh.flush()


def main():
    if len(sys.argv) < 3:
        print("Usage: claude_pty_runner.py <prompt> <log_file>", file=sys.stderr)
        sys.exit(1)

    prompt = sys.argv[1]
    log_file = sys.argv[2]

    master_fd, slave_fd = pty.openpty()

    env = {k: v for k, v in os.environ.items()
           if k not in ("CLAUDECODE", "CLAUDE_CODE_SSE_PORT", "CLAUDE_CODE_ENTRYPOINT")}

    proc = subprocess.Popen(
        [CLAUDE_BIN, "-p", "--verbose", "--output-format", "stream-json",
         "--dangerously-skip-permissions"],
        stdin=subprocess.PIPE,
        stdout=slave_fd,
        stderr=slave_fd,
        env=env,
        close_fds=True,
    )
    os.close(slave_fd)
    proc.stdin.write(prompt.encode())
    proc.stdin.close()

    buf = b""
    with open(log_file, "ab") as log_fh:
        while True:
            try:
                data = os.read(master_fd, 4096)
            except OSError:
                break
            if not data:
                break
            # Strip ANSI/terminal control sequences
            clean = b""
            i = 0
            while i < len(data):
                if data[i:i+1] == b"\x1b":
                    # Skip escape sequence
                    i += 1
                    if i < len(data) and data[i:i+1] in (b"[", b"]", b"(", b")"):
                        seq_type = data[i:i+1]
                        i += 1
                        if seq_type == b"]":
                            # OSC sequence: ends at BEL or ST
                            while i < len(data) and data[i:i+1] not in (b"\x07", b"\x1b"):
                                i += 1
                            if i < len(data) and data[i:i+1] == b"\x1b":
                                i += 2  # skip ST \x1b\\
                            else:
                                i += 1
                        else:
                            # CSI sequence: ends at letter
                            while i < len(data) and not (0x40 <= data[i] <= 0x7e):
                                i += 1
                            i += 1
                    continue
                elif data[i] in (0x07, 0x08, 0x0d):  # BEL, BS, CR
                    i += 1
                    continue
                else:
                    clean += data[i:i+1]
                    i += 1

            buf += clean
            while b"\n" in buf:
                line, buf = buf.split(b"\n", 1)
                line_str = line.decode("utf-8", errors="replace").strip()
                if not line_str:
                    continue
                try:
                    ev = json.loads(line_str)
                    msg = format_event(ev)
                    if msg:
                        emit(msg, log_fh)
                except json.JSONDecodeError:
                    pass

    os.close(master_fd)
    proc.wait()


if __name__ == "__main__":
    main()
