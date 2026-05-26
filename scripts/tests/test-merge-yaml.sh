#!/bin/bash
# 测试 merge-yaml.py 配置合并辅助脚本
# 重点验证3层合并逻辑：模板结构 → 用户值 → 新增键/移除废弃键
#
# 用法: ./test-merge-yaml.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

PASS=0
FAIL=0

assert_ok() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo -e "  ${GREEN}PASS${NC}: $desc"
        PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: $desc"
        FAIL=$((FAIL+1))
    fi
}

assert_eq() {
    local desc="$1" actual="$2" expected="$3"
    if [[ "$actual" == "$expected" ]]; then
        echo -e "  ${GREEN}PASS${NC}: $desc"
        PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: $desc (expected='$expected', actual='$actual')"
        FAIL=$((FAIL+1))
    fi
}

assert_contains() {
    local desc="$1" haystack="$2" needle="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        echo -e "  ${GREEN}PASS${NC}: $desc"
        PASS=$((PASS+1))
    else
        echo -e "  ${RED}FAIL${NC}: $desc (expected to contain '$needle')"
        FAIL=$((FAIL+1))
    fi
}

echo -e "${YELLOW}=== test-merge-yaml.sh ===${NC}"

MERGE_SCRIPT="$PROJECT_ROOT/scripts/merge-yaml.py"
if [[ ! -f "$MERGE_SCRIPT" ]]; then
    echo -e "${RED}merge-yaml.py 不存在${NC}"; exit 1
fi

# 检查 python 和 yaml 模块
if ! python -c "import yaml" 2>/dev/null; then
    echo -e "${YELLOW}python 或 yaml 模块不可用，跳过合并测试${NC}"
    echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
    exit 0
fi

TMPDIR=$(mktemp -d)
trap "rm -rf '$TMPDIR'" EXIT

# ── Test 1: 用户值覆盖模板默认值 ──
echo ""
echo "--- 用户值覆盖模板 ---"
cat > "$TMPDIR/template1.yaml" << 'EOF'
server:
  port: 26305
  mode: release
logging:
  level: info
  format: console
EOF

cat > "$TMPDIR/user1.yaml" << 'EOF'
server:
  port: 3000
  mode: debug
logging:
  level: debug
EOF

python "$MERGE_SCRIPT" "$TMPDIR/template1.yaml" "$TMPDIR/user1.yaml" "$TMPDIR/merged1.yaml" 2>&1
RESULT=$(cat "$TMPDIR/merged1.yaml")
assert_contains "用户端口覆盖 (3000)" "$RESULT" "3000"
assert_contains "用户模式覆盖 (debug)" "$RESULT" "debug"
assert_contains "用户日志级别覆盖 (debug)" "$RESULT" "level: debug"
assert_contains "模板字段保留 (format: console)" "$RESULT" "format: console"

# ── Test 2: 模板新增键自动添加 ──
echo ""
echo "--- 模板新增键自动添加 ---"
cat > "$TMPDIR/template2.yaml" << 'EOF'
server:
  port: 26305
  mode: release
logging:
  level: info
  format: console
  dir: ./logs
deployment:
  type: linux
EOF

cat > "$TMPDIR/user2.yaml" << 'EOF'
server:
  port: 3000
  mode: release
logging:
  level: info
EOF

python "$MERGE_SCRIPT" "$TMPDIR/template2.yaml" "$TMPDIR/user2.yaml" "$TMPDIR/merged2.yaml" 2>&1
RESULT=$(cat "$TMPDIR/merged2.yaml")
assert_contains "新增 logging.dir" "$RESULT" "dir: ./logs"
assert_contains "新增 deployment 节" "$RESULT" "deployment"
assert_contains "新增 deployment.type" "$RESULT" "type: linux"
assert_contains "用户端口保留 (3000)" "$RESULT" "3000"

# ── Test 3: 用户配置中模板已删除的键被移除 ──
echo ""
echo "--- 废弃键移除 ---"
cat > "$TMPDIR/template3.yaml" << 'EOF'
server:
  port: 26305
  mode: release
logging:
  level: info
