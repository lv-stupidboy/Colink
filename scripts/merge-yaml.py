#!/usr/bin/env python
"""YAML 配置合并工具

实现 3 层配置合并（与 Electron installer 的 mergeObjects 逻辑一致）：
1. 模板定义结构（哪些键存在）
2. 用户已有值覆盖模板默认值
3. 递归处理嵌套对象，数组整体替换
4. 模板中新增的键自动添加
5. 用户配置中模板已删除的键自动移除

用法:
    python merge-yaml.py <template.yaml> <user.yaml> [output.yaml]

    不指定 output.yaml 时输出到 stdout
"""

import sys
import yaml


def merge_objects(template, user):
    """以模板为基准合并用户配置

    - 模板定义哪些键存在
    - 用户值覆盖模板值
    - 嵌套 dict 递归合并
    - 数组整体替换
    - 模板中没有的键从用户配置中移除
    """
    if not isinstance(template, dict):
        return user
    if not isinstance(user, dict):
        return template

    result = {}
    for key, template_value in template.items():
        if key in user:
            user_value = user[key]
            if isinstance(template_value, dict) and isinstance(user_value, dict):
                result[key] = merge_objects(template_value, user_value)
            else:
                result[key] = user_value
        else:
            result[key] = template_value
    return result


def main():
    if len(sys.argv) < 3:
        print(f"用法: {sys.argv[0]} <template.yaml> <user.yaml> [output.yaml]", file=sys.stderr)
        sys.exit(1)

    template_path = sys.argv[1]
    user_path = sys.argv[2]
    output_path = sys.argv[3] if len(sys.argv) > 3 else None

    with open(template_path, 'r', encoding='utf-8') as f:
        template = yaml.safe_load(f) or {}
    with open(user_path, 'r', encoding='utf-8') as f:
        user = yaml.safe_load(f) or {}

    merged = merge_objects(template, user)

    result_yaml = yaml.dump(merged, default_flow_style=False, allow_unicode=True, sort_keys=False)

    if output_path:
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(result_yaml)
    else:
        print(result_yaml, end='')


if __name__ == '__main__':
    main()
