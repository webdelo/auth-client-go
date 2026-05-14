import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import { Readability } from "@mozilla/readability";
import TurndownService from "turndown";
import { parseHTML } from "linkedom";

const turndown = new TurndownService({
  headingStyle: "atx",
  codeBlockStyle: "fenced",
  bulletListMarker: "-",
});

// Remove non-text elements (addRule takes priority over built-in rules like 'image')
turndown.addRule("removeNonText", {
  filter: [
    "img",
    "figure",
    "picture",
    "video",
    "audio",
    "iframe",
    "svg",
    "canvas",
    "style",
    "script",
    "noscript",
  ],
  replacement: () => "",
});

const server = new McpServer({
  name: "web-fetch",
  version: "1.0.0",
});

server.tool(
  "fetch_page",
  "Fetch a web page, extract article content with Readability, convert to Markdown. Supports timeout.",
  {
    url: z.string().url().describe("URL to fetch"),
    timeout_seconds: z
      .number()
      .min(1)
      .max(120)
      .default(30)
      .describe("Request timeout in seconds (default 30, max 120)"),
    max_length: z
      .number()
      .min(100)
      .max(100000)
      .default(20000)
      .describe("Max characters in output (default 20000)"),
    raw: z
      .boolean()
      .default(false)
      .describe("Skip Readability, apply Turndown to full body"),
  },
  async ({ url, timeout_seconds = 30, max_length = 20000, raw = false }) => {
    try {
      const response = await fetch(url, {
        signal: AbortSignal.timeout(timeout_seconds * 1000),
        headers: {
          "User-Agent":
            "Mozilla/5.0 (compatible; MCP-WebFetch/1.0; +https://github.com/anthropics/claude-code)",
          Accept:
            "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
          "Accept-Language": "en-US,en;q=0.9",
        },
        redirect: "follow",
      });

      if (!response.ok) {
        return error(`HTTP ${response.status} ${response.statusText}`);
      }

      const contentType = response.headers.get("content-type") || "";
      if (
        !contentType.includes("text/html") &&
        !contentType.includes("application/xhtml") &&
        !contentType.includes("text/plain")
      ) {
        return error(
          `Unsupported content type: ${contentType.split(";")[0].trim()}`
        );
      }

      const html = await response.text();
      if (!html || html.trim().length === 0) {
        return error("Empty response body");
      }

      let markdown;
      const { document } = parseHTML(html);

      if (!raw) {
        const reader = new Readability(document);
        const article = reader.parse();

        if (article && article.content) {
          const title = article.title || "";
          const md = turndown.turndown(article.content);
          markdown = title ? `# ${title}\n\n${md}` : md;
        } else {
          // Fallback: Readability failed, use full body
          const body = document.body;
          markdown = body
            ? turndown.turndown(body.innerHTML)
            : turndown.turndown(html);
        }
      } else {
        const body = document.body;
        markdown = body
          ? turndown.turndown(body.innerHTML)
          : turndown.turndown(html);
      }

      // Clean up excessive whitespace
      markdown = markdown.replace(/\n{3,}/g, "\n\n").trim();

      // Truncate
      if (markdown.length > max_length) {
        markdown =
          markdown.slice(0, max_length) +
          `\n\n--- truncated at ${max_length} characters ---`;
      }

      const header = `**Source:** ${url}\n**Length:** ${markdown.length} chars\n\n---\n\n`;

      return {
        content: [{ type: "text", text: header + markdown }],
      };
    } catch (err) {
      if (err.name === "TimeoutError" || err.name === "AbortError") {
        return error(`Timeout after ${timeout_seconds}s fetching ${url}`);
      }
      return error(`Fetch failed: ${err.message}`);
    }
  }
);

function error(message) {
  return {
    content: [{ type: "text", text: `Error: ${message}` }],
    isError: true,
  };
}

const transport = new StdioServerTransport();
await server.connect(transport);