EOF

cat > "$TMPDIR/user3.yaml" << 'EOF'
server:
  port: 3000
  mode: release
logging:
  level: info
  format: console
old_section:
  deprecated_key: value
EOF

python "$MERGE_SCRIPT" "$TMPDIR/template3.yaml" "$TMPDIR/user3.yaml" "$TMPDIR/merged3.yaml" 2>&1
RESULT=$(cat "$TMPDIR/merged3.yaml")
assert_ok "废弃节 old_section 已移除" bash -c '! grep -q "old_section" "$TMPDIR/merged3.yaml"'
assert_ok "废弃键 logging.format 已移除" bash -c '! grep -q "format:" "$TMPDIR/merged3.yaml"'
assert_contains "有效键保留 (port: 3000)" "$RESULT" "3000"

# ── Test 4: 嵌套对象递归合并 ──
echo ""
echo "--- 嵌套对象递归合并 ---"
cat > "$TMPDIR/template4.yaml" << 'EOF'
database:
  type: sqlite
  path: ./data/sqlite/colink.db
git_url_conversion:
  enabled: false
  rules:
    - pattern: "https://gitee.com/"
      ssh_host: "git@gitee.com"
market:
  name: "Colink官方市场"
  url: "https://gitee.com/colink_1/colinkmarketplace.git"
  branch: "master"
EOF

cat > "$TMPDIR/user4.yaml" << 'EOF'
database:
  type: sqlite
  path: ./data/sqlite/colink.db
git_url_conversion:
  enabled: true
market:
  name: "Colink官方市场"
  url: "https://gitee.com/colink_1/colinkmarketplace.git"
  branch: "master"
EOF

python "$MERGE_SCRIPT" "$TMPDIR/template4.yaml" "$TMPDIR/user4.yaml" "$TMPDIR/merged4.yaml" 2>&1
RESULT=$(cat "$TMPDIR/merged4.yaml")
assert_contains "用户 git_url_conversion.enabled=true 保留" "$RESULT" "true"
assert_contains "模板 git_url_conversion.rules 添加" "$RESULT" "pattern"
assert_contains "database 完整保留" "$RESULT" "sqlite"

# ── Test 5: 数组整体替换 ──
echo ""
echo "--- 数组整体替换 ---"
cat > "$TMPDIR/template5.yaml" << 'EOF'
im:
  platforms:
    - type: feishu
      enabled: true
git_url_conversion:
  rules:
    - pattern: "https://gitee.com/"
      ssh_host: "git@gitee.com"
EOF

cat > "$TMPDIR/user5.yaml" << 'EOF'
im:
  platforms:
    - type: slack
      enabled: true
    - type: discord
      enabled: true
git_url_conversion:
  rules:
    - pattern: "https://github.com/"
      ssh_host: "git@github.com"
EOF

python "$MERGE_SCRIPT" "$TMPDIR/template5.yaml" "$TMPDIR/user5.yaml" "$TMPDIR/merged5.yaml" 2>&1
RESULT=$(cat "$TMPDIR/merged5.yaml")
assert_contains "用户数组整体替换 (slack)" "$RESULT" "slack"
assert_contains "用户数组整体替换 (discord)" "$RESULT" "discord"
assert_contains "用户 rules 替换 (github)" "$RESULT" "github"

# ── Test 6: 空用户配置 ──
echo ""
echo "--- 空用户配置 ---"
cat > "$TMPDIR/template6.yaml" << 'EOF'
server:
  port: 26305
logging:
  level: info
EOF

cat > "$TMPDIR/user6.yaml" << 'EOF'
EOF

python "$MERGE_SCRIPT" "$TMPDIR/template6.yaml" "$TMPDIR/user6.yaml" "$TMPDIR/merged6.yaml" 2>&1
RESULT=$(cat "$TMPDIR/merged6.yaml")
assert_contains "模板作为完整输出" "$RESULT" "port: 26305"
assert_contains "模板作为完整输出" "$RESULT" "level: info"

echo ""
echo -e "${YELLOW}Summary: PASS=$PASS FAIL=$FAIL${NC}"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
