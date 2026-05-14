# web-fetch MCP Server

MCP server for fetching web pages with timeout support. Extracts article content via Readability, converts to Markdown via Turndown, and truncates to a configurable length.

## Setup

```bash
cd .claude/mcp/web-fetch && npm install
ln -s .claude/mcp/.mcp.json .mcp.json   # symlink to project root
```

Add permission in `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "mcp__web-fetch__fetch_page"
    ]
  }
}
```

Restart Claude Code after configuration.

MCP config lives in `.claude/mcp/.mcp.json`, symlinked to project root (Claude Code reads `.mcp.json` only from root).

## Tool: `fetch_page`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | string | required | URL to fetch |
| `timeout_seconds` | number | 30 | Request timeout, max 120 |
| `max_length` | number | 20000 | Max characters in output (~5K tokens) |
| `raw` | boolean | false | Skip Readability, apply Turndown to full body |

## Portability

To use in another project:
1. Copy `.claude/mcp/` (without `node_modules/`)
2. `cd .claude/mcp/web-fetch && npm install`
3. `ln -s .claude/mcp/.mcp.json .mcp.json` (symlink to project root)
4. Add `mcp__web-fetch__fetch_page` to `.claude/settings.json` permissions
5. Restart Claude Code
