package port

// Field 是结构化日志的字段（key-value 对）。
// 刻意不依赖任何具体日志库（如 zap.Field），保持 port 层纯净。
// 适配器层负责把 Field 转换为具体日志库的类型。
type Field struct {
	Key   string
	Value any
}

// String / Int / Float64 / Bool / Err 是常用类型的构造便利函数。
func String(key, val string) Field       { return Field{Key: key, Value: val} }
func Int(key string, val int) Field      { return Field{Key: key, Value: val} }
func Float64(key string, val float64) Field { return Field{Key: key, Value: val} }
func Bool(key string, val bool) Field    { return Field{Key: key, Value: val} }
func Err(err error) Field                { return Field{Key: "error", Value: err} }

// Logger 是日志抽象接口（横切关注点的边界）。
//
// 依赖倒置：接口由【用例层】声明并拥有，由【适配器层】实现（如 zap）。
// 用例/适配器通过依赖注入获得 Logger，不再散用 fmt.Printf。
// 这样更换日志库时，用例层零修改。
//
// 设计要点：
//   - With 返回带上下文字段的子 Logger（链式调用）
//   - 方法签名不暴露任何具体日志库类型
type Logger interface {
	// Debug 调试级别日志（开发环境输出，生产环境丢弃）。
	Debug(msg string, fields ...Field)
	// Info 信息级别日志。
	Info(msg string, fields ...Field)
	// Warn 警告级别日志。
	Warn(msg string, fields ...Field)
	// Error 错误级别日志（不 panic）。
	Error(msg string, fields ...Field)

	// With 返回附加了持久字段的子 Logger。
	// 例如 logger.With("module", "collect").Info("started") 后续日志都带 module=collect。
	With(fields ...Field) Logger

	// Sync 刷新缓冲（进程退出前调用）。
	Sync() error
}

// NopLogger 是空操作的 Logger，用于不需要日志的场景（如部分测试）。
type NopLogger struct{}

func (NopLogger) Debug(string, ...Field) {}
func (NopLogger) Info(string, ...Field)  {}
func (NopLogger) Warn(string, ...Field)  {}
func (NopLogger) Error(string, ...Field) {}
func (n NopLogger) With(...Field) Logger { return n }
func (NopLogger) Sync() error            { return nil }
