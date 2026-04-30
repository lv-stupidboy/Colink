#!/usr/bin/env python3
"""
ISDP Test Report Summary Tool
按时间和特性整理测试报告，生成汇总视图

Usage:
    python scripts/test-report-summary.py                    # 显示最新报告
    python scripts/test-report-summary.py --latest           # 显示最新报告详情
    python scripts/test-report-summary.py --list             # 列出所有历史报告
    python scripts/test-report-summary.py --by-feature       # 按特性分组显示
    python scripts/test-report-summary.py --run-id YYYYMMDD-HHMMSS  # 查看指定报告
"""

import argparse
import json
import os
import sys
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Any

# 项目根目录
PROJECT_ROOT = Path(__file__).parent.parent
WEB_DIR = PROJECT_ROOT / "web"
PLAYWRIGHT_REPORT_DIR = WEB_DIR / "playwright-report"
TEST_RESULTS_DIR = WEB_DIR / "test-results"
FEATURE_MAP_FILE = PROJECT_ROOT / "auto-test" / "feature-map.yaml"

# Feature ID 到名称的映射
FEATURE_NAMES: Dict[str, str] = {
    "F001": "Agent 对话核心",
    "F002": "WebSocket 流式",
    "F003": "多 Agent 协作 (A2A)",
    "F004": "团队包管理",
    "F005": "线程管理",
    "F006": "工作流执行",
    "F007": "IM 集成",
    "F008": "消息渲染",
    "F009": "深色模式",
    "F010": "性能优化",
}


