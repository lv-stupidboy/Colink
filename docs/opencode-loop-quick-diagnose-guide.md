# 快速启用诊断模式指南

## 问题：如何在问题发生时快速捕捉证据？

OpenCode 无限循环问题是偶发的，无法预测何时出现。如果问题发生时没有开启 debug 日志，事后无法捕捉关键证据。

## 解决方案 1: 默认收集关键诊断信息（已实现）

**现在已默认开启**，无需修改配置：

### 自动收集的数据（info 级别）

1. **通知进度摘要**（每 20 个通知）
   ```
   ACP: notification progress (count: 20, duplicateCount: 0)
   ```

2. **重复通知警告**（连续重复 >3 次）
   ```
   ACP: duplicate session/update detected (duplicateCount: 4, totalCount: 80)
   ```

3. **Cleanup 状态**（每次清理）
   ```
   ACP: cleanup called (notificationCount: 150, duplicateCount: 5, outputLen: 1024)
   ```

### 诊断字段（自动追踪）

```go
type acpSession struct {
    notificationCount    int    // 收到的通知总数
    duplicateUpdateCount int    // 连续重复通知计数
    lastUpdateHash       string // 最后一次 session/update 的内容哈希
}
```

**效果**：即使 info 级别，也能捕捉到：
- notificationCount 异常增长（>100 表示可能循环）
- duplicateUpdateCount >5 表示重复循环
- cleanup 是否调用、何时调用

## 解决方案 2: 问题发生时快速切换（可选）

如果问题正在发生，想获取更详细信息：

### 方法 A: 环境变量快速启用（推荐）

```bash
# 创建快速诊断启动脚本
export ISDP_LOG_LEVEL=debug
./scripts/start.sh
```

或修改 `scripts/start.sh`：

```bash
#!/bin/bash
# 检查是否需要诊断模式
if [ "$1" = "--diagnose" ]; then
    export ISDP_LOG_LEVEL=debug
    echo "启动诊断模式（debug 日志）"
fi

# 正常启动
./bin/isdp-server
```

使用：
```bash
# 正常启动
./scripts/start.sh

# 问题发生时快速重启为诊断模式
./scripts/stop.sh
./scripts/start.sh --diagnose
```

### 方法 B: 配置文件快速切换

创建两个配置文件：

```bash
# 正常配置
configs/config.yaml         # logging.level: info

# 诊断配置
configs/config-diagnose.yaml # logging.level: debug
```

问题发生时：
```bash
# 快速切换
cp configs/config-diagnose.yaml configs/config.yaml
./scripts/stop.sh
./scripts/start.sh
```

### 方法 C: 实时日志监控（无需重启）

问题发生时立即执行：

```bash
# 监控重复模式
tail -f data/logs/*.log | grep --line-buffered "duplicate session/update"

# 监控通知频率
tail -f data/logs/*.log | grep --line-buffered "notification progress" | awk '{print $NF}' | awk -F: '{print $2}'
```

## 解决方案 3: 持续后台监控（推荐）

### 启动后台监控脚本

创建 `scripts/monitor-opencode-loop.sh`：

```bash
#!/bin/bash
# 后台监控 OpenCode 循环问题
# 用法: ./scripts/monitor-opencode-loop.sh &

LOG_FILE="data/logs/monitor-opencode.log"

echo "启动 OpenCode 循环监控（后台）..." > $LOG_FILE

while true; do
    # 检查最近 100 行日志中的重复
    DUPLICATE_COUNT=$(tail -100 data/logs/*.log 2>/dev/null | grep -c "duplicate session/update")
    
    if [ "$DUPLICATE_COUNT" -gt 5 ]; then
        echo "[ALERT] $(date): 发现 $DUPLICATE_COUNT 次重复通知" >> $LOG_FILE
        echo "详细信息:" >> $LOG_FILE
        tail -50 data/logs/*.log | grep "duplicate session/update" >> $LOG_FILE
        
        # 可选：自动终止进程
        # if [ "$DUPLICATE_COUNT" -gt 10 ]; then
        #     kill -9 $(pgrep -f "opencode")
        #     echo "[AUTO-KILL] 进程已自动终止" >> $LOG_FILE
        # fi
    fi
    
    # 每分钟检查一次
    sleep 60
done
```

启动监控：
```bash
# 后台运行
nohup ./scripts/monitor-opencode-loop.sh > /dev/null 2>&1 &

# 查看监控日志
tail -f data/logs/monitor-opencode.log
```

