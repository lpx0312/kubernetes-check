package errors

import "fmt"

// ErrorCode 错误代码类型
type ErrorCode int

const (
    // K8S 客户端错误 1000-1099
    ErrK8SClient     ErrorCode = 1001
    ErrMetricsClient ErrorCode = 1002
    ErrConfigLoad    ErrorCode = 1003

    // 业务逻辑错误 2000-2099
    ErrNodeNotFound ErrorCode = 2001
    ErrPodAnalysis  ErrorCode = 2002
)

// AppError 应用错误
type AppError struct {
    Code    ErrorCode
    Op      string // 操作名称
    Err     error  // 原始错误
    Message string // 用户友好消息
}

// Error 实现 error 接口
func (e *AppError) Error() string {
    return fmt.Sprintf("[%d] %s: %s", e.Code, e.Op, e.Message)
}

// Unwrap 实现 errors.Unwrap 接口
func (e *AppError) Unwrap() error {
    return e.Err
}

// NewAppError 创建新的应用错误
func NewAppError(code ErrorCode, op string, err error, msg string) *AppError {
    return &AppError{
        Code:    code,
        Op:      op,
        Err:     err,
        Message: msg,
    }
}

// Wrap 包装错误,自动生成消息
func Wrap(err error, op string) *AppError {
    return &AppError{
        Code: ErrConfigLoad, // 默认错误码,调用方可覆盖
        Op:   op,
        Err:  err,
        Message: err.Error(),
    }
}
