# Улучшенный jq фильтр для читаемого вывода claude --output-format stream-json
# Использование: claude -p --verbose --output-format stream-json ... | jq --unbuffered -r -f claude_stream_filter.jq

def timestamp: now | strftime("%H:%M:%S");

def format_tool:
  if .name == "Read" then "📖 Read: " + (.input.file_path | split("/") | .[-1])
  elif .name == "Write" then "✍️  Write: " + (.input.file_path | split("/") | .[-1])
  elif .name == "Edit" then "✏️  Edit: " + (.input.file_path | split("/") | .[-1])
  elif .name == "Task" then "🤖 Agent [" + (.input.subagent_type // "agent") + "]: " + (.input.description // "")
  elif .name == "Grep" then "🔍 Grep: " + (.input.pattern // "")
  elif .name == "Glob" then "📁 Glob: " + (.input.pattern // "")
  elif .name == "Bash" then "💻 Bash: " + ((.input.command // "")[0:60])
  elif .name == "TodoWrite" then "📋 TodoWrite"
  elif .name == "WebSearch" then "🌐 WebSearch: " + (.input.query // "")
  elif .name == "WebFetch" then "🔗 WebFetch: " + ((.input.url // "")[0:60])
  else "🔧 " + .name
  end;

if .type == "assistant" then
  .message.content[]? |
    if .type == "text" then "\n" + timestamp + " | 📝\n\(.text)\n"
    elif .type == "tool_use" then timestamp + " | " + format_tool
    else empty end
elif .type == "result" then
  "\n============================================================\n" +
  timestamp + " | " +
  (if .is_error then "❌ FAILED" else "✅ SUCCESS" end) +
  " | ⏱️ " + ((.duration_ms / 1000 | floor | tostring) + "s") +
  " | 💰 $" + (.total_cost_usd | tostring) +
  " | 🔄 " + (.num_turns | tostring) + " turns" +
  "\n============================================================"
else empty end