停止监控：
```bash
pkill -f "monitor-opencode-loop.sh"
```

## 推荐使用流程

### 开发阶段（现在）

1. **默认已启用关键诊断**（info 级别）
   - 无需额外配置
   - 每次对话自动收集：notificationCount, duplicateUpdateCount
   - cleanup 时打印完整状态

2. **定期检查日志**
   ```bash
   # 每周检查是否有重复警告
   grep "duplicate session/update" data/logs/*.log
   ```

3. **发现问题时运行诊断**
   ```bash
   ./scripts/quick-diagnose.sh
   ```

### 生产阶段（可选）

1. **启用后台监控**
   ```bash
   nohup ./scripts/monitor-opencode-loop.sh &
   ```

2. **设置告警阈值**
   - duplicateCount >5 → Warning
   - duplicateCount >10 → Alert
   - notificationCount >200 → Error

3. **自动恢复（可选）**
   ```bash
   # 在监控脚本中添加自动终止
   if [ "$DUPLICATE_COUNT" -gt 15 ]; then
       kill -9 $(pgrep -f "opencode")
       echo "[AUTO-KILL] 检测到严重循环，进程已终止" >> $LOG_FILE
   fi
   ```

## 实际案例演示

### 场景 A: 问题正在发生

```bash
# 1. 运行快速诊断
./scripts/quick-diagnose.sh

# 输出：
# ⚠️ WARNING: Found 15 duplicate notifications
# ⚠️ WARNING: Process running with high notification rate (count: 300)
# ⚠️ WARNING: No cleanup called in recent logs

# 2. 查看实时日志
tail -f data/logs/*.log | grep "duplicate session/update"

# 输出：
# ACP: duplicate session/update detected (duplicateCount: 6, totalCount: 120)
# ACP: duplicate session/update detected (duplicateCount: 7, totalCount: 140)
# ACP: duplicate session/update detected (duplicateCount: 8, totalCount: 160)

# 3. 强制终止（如果无限循环）
kill -9 $(pgrep -f "opencode")

# 或 Windows:
taskkill /F /T /PID $(pgrep -f "opencode")

# 4. 收集证据
tail -500 data/logs/*.log > /tmp/opencode-loop-evidence.log
```

### 场景 B: 事后分析

```bash
# 1. 检查历史日志
grep "notification progress" data/logs/*.log | tail -20

# 输出：
# ACP: notification progress (count: 20, duplicateCount: 0)
# ACP: notification progress (count: 40, duplicateCount: 0)
# ACP: notification progress (count: 60, duplicateCount: 2)
# ACP: notification progress (count: 80, duplicateCount: 5) ← 开始重复
# ACP: notification progress (count: 100, duplicateCount: 8) ← 循环加剧

# 2. 查看 cleanup 状态
grep "cleanup called" data/logs/*.log

# 输出：
# ACP: cleanup called (notificationCount: 150, duplicateCount: 10, outputLen: 2048)
# ↑ cleanup 被调用，但 duplicateCount=10 表示已循环

# 3. 查看进程退出
grep "process exited" data/logs/*.log

# 输出：
# ACP: process exited normally
# ↑ 进程正常退出，但之前已循环 10 次

# 4. 分析根本原因
# 根据 duplicateCount 从 0→5→10 的增长趋势
# 判断为 OpenCode CLI 陷入循环
# 参考 docs/opencode-loop-diagnosis-checklist.md 定位修复方案
```

## 总结

### 无需额外配置即可捕捉证据

现在的代码已默认收集关键诊断信息（info 级别）：
- ✅ notificationCount（每 20 个通知）
- ✅ duplicateUpdateCount（>3 立即警告）
- ✅ cleanup 状态（完整指标）

### 问题发生时的应对流程

```bash
# 快速三步：
1. ./scripts/quick-diagnose.sh          # 快速诊断
2. tail -f data/logs/*.log | grep duplicate  # 实时监控
3. kill -9 $(pgrep -f "opencode")       # 强制终止（如需要）

# 事后分析：
4. grep "notification progress" data/logs/*.log  # 查看趋势
5. 参考 docs/opencode-loop-diagnosis-checklist.md  # 定位根因
```

### 长期监控（可选）

```bash
# 启动后台监控
nohup ./scripts/monitor-opencode-loop.sh &
```