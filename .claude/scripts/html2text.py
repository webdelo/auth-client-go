#!/usr/bin/env python3
"""Extract readable text content from HTML (stdin).

Usage: curl -sL <URL> | python3 .claude/scripts/html2text.py [max_chars]

Strips navigation, scripts, styles. Extracts <article>, <main>, or full body.
Default max output: 50000 chars.
"""
import sys
import re

def extract(html: str, max_chars: int = 50000) -> str:
    # Try content containers in priority order
    for tag in ['article', 'main', r'div\s+class="[^"]*content[^"]*"',
                r'div\s+class="[^"]*body[^"]*"', r'div\s+id="content"']:
        close_tag = tag.split('\\')[0].split()[0]
        pattern = rf'<{tag}[^>]*>(.*?)</{close_tag}>'
        match = re.search(pattern, html, re.DOTALL | re.IGNORECASE)
        if match and len(match.group(1)) > 500:
            html = match.group(1)
            break

    # Remove scripts, styles, nav, header, footer
    for strip_tag in ['script', 'style', 'nav', 'header', 'footer', 'noscript']:
        html = re.sub(rf'<{strip_tag}[^>]*>.*?</{strip_tag}>', '', html, flags=re.DOTALL | re.IGNORECASE)

    # Convert common elements to readable text
    html = re.sub(r'<br\s*/?>', '\n', html)
    html = re.sub(r'</?p[^>]*>', '\n', html)
    html = re.sub(r'</?h[1-6][^>]*>', '\n', html)
    html = re.sub(r'</?li[^>]*>', '\n- ', html)
    html = re.sub(r'</?(?:ul|ol|div|section|article|td|tr|th)[^>]*>', '\n', html)
    html = re.sub(r'<pre[^>]*>(.*?)</pre>', lambda m: '\n```\n' + re.sub(r'<[^>]+>', '', m.group(1)) + '\n```\n', html, flags=re.DOTALL)
    html = re.sub(r'<code[^>]*>(.*?)</code>', lambda m: '`' + re.sub(r'<[^>]+>', '', m.group(1)) + '`', html, flags=re.DOTALL)

    # Strip remaining tags
    text = re.sub(r'<[^>]+>', ' ', html)

    # Decode HTML entities
    text = text.replace('&amp;', '&').replace('&lt;', '<').replace('&gt;', '>')
    text = text.replace('&quot;', '"').replace('&#39;', "'").replace('&rsquo;', "'")
    text = text.replace('&lsquo;', "'").replace('&mdash;', '—').replace('&ndash;', '–')
    text = text.replace('&hellip;', '...').replace('&mldr;', '...')
    text = text.replace('&nbsp;', ' ')

    # Clean whitespace
    text = re.sub(r'[ \t]+', ' ', text)
    text = re.sub(r'\n\s*\n', '\n\n', text)
    text = text.strip()

    return text[:max_chars]


if __name__ == '__main__':
    max_chars = int(sys.argv[1]) if len(sys.argv) > 1 else 50000
    html = sys.stdin.read()
    print(extract(html, max_chars))
