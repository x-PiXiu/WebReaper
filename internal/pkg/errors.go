package pkg

import "errors"

// 本文件定义业务级错误。
// 注意：这是通用错误类型，不依赖任何框架。仓储/用例/适配器都可使用。
// 后续如果需要 HTTP 状态码映射，由适配器层负责转换（依赖方向：适配器依赖用例，反之不行）。

var (
	// ErrNotFound 资源不存在。
	ErrNotFound = errors.New("resource not found")
	// ErrAlreadyExists 资源已存在（去重冲突）。
	ErrAlreadyExists = errors.New("resource already exists")
	// ErrInvalidArgument 输入参数不合法（违反领域规则）。
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrTaskNotExecutable 任务不可执行（状态机不允许）。
	ErrTaskNotExecutable = errors.New("task is not executable in current status")
	// ErrSpiderNotRegistered 找不到对应类型的爬虫实现。
	ErrSpiderNotRegistered = errors.New("spider not registered for this source type")
	// ErrPlatformNotRegistered 找不到对应名称的推送平台实现。
	ErrPlatformNotRegistered = errors.New("platform not registered")
	// ErrAlreadyPublished 该内容已成功推送到目标平台（去重拦截）。
	ErrAlreadyPublished = errors.New("content already published to this platform")
)
