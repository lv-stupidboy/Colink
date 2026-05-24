package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var ErrOutsideWorkspace = errors.New("path outside workspace")

type Guard struct {
	root string
}

func NewGuard(root string) (*Guard, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return &Guard{}, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析工作空间路径失败: %w", err)
	}
	return &Guard{root: filepath.Clean(abs)}, nil
}

func (g *Guard) Enabled() bool {
	return g != nil && g.root != ""
}

func (g *Guard) Root() string {
	if g == nil {
		return ""
	}
	return g.root
}

func (g *Guard) NormalizeStart(path string) string {
	if !g.Enabled() {
		return path
	}
	if strings.TrimSpace(path) == "" {
		return g.root
	}
	return path
}

func (g *Guard) Validate(path string) error {
	if !g.Enabled() {
		return nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("路径不能为空")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}
	if !isWithin(g.root, filepath.Clean(abs)) {
		return fmt.Errorf("路径必须位于工作空间内: %s", g.root)
	}
	return nil
}

func (g *Guard) ValidateChild(parentPath string, childName string) (string, error) {
	name := strings.TrimSpace(childName)
	if name == "" {
		return "", fmt.Errorf("目录名称不能为空")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("目录名称不能包含路径分隔符")
	}
	if err := g.Validate(parentPath); err != nil {
		return "", err
	}
	child := filepath.Join(parentPath, name)
	if err := g.Validate(child); err != nil {
		return "", err
	}
	return child, nil
}

func isWithin(root string, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
