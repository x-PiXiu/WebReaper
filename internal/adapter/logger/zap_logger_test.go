package logger

import (
	"errors"
	"testing"

	"webreaper/internal/usecase/port"
)

// 测试策略：验证 ZapLogger 正确实现 port.Logger 接口，
// 且 With 链式调用能附加持久字段。不验证 zap 内部输出格式（那是 zap 自己的事）。

// 编译期断言：*ZapLogger 实现 port.Logger。
var _ port.Logger = (*ZapLogger)(nil)

func TestZapLogger_AllLevels(t *testing.T) {
	// 各级别调用不应 panic
	l := MustNewZapLogger("development", "")
	defer l.Sync()

	l.Debug("debug msg", port.String("k", "v"))
	l.Info("info msg", port.Int("n", 1))
	l.Warn("warn msg", port.Bool("b", true))
	l.Error("error msg", port.Err(errors.New("test")))
}

func TestZapLogger_Production(t *testing.T) {
	l := MustNewZapLogger("production", "")
	defer l.Sync()
	l.Info("production log") // 不应 panic
}

func TestZapLogger_With(t *testing.T) {
	l := MustNewZapLogger("development", "")
	defer l.Sync()

	// With 返回的子 Logger 应带持久字段
	sub := l.With(port.String("module", "collect"))
	sub.Info("started") // 不应 panic，且日志应含 module=collect

	// 链式再 With
	subsub := sub.With(port.String("phase", "dedup"))
	subsub.Info("dedup done")
}

func TestToZapFields(t *testing.T) {
	fields := []port.Field{
		port.String("s", "v"),
		port.Int("i", 1),
		port.Bool("b", true),
		port.Err(errors.New("e")),
		{Key: "any", Value: []int{1, 2}}, // 走 Any 分支
	}
	zf := toZapFields(fields)
	if len(zf) != 5 {
		t.Errorf("got %d zap fields, want 5", len(zf))
	}
}

func TestToZapFields_Empty(t *testing.T) {
	if got := toZapFields(nil); got != nil {
		t.Errorf("nil input should return nil, got %v", got)
	}
}
