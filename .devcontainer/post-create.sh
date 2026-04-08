#!/usr/bin/env bash
set -euo pipefail

echo "=== Colink DevContainer Setup ==="

cd /workspaces/isdp

# ── Go dependencies ─────────────────────────────────────────
echo "[1/6] Installing Go dependencies..."
go mod download

# ── Frontend dependencies ───────────────────────────────────
echo "[2/6] Installing frontend dependencies..."
cd /workspaces/isdp/web
rm -rf node_modules
npm install

# ── Playwright browsers ─────────────────────────────────────
echo "[3/6] Installing Playwright browsers..."
npx playwright install chromium 2>/dev/null || echo "  Playwright install skipped (non-fatal)"

# ── Dev config ──────────────────────────────────────────────
echo "[4/6] Generating dev config..."
cd /workspaces/isdp

DEV_CONFIG="configs/config.yaml"
if [ ! -f "$DEV_CONFIG" ]; then
    cat > "$DEV_CONFIG" <<'YAML'
server:
  port: 8080
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
  base_url: http://localhost:8080/api/v1/mcp
  token_ttl: 30m

auth:
  invite_code: ""

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
echo "[5/6] Creating data directories..."
sudo mkdir -p data/configs data/logs data/agent-assets data/agent-configs data/repos
sudo chown -R "$(id -u):$(id -g)" data

# ── Verify tooling ──────────────────────────────────────────
echo "[6/6] Verifying tooling..."
echo "  go:    $(go version | awk '{print $3}')"
echo "  node:  $(node --version)"
echo "  npm:   $(npm --version)"

if claude --version &>/dev/null; then echo "  claude: OK"; else echo "  claude: NOT FOUND"; fi
if opencode --version &>/dev/null; then echo "  opencode: OK"; else echo "  opencode: NOT FOUND"; fi
if golangci-lint --version &>/dev/null; then echo "  golangci-lint: OK"; else echo "  golangci-lint: NOT FOUND"; fi

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
