package errors

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorCode 错误码类型
type ErrorCode string

// AppError 应用错误结构
type AppError struct {
	Code    ErrorCode // 错误码（给前端）
	Message string    // 简化提示（给前端）
	Detail  string    // 技术细节（仅日志）
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	return e.Detail
}

// 预定义错误码
const (
	CodeNetworkTimeout  ErrorCode = "NETWORK_TIMEOUT"
	CodeAuthFailed      ErrorCode = "AUTH_FAILED"
	CodeRepoNotFound    ErrorCode = "REPO_NOT_FOUND"
	CodeBranchNotFound  ErrorCode = "BRANCH_NOT_FOUND"
	CodeParseFailed     ErrorCode = "PARSE_FAILED"
	CodePackageNotFound ErrorCode = "PACKAGE_NOT_FOUND"
	CodeInvalidParam    ErrorCode = "INVALID_PARAM"
	CodeInternal        ErrorCode = "INTERNAL_ERROR"
)

// 预定义错误（基础模板）
var (
	ErrNetworkTimeout   = &AppError{Code: CodeNetworkTimeout, Message: "网络连接超时"}
	ErrAuthFailed       = &AppError{Code: CodeAuthFailed, Message: "认证失败"}
	ErrRepoNotFound     = &AppError{Code: CodeRepoNotFound, Message: "仓库不存在"}
	ErrBranchNotFound   = &AppError{Code: CodeBranchNotFound, Message: "分支不存在"}
	ErrParseFailed      = &AppError{Code: CodeParseFailed, Message: "配置文件解析失败"}
	ErrPackageNotFound  = &AppError{Code: CodePackageNotFound, Message: "团队包不存在"}
	ErrInvalidParam     = &AppError{Code: CodeInvalidParam, Message: "参数错误"}
	ErrInternal         = &AppError{Code: CodeInternal, Message: "内部错误"}
)

// WithDetail 创建带详细信息的 AppError
func WithDetail(base *AppError, detail string) *AppError {
	return &AppError{
		Code:    base.Code,
		Message: base.Message,
		Detail:  detail,
	}
}

// WrapError 从原始错误包装为 AppError
func WrapError(err error) *AppError {
	if err == nil {
		return nil
	}

	// 如果已经是 AppError，直接返回
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	errMsg := err.Error()
	return WithDetail(ErrInternal, errMsg)
}

// WrapGitError 从 Git 命令输出分析并包装错误
func WrapGitError(output string, err error) *AppError {
	if err == nil {
		return nil
	}

	outputLower := strings.ToLower(output)

	// 网络超时
	if strings.Contains(outputLower, "timeout") ||
		strings.Contains(outputLower, "timed out") ||
		strings.Contains(output, "context deadline exceeded") {
		return WithDetail(ErrNetworkTimeout, fmt.Sprintf("%s: %s", err.Error(), output))
	}

	// 认证失败
	if strings.Contains(output, "Authentication failed") ||
		strings.Contains(output, "could not read Username") ||
		strings.Contains(output, "Permission denied") ||
		strings.Contains(outputLower, "fatal: could not read") {
		return WithDetail(ErrAuthFailed, fmt.Sprintf("%s: %s", err.Error(), output))
	}

	// 仓库不存在
	if strings.Contains(output, "Repository not found") ||
		strings.Contains(output, "remote: Repository not found") ||
		strings.Contains(output, "does not exist") {
		return WithDetail(ErrRepoNotFound, fmt.Sprintf("%s: %s", err.Error(), output))
	}

	// 分支不存在
	if strings.Contains(output, "Remote branch") && strings.Contains(output, "not found") ||
		strings.Contains(output, "could not find remote branch") {
		return WithDetail(ErrBranchNotFound, fmt.Sprintf("%s: %s", err.Error(), output))
	}

	// 其他错误归类为内部错误
	return WithDetail(ErrInternal, fmt.Sprintf("%s: %s", err.Error(), output))
}

// ToHTTPStatus 将错误码转换为 HTTP 状态码
func ToHTTPStatus(code ErrorCode) int {
	switch code {
	case CodeNetworkTimeout:
		return http.StatusServiceUnavailable // 503
	case CodeAuthFailed:
		return http.StatusUnauthorized // 401
	case CodeRepoNotFound, CodeBranchNotFound, CodePackageNotFound:
		return http.StatusNotFound // 404
	case CodeInvalidParam:
		return http.StatusBadRequest // 400
	default:
		return http.StatusInternalServerError // 500
	}
}

// NewInvalidParam 创建参数错误
func NewInvalidParam(detail string) *AppError {
	return WithDetail(ErrInvalidParam, detail)
}

// NewPackageNotFound 创建团队包不存在错误
func NewPackageNotFound(packageName string) *AppError {
	return WithDetail(ErrPackageNotFound, fmt.Sprintf("package not found: %s", packageName))
}

// NewParseFailed 创建解析失败错误
func NewParseFailed(filename string, err error) *AppError {
	return WithDetail(ErrParseFailed, fmt.Sprintf("parse %s failed: %s", filename, err.Error()))
}