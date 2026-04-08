// Package config 提供配置管理功能
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// MergeConfig 合并用户配置与模板配置
// 策略：用户值优先 + 模板补充缺失字段 + 更新版本号
// 返回：是否进行了合并操作
func MergeConfig(userConfigPath, templateConfigPath string) (bool, error) {
	// 1. 检查用户配置文件是否存在
	userExists, err := fileExists(userConfigPath)
	if err != nil {
		return false, fmt.Errorf("检查用户配置文件失败: %w", err)
	}

	// 2. 如果用户配置不存在，直接复制模板
	if !userExists {
		templateContent, err := os.ReadFile(templateConfigPath)
		if err != nil {
			return false, fmt.Errorf("读取模板配置失败: %w", err)
		}
		// 确保目录存在
		dir := filepath.Dir(userConfigPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return false, fmt.Errorf("创建配置目录失败: %w", err)
		}
		if err := os.WriteFile(userConfigPath, templateContent, 0644); err != nil {
			return false, fmt.Errorf("写入用户配置失败: %w", err)
		}
		return true, nil
	}

	// 3. 读取模板配置
	templateContent, err := os.ReadFile(templateConfigPath)
	if err != nil {
		return false, fmt.Errorf("读取模板配置失败: %w", err)
	}

	// 4. 解析模板 YAML
	var templateMap map[string]interface{}
	if err := yaml.Unmarshal(templateContent, &templateMap); err != nil {
		return false, fmt.Errorf("解析模板配置失败: %w", err)
	}

	// 5. 读取用户配置
	userContent, err := os.ReadFile(userConfigPath)
	if err != nil {
		return false, fmt.Errorf("读取用户配置失败: %w", err)
	}

	// 6. 解析用户 YAML
	var userMap map[string]interface{}
	if err := yaml.Unmarshal(userContent, &userMap); err != nil {
		return false, fmt.Errorf("解析用户配置失败: %w", err)
	}

	// 7. 比较版本号
	templateVersion := getStringFromMap(templateMap, "version")
	userVersion := getStringFromMap(userMap, "version")

	// 版本相同，无需合并
	if userVersion == templateVersion {
		return false, nil
	}

	// 8. 合并配置：用户值优先，模板补充缺失字段
	mergedMap := mergeYamlMaps(userMap, templateMap)

	// 9. 更新版本号
	mergedMap["version"] = templateVersion

	// 10. 写回配置文件
	mergedContent, err := yaml.Marshal(mergedMap)
	if err != nil {
		return false, fmt.Errorf("序列化合并配置失败: %w", err)
	}

	// 保留文件头注释（如果模板有）
	headerComment := extractHeaderComment(templateContent)
	finalContent := append([]byte(headerComment), mergedContent...)

	if err := os.WriteFile(userConfigPath, finalContent, 0644); err != nil {
		return false, fmt.Errorf("写入合并配置失败: %w", err)
	}

	return true, nil
}

// mergeYamlMaps 合并两个 YAML map，用户值优先
func mergeYamlMaps(user, template map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制模板所有字段
	for k, v := range template {
		result[k] = deepCopyValue(v)
	}

	// 用户值覆盖（递归合并嵌套 map）
	for k, v := range user {
		if isMap(v) && isMap(result[k]) {
			// 递归合并嵌套 map
			result[k] = mergeYamlMaps(
				v.(map[string]interface{}),
				result[k].(map[string]interface{}),
			)
		} else {
			// 用户值直接覆盖
			result[k] = deepCopyValue(v)
		}
	}

	return result
}

// deepCopyValue 深拷贝一个值
func deepCopyValue(v interface{}) interface{} {
	if isMap(v) {
		m := v.(map[string]interface{})
		result := make(map[string]interface{})
		for k, val := range m {
			result[k] = deepCopyValue(val)
		}
		return result
	}
	// 其他类型直接返回（YAML 基本类型是值类型）
	return v
}

// isMap 检查值是否为 map[string]interface{}
func isMap(v interface{}) bool {
	if v == nil {
		return false
	}
	_, ok := v.(map[string]interface{})
	return ok
}

// getStringFromMap 从 map 中获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// fileExists 检查文件是否存在
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// extractHeaderComment 提取 YAML 文件头部的注释
func extractHeaderComment(content []byte) string {
	lines := splitLines(string(content))
	headerLines := []string{}

	for _, line := range lines {
		// 头部注释以 # 开头，遇到非注释行停止
		if len(line) > 0 && line[0] == '#' {
			headerLines = append(headerLines, line)
		} else if len(line) == 0 && len(headerLines) > 0 {
			// 空行在注释后面也算头部
			headerLines = append(headerLines, line)
		} else if len(headerLines) > 0 {
			// 已经有注释了，遇到非注释非空行停止
			break
		}
	}

	if len(headerLines) == 0 {
		return ""
	}

	result := ""
	for _, line := range headerLines {
		result += line + "\n"
	}
	return result
}

// splitLines 分割字符串为行
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}