def parse_feature_map() -> Dict[str, Any]:
    """解析 feature-map.yaml 文件"""
    import yaml

    if not FEATURE_MAP_FILE.exists():
        return {}

    with open(FEATURE_MAP_FILE, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def get_all_run_ids() -> List[str]:
    """获取所有测试运行的 ID（时间戳格式）"""
    run_ids = []

    # 从 test-results 目录获取
    if TEST_RESULTS_DIR.exists():
        for item in TEST_RESULTS_DIR.iterdir():
            if item.is_dir() and item.name.match("YYYYMMDD-HHMMSS"):
                run_ids.append(item.name)

    # 从 playwright-report 目录获取
    if PLAYWRIGHT_REPORT_DIR.exists():
        for item in PLAYWRIGHT_REPORT_DIR.iterdir():
            if item.is_dir() and item.name.match("YYYYMMDD-HHMMSS"):
                if item.name not in run_ids:
                    run_ids.append(item.name)

    # 按时间倒序排列
    run_ids.sort(reverse=True)
    return run_ids


def parse_run_id(run_id: str) -> datetime:
    """将运行 ID 转换为 datetime"""
    try:
        date_part, time_part = run_id.split("-")
        year = int(date_part[:4])
        month = int(date_part[4:6])
        day = int(date_part[6:8])
        hour = int(time_part[:2])
        minute = int(time_part[2:4])
        second = int(time_part[4:6])
        return datetime(year, month, day, hour, minute, second)
    except (ValueError, IndexError):
        return datetime.min


def load_test_results(run_id: str) -> Optional[Dict]:
    """加载指定运行的测试结果 JSON"""
    json_file = TEST_RESULTS_DIR / run_id / "test-results.json"
    if json_file.exists():
        with open(json_file, "r", encoding="utf-8") as f:
            return json.load(f)
    return None


def extract_feature_id(test_name: str) -> Optional[str]:
    """从测试名称提取 Feature ID"""
    import re

    # 匹配 [F001], [F002] 等格式
    match = re.search(r"\[(F\d{3})\]", test_name)
    if match:
        return match.group(1)

    # 匹配 @feature F001 格式
    match = re.search(r"@feature\s+(F\d{3})", test_name)
    if match:
        return match.group(1)

    return None


def extract_priority(test_name: str) -> str:
    """从测试名称提取优先级"""
    import re

    # 匹配 [P0], [P1] 等格式
    match = re.search(r"\[(P\d)\]", test_name)
    if match:
        return match.group(1)

    # 匹配 D-P0, D-P1 格式
    match = re.search(r"D-(P\d)", test_name)
    if match:
        return match.group(1)

    return "P?"


def group_by_feature(results: Dict) -> Dict[str, List[Dict]]:
    """按 Feature ID 分组测试结果"""
    feature_map: Dict[str, List[Dict]] = {}

    if not results or "specs" not in results:
        return feature_map

    for spec in results.get("specs", []):
        tests = spec.get("tests", [])
        for test in tests:
            test_name = test.get("name", "")
            feature_id = extract_feature_id(test_name) or "F???"

            if feature_id not in feature_map:
                feature_map[feature_id] = []

            feature_map[feature_id].append({
                "name": test_name,
                "status": test.get("status", "unknown"),
                "duration": test.get("duration", 0),
                "priority": extract_priority(test_name),
                "error": test.get("error", None),
            })

    return feature_map


def format_duration(ms: int) -> str:
    """格式化持续时间"""
    if ms < 1000:
        return f"{ms}ms"
    seconds = ms / 1000
    if seconds < 60:
        return f"{seconds:.1f}s"
    minutes = int(seconds // 60)
    secs = seconds % 60
    return f"{minutes}m {secs:.0f}s"


def print_summary(run_id: str, results: Optional[Dict]):
    """打印测试运行摘要"""
    print(f"\n{'=' * 60}")
    print(f"  测试运行: {run_id}")
    print(f"  时间: {parse_run_id(run_id).strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"{'=' * 60}\n")

    if not results:
        print("  ⚠️  未找到测试结果文件")
        return

    # 统计总数
    total = 0
    passed = 0
    failed = 0
    skipped = 0

    for spec in results.get("specs", []):
        for test in spec.get("tests", []):
            total += 1
            status = test.get("status", "unknown")
            if status == "passed":
                passed += 1
            elif status == "failed":
                failed += 1
            elif status == "skipped":
                skipped += 1

    # 打印统计
    print(f"  总计: {total}  ✅ 通过: {passed}  ❌ 失败: {failed}  ⏭️  跳过: {skipped}")
    print()

    # 打印失败的测试
    if failed > 0:
        print(f"  {'─' * 50}")
        print("  ❌ 失败的测试:")
        print(f"  {'─' * 50}\n")

        for spec in results.get("specs", []):
            for test in spec.get("tests", []):
                if test.get("status") == "failed":
                    name = test.get("name", "unknown")
                    feature_id = extract_feature_id(name) or "F???"
                    feature_name = FEATURE_NAMES.get(feature_id, "未知特性")
                    print(f"    • [{feature_id}] {name}")
                    print(f"      特性: {feature_name}")
                    if test.get("error"):
                        error_lines = test.get("error", "").split("\n")[:3]
                        for line in error_lines:
                            print(f"      {line}")
                    print()


def print_by_feature(run_id: str, results: Optional[Dict]):
    """按特性分组打印测试结果"""
    print(f"\n{'=' * 60}")
    print(f"  测试运行: {run_id}")
    print(f"  时间: {parse_run_id(run_id).strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"{'=' * 60}\n")

    if not results:
        print("  ⚠️  未找到测试结果文件")
        return

    feature_groups = group_by_feature(results)

    # 按特性 ID 排序
    sorted_features = sorted(feature_groups.keys())

    for feature_id in sorted_features:
        tests = feature_groups[feature_id]
        feature_name = FEATURE_NAMES.get(feature_id, "未知特性")

        passed = sum(1 for t in tests if t["status"] == "passed")
        failed = sum(1 for t in tests if t["status"] == "failed")
        total = len(tests)

        status_icon = "✅" if failed == 0 else "❌"
        print(f"\n  {status_icon} [{feature_id}] {feature_name}")
        print(f"     通过: {passed}/{total}  失败: {failed}")
        print(f"  {'─' * 50}")

        # 按优先级排序
        priority_order = {"P0": 0, "P1": 1, "P2": 2, "P3": 3, "P?": 4}
        sorted_tests = sorted(tests, key=lambda t: priority_order.get(t["priority"], 4))

        for test in sorted_tests:
            icon = "✅" if test["status"] == "passed" else "❌"
            duration = format_duration(test.get("duration", 0))
            print(f"    {icon} [{test['priority']}] {test['name']} ({duration})")

    # 总结
    total_tests = sum(len(t) for t in feature_groups.values())
    total_passed = sum(1 for t in feature_groups.values() for tt in t if tt["status"] == "passed")
    total_failed = sum(1 for t in feature_groups.values() for tt in t if tt["status"] == "failed")

    print(f"\n{'=' * 60}")
    print(f"  总结: {total_tests} 测试  ✅ {total_passed}  ❌ {total_failed}")
    print(f"{'=' * 60}\n")


def list_all_reports():
    """列出所有历史报告"""
    run_ids = get_all_run_ids()

    if not run_ids:
        print("\n  ⚠️  未找到任何测试报告\n")
        return

    print(f"\n{'=' * 60}")
    print("  历史测试报告列表")
    print(f"{'=' * 60}\n")

    print(f"  {'序号':<6} {'运行 ID':<20} {'时间':<20} {'状态':<10}")
    print(f"  {'─' * 60}")

    for i, run_id in enumerate(run_ids, 1):
        dt = parse_run_id(run_id)
        time_str = dt.strftime("%Y-%m-%d %H:%M:%S")

        # 检查是否有失败
        results = load_test_results(run_id)
        if results:
            failed = sum(
                1
                for spec in results.get("specs", [])
                for test in spec.get("tests", [])
                if test.get("status") == "failed"
            )
            status = "✅ 全通过" if failed == 0 else f"❌ {failed} 失败"
        else:
            status = "⚠️ 无数据"

        print(f"  {i:<6} {run_id:<20} {time_str:<20} {status:<10}")

    print(f"\n  使用 --run-id {run_ids[0]} 查看最新报告详情\n")


def main():
    parser = argparse.ArgumentParser(description="ISDP 测试报告汇总工具")
    parser.add_argument("--latest", action="store_true", help="显示最新报告详情")
    parser.add_argument("--list", action="store_true", help="列出所有历史报告")
    parser.add_argument("--by-feature", action="store_true", help="按特性分组显示")
    parser.add_argument("--run-id", type=str, help="查看指定运行 ID 的报告")

    args = parser.parse_args()

    # 列出所有报告
    if args.list:
        list_all_reports()
        return

    # 获取运行 ID
    run_ids = get_all_run_ids()
    if not run_ids:
        print("\n  ⚠️  未找到任何测试报告\n")
        print("  请先运行测试: cd web && npm run test:e2e\n")
        return

    if args.run_id:
        run_id = args.run_id
    else:
        run_id = run_ids[0]  # 默认使用最新的

    # 加载结果
    results = load_test_results(run_id)

    # 按显示模式输出
    if args.by_feature:
        print_by_feature(run_id, results)
    elif args.latest:
        print_summary(run_id, results)
    else:
        # 默认显示简要摘要
        print_summary(run_id, results)


if __name__ == "__main__":
    main()