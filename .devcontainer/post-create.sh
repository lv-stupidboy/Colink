#!/usr/bin/env bash
set -euo pipefail

echo "=== Colink DevContainer Setup ==="

cd /workspaces/isdp

# ── Go dependencies ─────────────────────────────────────────
echo "[1/8] Installing Go dependencies..."
go mod download

# ── Frontend dependencies ───────────────────────────────────
echo "[2/8] Installing frontend dependencies..."
cd /workspaces/isdp/web
rm -rf node_modules
npm install

# ── Playwright browsers ─────────────────────────────────────
echo "[3/8] Installing Playwright browsers..."
npx playwright install chromium 2>/dev/null || echo "  Playwright install skipped (non-fatal)"

# ── agent-browser ───────────────────────────────────────────
echo "[4/8] Installing agent-browser..."
npm install -g agent-browser 2>/dev/null || echo "  agent-browser install skipped"

# Chromium: ARM64 has no Chrome for Testing, use system package
ARCH=$(uname -m)
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    if ! chromium --version &>/dev/null; then
        sudo apt-get update -qq && sudo apt-get install -y -qq chromium 2>/dev/null \
            || echo "  Chromium install skipped"
    fi
    mkdir -p ~/.agent-browser
    if [ ! -f ~/.agent-browser/config.json ]; then
        echo '{"executablePath":"/usr/bin/chromium"}' > ~/.agent-browser/config.json
    fi
else
    agent-browser install 2>/dev/null || echo "  agent-browser Chrome install skipped"
fi

# ── agent-browser skills for opencode ───────────────────────
echo "[5/8] Installing agent-browser skills..."
npx skills add vercel-labs/agent-browser --yes 2>/dev/null || echo "  agent-browser skills install skipped"

# ── Dev config ──────────────────────────────────────────────
echo "[6/8] Generating dev config..."
cd /workspaces/isdp

DEV_CONFIG="configs/config.yaml"
if [ ! -f "$DEV_CONFIG" ]; then
    cat > "$DEV_CONFIG" <<'YAML'
server:
  port: 26305
  mode: debug

data:
  base_path: ./data

database:
  type: sqlite
  path: ./data/isdp.db

redis:
  addr: localhost:6379
  password: ""
  db: 0

claude:
  path: claude
  default_model: claude-sonnet-4-6
  timeout: 30m

sandbox:
  port_range_start: 30000
  port_range_end: 40000
  default_cpu_limit: 2
  default_memory_limit: 4096
  network: isdp-network
  repos_dir: ./data/repos

agent:
  max_depth: 15
  max_retries: 3
  context_max_lines: 400

logging:
  level: info
  format: json

mcp:
  base_url: http://localhost:26305/api/v1/mcp
  token_ttl: 30m

agent_assets:
  base_path: ./data/agent-assets

skill:
  use_count_update_interval: "1h"
  upload_max_size: 5

subagent:
  upload_max_size: 2

command:
  upload_max_size: 2

rule:
  upload_max_size: 2

agent_config:
  data_dir: ./data/agent-configs

feishu:
  enabled: false
  app_id: ""
  app_secret: ""
  verification_token: ""
  encrypt_key: ""
  lark_cli_path: lark-cli
  default_project_id: ""
YAML
    echo "  Created configs/config.yaml (SQLite mode)"
else
    echo "  configs/config.yaml already exists, skipped"
fi

# ── Data directories ────────────────────────────────────────
echo "[7/8] Creating data directories..."
sudo mkdir -p data/configs data/logs data/agent-assets data/agent-configs data/repos
sudo chown -R "$(id -u):$(id -g)" data

# ── Verify tooling ──────────────────────────────────────────
echo "[8/8] Verifying tooling..."
echo "  go:    $(go version | awk '{print $3}')"
echo "  node:  $(node --version)"
echo "  npm:   $(npm --version)"

if claude --version &>/dev/null; then echo "  claude: OK"; else echo "  claude: NOT FOUND"; fi
if opencode --version &>/dev/null; then echo "  opencode: OK"; else echo "  opencode: NOT FOUND"; fi
if golangci-lint --version &>/dev/null; then echo "  golangci-lint: OK"; else echo "  golangci-lint: NOT FOUND"; fi
if agent-browser --version &>/dev/null; then echo "  agent-browser: $(agent-browser --version)"; else echo "  agent-browser: NOT FOUND"; fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Backend:  make run"
echo "Frontend: cd web && npm run dev"
echo "Tests:    make test"
echo "E2E:      cd web && npm run test:e2e"
echo ""
echo "MySQL: localhost:3306 (isdp / isdp_dev_pass / isdp_dev)"
echo "Redis:  localhost:6379"
echo ""
echo "To switch to MySQL: edit configs/config.yaml → database.type: mysql"
