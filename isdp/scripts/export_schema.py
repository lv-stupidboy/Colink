#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
导出 MySQL 数据库表结构
"""

import subprocess
import re
import os

# 数据库连接信息
HOST = 'rm-bp1u503844l66n8g7no.mysql.rds.aliyuncs.com'
PORT = 3306
USER = 'isdp'
PASSWORD = 'dthxzsy@2026'
DATABASE = 'dev_ji'
OUTPUT_FILE = 'sql-change/schema_dump_20260327.sql'

def run_mysqlsh(query):
    """执行 mysqlsh 命令"""
    env = os.environ.copy()
    env['PYTHONIOENCODING'] = 'utf-8'
    cmd = [
        'mysqlsh', '--sql',
        '-h', HOST, '-P', str(PORT),
        '-u', USER, '-p' + PASSWORD,
        '-D', DATABASE,
        '-e', query
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, encoding='utf-8', errors='ignore', env=env)
    return result.stdout or ''

def main():
    # 获取所有表
    stdout = run_mysqlsh(
        f"SELECT table_name FROM information_schema.tables WHERE table_schema = '{DATABASE}' ORDER BY table_name;"
    )

    lines_raw = stdout.strip().split('\n')
    tables = []
    for t in lines_raw:
        t = t.strip()
        if t and not t.startswith('TABLE_NAME') and not t.startswith('-'):
            tables.append(t)

    print(f"找到 {len(tables)} 个表")

    # 构建输出
    output_lines = []
    output_lines.append('-- ============================================')
    output_lines.append('-- ISDP 数据库表结构导出')
    output_lines.append('-- 导出时间: 2026-03-27')
    output_lines.append(f'-- 数据库: {DATABASE}')
    output_lines.append(f'-- 表数量: {len(tables)}')
    output_lines.append('-- 说明: 仅包含表结构，不包含数据')
    output_lines.append('-- ============================================')
    output_lines.append('')
    output_lines.append('SET NAMES utf8mb4;')
    output_lines.append('SET FOREIGN_KEY_CHECKS = 0;')
    output_lines.append('')

    for table in tables:
        table = table.strip()
        if not table:
            continue
        print(f"导出表: {table}")
        output_lines.append(f'-- ----------------------------------------')
        output_lines.append(f'-- Table: {table}')
        output_lines.append(f'-- ----------------------------------------')
        output_lines.append(f'DROP TABLE IF EXISTS `{table}`;')

        # 获取建表语句
        stdout = run_mysqlsh(f"SHOW CREATE TABLE `{table}`;")

        # 解析输出 - 找到 CREATE TABLE 语句
        if stdout:
            found_create = False
            for line in stdout.split('\n'):
                if 'CREATE TABLE' in line:
                    found_create = True
                if found_create:
                    output_lines.append(line.rstrip())
        else:
            print(f"  警告: 无输出")

        output_lines.append('')
        output_lines.append('')

    output_lines.append('SET FOREIGN_KEY_CHECKS = 1;')
    output_lines.append('')

    # 写入文件
    with open(OUTPUT_FILE, 'w', encoding='utf-8') as f:
        f.write('\n'.join(output_lines))

    print(f"\n导出完成: {OUTPUT_FILE}")

if __name__ == '__main__':
    main()