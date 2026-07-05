// Package handler 提供 HTTP Controller（接口适配器层）。
//
// 整洁架构定位：本层是"谦卑对象"——只做 HTTP 协议与用例之间的数据搬运，
// 不含任何业务逻辑。每个 handler 持有一个用例引用，把 HTTP 请求翻译成
// 用例的 Input Model，调用 Execute，再把 Output Model 翻译成 HTTP 响应。
//
// 依赖方向：handler → usecase（向内）。Gin 只出现在本目录和 cmd/。
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"webreaper/internal/pkg"
)

// envelope 是统一响应信封。
// 前端按 code 判断业务结果，data 承载实际数据，msg 用于错误提示。
type envelope struct {
	Code int    `json:"code"` // 0=成功，非 0=业务错误码
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// success 返回成功响应（HTTP 200）。
func success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Code: 0, Msg: "ok", Data: data})
}

// fail 把业务错误映射为 HTTP 状态码并返回。
// 映射规则见 statusForError。
func fail(c *gin.Context, err error) {
	status, code := statusForError(err)
	c.JSON(status, envelope{Code: code, Msg: err.Error()})
}

// statusForError 把 pkg 中的业务错误映射到 HTTP 状态码 + 业务码。
// 未识别的错误统一归为 500。
//
// 注意：Gin 的 binding 校验失败（字段缺失/格式错）返回 validator 错误，
// 这类错误也归为 400，与业务参数错误 ErrInvalidArgument 同等对待。
func statusForError(err error) (httpStatus int, bizCode int) {
	switch {
	case errors.Is(err, pkg.ErrInvalidArgument):
		return http.StatusBadRequest, 40001
	case errors.Is(err, pkg.ErrNotFound):
		return http.StatusNotFound, 40401
	case errors.Is(err, pkg.ErrAlreadyExists), errors.Is(err, pkg.ErrAlreadyPublished):
		return http.StatusConflict, 40901
	case errors.Is(err, pkg.ErrSpiderNotRegistered), errors.Is(err, pkg.ErrPlatformNotRegistered):
		return http.StatusUnprocessableEntity, 42201
	case errors.Is(err, pkg.ErrTaskNotExecutable):
		return http.StatusConflict, 40902
	default:
		// Gin binding 错误（json.UnmarshalTypeError、validator.ValidationErrors 等）
		// 属于客户端请求格式问题，归 400。
		if isBindingError(err) {
			return http.StatusBadRequest, 40002
		}
		return http.StatusInternalServerError, 50000
	}
}

// isBindingError 判断是否为请求绑定/校验类错误。
func isBindingError(err error) bool {
	if err == nil {
		return false
	}
	// Gin 的 binding 错误（validator 或 json 解码）特征匹配。
	msg := err.Error()
	markers := []string{"required", "validation", "binding", "json:", "cannot unmarshal", "Field validation"}
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}
