// Package logger 提供 port.Logger 的 zap 实现（框架与驱动层）。
//
// 整洁架构定位：本层把日志库（zap）的细节封装起来，
// 用例层只依赖 port.Logger 接口，对本文件的具体实现一无所知。
//
// 依赖方向：logger → zap + port（向内）。
// zap 只出现在本目录，domain/usecase 对其一无所知。
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"webreaper/internal/usecase/port"
)

// ZapLogger 是 port.Logger 的 zap 实现。
type ZapLogger struct {
	zap *zap.Logger
}

// NewZapLogger 创建 zap Logger。
// env="development" 用 Console 编码（彩色、人类友好）；
// env="production" 用 JSON 编码（机器可解析、便于 ELK 采集）。
// level 显式覆盖（debug/info/warn/error；空=按环境默认：dev=debug、prod=info）。
func NewZapLogger(env, level string) (*ZapLogger, error) {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel) // 生产环境不打 Debug
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 彩色级别
	}
	if lv := parseZapLevel(level); lv != nil {
		cfg.Level = zap.NewAtomicLevelAt(*lv) // LOG_LEVEL 显式覆盖（本地排查用）
	}

	z, err := cfg.Build(zap.AddCallerSkip(1)) // 跳过本包装层，显示真实调用方
	if err != nil {
		return nil, err
	}
	return &ZapLogger{zap: z}, nil
}

// parseZapLevel 字符串 → zap 级别（未知/空返回 nil=不覆盖）。
func parseZapLevel(s string) *zapcore.Level {
	var lv zapcore.Level
	if err := lv.UnmarshalText([]byte(s)); err == nil && s != "" {
		return &lv
	}
	return nil
}

// MustNewZapLogger 创建 zap Logger，失败时 panic。
// 仅供 main 启动阶段使用（启动失败无法恢复）。
func MustNewZapLogger(env, level string) *ZapLogger {
	l, err := NewZapLogger(env, level)
	if err != nil {
		panic(err)
	}
	return l
}

func (l *ZapLogger) Debug(msg string, fields ...port.Field) {
	l.zap.Debug(msg, toZapFields(fields)...)
}

func (l *ZapLogger) Info(msg string, fields ...port.Field) {
	l.zap.Info(msg, toZapFields(fields)...)
}

func (l *ZapLogger) Warn(msg string, fields ...port.Field) {
	l.zap.Warn(msg, toZapFields(fields)...)
}

func (l *ZapLogger) Error(msg string, fields ...port.Field) {
	l.zap.Error(msg, toZapFields(fields)...)
}

func (l *ZapLogger) With(fields ...port.Field) port.Logger {
	return &ZapLogger{zap: l.zap.With(toZapFields(fields)...)}
}

func (l *ZapLogger) Sync() error {
	return l.zap.Sync()
}

// toZapFields 把 port.Field 转换为 zap.Field，隔离日志库类型。
func toZapFields(fields []port.Field) []zap.Field {
	if len(fields) == 0 {
		return nil
	}
	zf := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		switch v := f.Value.(type) {
		case error:
			zf = append(zf, zap.NamedError(f.Key, v))
		case string:
			zf = append(zf, zap.String(f.Key, v))
		case int:
			zf = append(zf, zap.Int(f.Key, v))
		case int64:
			zf = append(zf, zap.Int64(f.Key, v))
		case bool:
			zf = append(zf, zap.Bool(f.Key, v))
		default:
			zf = append(zf, zap.Any(f.Key, v))
		}
	}
	return zf
}
