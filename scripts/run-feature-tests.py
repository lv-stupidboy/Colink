#!/usr/bin/env python3
"""
特性测试运行脚本
根据 feature-map.yaml 执行特性相关的所有测试
"""

import yaml
import subprocess
import argparse
from pathlib import Path

def load_feature_map():
    config_path = Path("auto-test/feature-map.yaml")
    if not config_path.exists():
        raise FileNotFoundError(f"特性映射文件不存在: {config_path}")

    with open(config_path, 'r', encoding='utf-8') as f:
        return yaml.safe_load(f)

def run_e2e_tests(test_patterns):
    cmd = ["npx", "playwright", "test", "auto-test/e2e/"]
    for pattern in test_patterns:
        cmd.extend(["--grep", pattern])
    print(f"执行 E2E 测试: {cmd}")
    subprocess.run(cmd, cwd="web", check=False)

def run_internal_tests(test_patterns):
    cmd = ["go", "test", "./auto-test/internal/...", "-v"]
    for pattern in test_patterns:
        cmd.extend(["-run", pattern])
    print(f"执行 Internal 测试: {cmd}")
    subprocess.run(cmd, check=False)

def run_feature_tests(feature_id):
    feature_map = load_feature_map()

    if feature_id not in feature_map['features']:
        print(f"特性 ID 不存在: {feature_id}")
        return

    feature = feature_map['features'][feature_id]
    print(f"\n{'='*60}")
    print(f"特性: {feature['name']} ({feature_id})")
    print(f"优先级: {feature['priority']}")
    print(f"{'='*60}\n")

    tests = feature['tests']

    if 'e2e' in tests:
        print("\n>>> E2E 测试 <<<")
        run_e2e_tests(tests['e2e'])

    if 'internal' in tests:
        print("\n>>> Internal 测试 <<<")
        run_internal_tests(tests['internal'])

def run_priority_tests(priority):
    feature_map = load_feature_map()

    matching_features = []
    for fid, fdata in feature_map['features'].items():
        if fdata['priority'] in priority.split(','):
            matching_features.append(fid)

    if not matching_features:
        print(f"没有找到优先级为 {priority} 的特性")
        return

    print(f"\n执行优先级 {priority} 的特性测试")
    print(f"特性列表: {matching_features}\n")

    for fid in matching_features:
        run_feature_tests(fid)

def main():
    parser = argparse.ArgumentParser(description='特性测试运行脚本')
    parser.add_argument('--feature', '-f', help='特性 ID (如 F001)')
    parser.add_argument('--priority', '-p', help='优先级 (如 P0 或 P0,P1)')

    args = parser.parse_args()

    if args.feature:
        run_feature_tests(args.feature)
    elif args.priority:
        run_priority_tests(args.priority)
    else:
        parser.print_help()

if __name__ == '__main__':
    main()