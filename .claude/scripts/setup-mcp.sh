#!/usr/bin/env bash
set -euo pipefail

# Setup MCP server dependencies.
# Moves node_modules out of .claude/ into .memory_bank/ to keep .claude/ lightweight.
# Uses symlink (Linux/macOS) or junction (Windows/MSYS/Git Bash) so Node.js resolves modules transparently.

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

MCP_SRC="${PROJECT_ROOT}/.claude/mcp/web-fetch"
MCP_DEST="${PROJECT_ROOT}/.memory_bank/mcp_node_modules/web-fetch"

# --- Pre-flight checks ---

if ! command -v node >/dev/null 2>&1; then
    echo "[setup-mcp] SKIP: node is not installed. MCP web-fetch server won't work."
    exit 0
fi

if ! command -v npm >/dev/null 2>&1; then
    echo "[setup-mcp] SKIP: npm is not installed. MCP web-fetch server won't work."
    exit 0
fi

if [ ! -f "${MCP_SRC}/package.json" ]; then
    echo "[setup-mcp] SKIP: ${MCP_SRC}/package.json not found."
    exit 0
fi

# --- Detect OS ---

create_link() {
    local target="$1" link_name="$2"
    case "$(uname -s)" in
        MINGW*|MSYS*|CYGWIN*)
            # Windows: use directory junction (no admin rights needed)
            cmd //c "mklink /J \"$(cygpath -w "${link_name}")\" \"$(cygpath -w "${target}")\"" >/dev/null 2>&1 \
                || cp -r "${target}" "${link_name}"
            ;;
        *)
            ln -s "$(realpath --relative-to="$(dirname "${link_name}")" "${target}")" "${link_name}"
            ;;
    esac
}

# --- Install dependencies ---

mkdir -p "${MCP_DEST}"

if [ ! -d "${MCP_DEST}/node_modules" ]; then
    echo "[setup-mcp] Installing MCP dependencies into .memory_bank/mcp_node_modules/web-fetch/ ..."
    cp "${MCP_SRC}/package.json" "${MCP_DEST}/package.json"
    npm install --prefix "${MCP_DEST}" \
        --no-fund --no-audit --loglevel=error
else
    echo "[setup-mcp] MCP dependencies already installed, skipping."
fi

# --- Create link ---

if [ -L "${MCP_SRC}/node_modules" ] || [ -d "${MCP_SRC}/node_modules" ]; then
    # Check if existing link/dir points to the right place
    if [ -L "${MCP_SRC}/node_modules" ]; then
        echo "[setup-mcp] Symlink already exists."
    else
        echo "[setup-mcp] Removing existing node_modules from .claude/ and creating link..."
        rm -rf "${MCP_SRC}/node_modules"
        create_link "${MCP_DEST}/node_modules" "${MCP_SRC}/node_modules"
    fi
else
    echo "[setup-mcp] Creating link..."
    create_link "${MCP_DEST}/node_modules" "${MCP_SRC}/node_modules"
fi

echo "[setup-mcp] Done."
