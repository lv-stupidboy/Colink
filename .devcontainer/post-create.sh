#!/usr/bin/env bash
set -euo pipefail

echo "=== ISDP DevContainer Setup ==="

# ── Go backend ──────────────────────────────────────────────
echo "[1/6] Installing Go dependencies..."
cd /workspaces/isdp/isdp
export GOMODCACHE="${HOME}/.cache/go-mod"
go mod download

# ── Frontend ────────────────────────────────────────────────
echo "[2/6] Installing frontend dependencies..."
cd /workspaces/isdp/isdp/web
rm -rf node_modules
npm install

# ── Playwright browsers ─────────────────────────────────────
echo "[3/6] Installing Playwright browsers..."
npx playwright install chromium 2>/dev/null || echo "  Playwright install skipped (non-fatal)"

# ── Dev config ──────────────────────────────────────────────
echo "[4/6] Generating dev config..."
cd /workspaces/isdp/isdp

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
YAML
    echo "  Created configs/config.yaml (SQLite mode)"
else
    echo "  configs/config.yaml already exists, skipped"
fi

# ── Data directories ────────────────────────────────────────
echo "[5/6] Creating data directories..."
mkdir -p data/configs data/logs data/agent-assets data/agent-configs data/repos

echo "[6/6] Verifying agent CLIs..."
claude --version 2>/dev/null && echo "  claude: OK" || echo "  claude: NOT FOUND (run: npm i -g @anthropic-ai/claude-code)"
opencode --version 2>/dev/null && echo "  opencode: OK" || echo "  opencode: NOT FOUND (run: npm i -g @opencode-ai/opencode)"

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Backend:  cd isdp && make run"
echo "Frontend: cd isdp/web && npm run dev"
echo "Tests:    cd isdp && make test"
echo "E2E:      cd isdp/web && npm run test:e2e"
echo ""
echo "Agent CLIs: claude, opencode (credentials mounted from host ~/.claude, ~/.opencode)"
echo "MySQL available at localhost:3306 (user: isdp, pass: isdp_dev_pass, db: isdp_dev)"
echo "Redis available at localhost:6379"
echo ""
echo "To switch to MySQL: edit isdp/configs/config.yaml → database.type: mysql"
