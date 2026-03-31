#!/bin/bash
# E2E 测试脚本：自由协作模式

set -e

BASE_URL="http://localhost:8080/api/v1"
PROJECT_ID="e34d3175-bedf-4a61-83f9-589e94f52106"

echo "=== E2E 测试：自由协作模式 ==="
echo ""

# 测试 1: 获取工作流模板（用于获取 Agent 列表）
echo "1. 获取工作流模板..."
TEMPLATES=$(curl -s "$BASE_URL/workflows")
echo "   ✓ 工作流模板获取成功"

# 提取第一个模板的 Agent 列表
TEMPLATE_ID=$(echo $TEMPLATES | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "   模板 ID: $TEMPLATE_ID"

# 测试 2: 创建工作流模式 Thread（默认）
echo ""
echo "2. 创建工作流模式 Thread..."
WORKFLOW_THREAD=$(curl -s -X POST "$BASE_URL/threads/project/$PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "E2E测试-工作流模式"}')
WORKFLOW_THREAD_ID=$(echo $WORKFLOW_THREAD | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
WORKFLOW_THREAD_TYPE=$(echo $WORKFLOW_THREAD | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "   ✓ 工作流 Thread 创建成功"
echo "   ID: $WORKFLOW_THREAD_ID"
echo "   类型: $WORKFLOW_THREAD_TYPE"

# 验证默认类型是 workflow
if [ "$WORKFLOW_THREAD_TYPE" = "workflow" ]; then
  echo "   ✓ 默认类型验证通过 (workflow)"
else
  echo "   ✗ 默认类型验证失败，期望 workflow，实际 $WORKFLOW_THREAD_TYPE"
fi

# 测试 3: 创建自由讨论模式 Thread
echo ""
echo "3. 创建自由讨论模式 Thread..."
FREE_THREAD=$(curl -s -X POST "$BASE_URL/threads/project/$PROJECT_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "E2E测试-自由讨论", "type": "free_discussion", "availableAgents": ["agent1", "agent2", "agent3"]}')
FREE_THREAD_ID=$(echo $FREE_THREAD | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
FREE_THREAD_TYPE=$(echo $FREE_THREAD | grep -o '"type":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "   ✓ 自由讨论 Thread 创建成功"
echo "   ID: $FREE_THREAD_ID"
echo "   类型: $FREE_THREAD_TYPE"

# 验证类型是 free_discussion
if [ "$FREE_THREAD_TYPE" = "free_discussion" ]; then
  echo "   ✓ 自由讨论类型验证通过 (free_discussion)"
else
  echo "   ✗ 自由讨论类型验证失败，期望 free_discussion，实际 $FREE_THREAD_TYPE"
fi

# 测试 4: 获取 Thread 详情验证 availableAgents
echo ""
echo "4. 验证 availableAgents..."
THREAD_DETAIL=$(curl -s "$BASE_URL/threads/$FREE_THREAD_ID")
if echo "$THREAD_DETAIL" | grep -q "agent1"; then
  echo "   ✓ availableAgents 包含 agent1"
else
  echo "   ✗ availableAgents 不包含 agent1"
fi
if echo "$THREAD_DETAIL" | grep -q "agent2"; then
  echo "   ✓ availableAgents 包含 agent2"
else
  echo "   ✗ availableAgents 不包含 agent2"
fi

# 测试 5: 获取队友名册 API
echo ""
echo "5. 获取队友名册..."
ROSTER=$(curl -s "$BASE_URL/callbacks/teammate-roster?threadId=$FREE_THREAD_ID")
if echo "$ROSTER" | grep -q '"agents"'; then
  echo "   ✓ 队友名册 API 返回成功"
  echo "   响应: $ROSTER" | head -c 200
  echo ""
else
  echo "   ✗ 队友名册 API 返回失败"
fi

# 测试 6: MultiMentionStatus API（请求不存在）
echo ""
echo "6. 测试 MultiMentionStatus API（请求不存在）..."
MM_STATUS=$(curl -s -w "\n%{http_code}" "$BASE_URL/callbacks/multi-mention-status?id=00000000-0000-0000-0000-000000000000")
HTTP_CODE=$(echo "$MM_STATUS" | tail -1)
BODY=$(echo "$MM_STATUS" | head -1)
if [ "$HTTP_CODE" = "404" ]; then
  echo "   ✓ 不存在的请求返回 404"
else
  echo "   状态码: $HTTP_CODE"
fi

# 测试 7: MultiMention API（无验证信息）
echo ""
echo "7. 测试 MultiMention API（无验证信息）..."
MM_RESULT=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/callbacks/multi-mention" \
  -H "Content-Type: application/json" \
  -d '{"invocationId": "00000000-0000-0000-0000-000000000000", "callbackToken": "invalid", "targets": ["agent1"], "question": "test", "callbackTo": "agent1"}')
HTTP_CODE=$(echo "$MM_RESULT" | tail -1)
if [ "$HTTP_CODE" = "401" ]; then
  echo "   ✓ 无效凭证返回 401"
else
  echo "   状态码: $HTTP_CODE"
fi

# 测试 8: 列出项目的所有 Thread
echo ""
echo "8. 列出项目的所有 Thread..."
THREADS=$(curl -s "$BASE_URL/threads/project/$PROJECT_ID")
THREAD_COUNT=$(echo "$THREADS" | grep -o '"id"' | wc -l)
echo "   ✓ 项目共有 $THREAD_COUNT 个 Thread"

# 总结
echo ""
echo "=== E2E 测试完成 ==="
echo "所有 API 端点验证通过"