// Package telemetry 提供 OpenTelemetry 可观测性实现（框架与驱动层）。
//
// 整洁架构定位：本层把 OpenTelemetry 的初始化细节封装起来，
// 并通过实现 port.Tracer 接口为用例层提供 trace 能力。
// 用例层依赖 port.Tracer（不直接 import otel），依赖方向合法：
// telemetry(适配器) → otel / port（向内），usecase → port（向内）。
//
// exporter 支持：
//   - stdout：开发模式，trace 直接打印到控制台（不依赖外部 Collector）
//   - otlp：生产模式，通过 OTLP/HTTP 发往 Collector（如 Jaeger/Tempo）
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"webreaper/internal/usecase/port"
)

// ExporterKind 选择 trace 输出方式。
type ExporterKind string

const (
	ExporterStdout ExporterKind = "stdout" // 控制台输出（开发）
	ExporterOTLP   ExporterKind = "otlp"   // OTLP/HTTP 发往 Collector（生产）
)

// Config 是 telemetry 初始化配置。
type Config struct {
	Enabled       bool         // 是否启用 trace
	Exporter      ExporterKind // exporter 类型（stdout / otlp）
	OTLPEndpoint  string       // OTLP/HTTP 端点，如 localhost:4318（仅 otlp 生效）
	ServiceName   string       // 服务名（resource 属性）
}

// InitTracer 初始化全局 TracerProvider。
//
// 返回 (shutdown 函数, port.Tracer 实现, error)。
// enabled=false 时初始化 no-op tracer（不采集 trace）。
// shutdown 函数在进程退出前调用以刷新缓冲。
//
// 兼容旧调用：保留二返回值形态的 InitTracer 已弃用，新代码用 Init。
func InitTracer(serviceName string, enabled bool) (func(context.Context) error, error) {
	shutdown, _, err := Init(Config{Enabled: enabled, Exporter: ExporterStdout, ServiceName: serviceName})
	return shutdown, err
}

// Init 初始化 tracer 并返回 port.Tracer 实现。
//
// 这是推荐的入口：把 OTel 细节封装在本包，对外只暴露 port.Tracer。
// 调用方（cmd/server）把返回的 Tracer 注入到各 usecase。
func Init(cfg Config) (shutdown func(context.Context) error, tracer port.Tracer, err error) {
	if !cfg.Enabled {
		// 不启用：返回 no-op tracer 和空 shutdown
		return func(context.Context) error { return nil }, port.NewNopTracer(), nil
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "webreaper"
	}

	exporter, err := buildExporter(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		// schema 冲突时降级：只用自定义属性，不合并 default resource
		res = resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, &otelTracer{tp: tp}, nil
}

// buildExporter 按 Config.Exporter 构造对应的 span exporter。
func buildExporter(cfg Config) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case ExporterOTLP, ExporterKind(""):
		// 显式 otlp，或未指定但有 endpoint 时走 otlp
		if cfg.Exporter == ExporterOTLP && cfg.OTLPEndpoint == "" {
			return nil, errors.New("OTLP exporter 需要配置 OTLPEndpoint")
		}
		if cfg.OTLPEndpoint == "" {
			// 无 endpoint 退回 stdout，避免初始化失败
			return stdoutExporter()
		}
		host, port := parseOTLPEndpoint(cfg.OTLPEndpoint)
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(host + ":" + port)}
		// 本地开发常无 TLS
		if isLocalhost(host) {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(context.Background(), opts...)
	case ExporterStdout:
		return stdoutExporter()
	default:
		return nil, fmt.Errorf("unknown exporter kind: %q", cfg.Exporter)
	}
}

func stdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New(stdouttrace.WithWriter(os.Stdout), stdouttrace.WithPrettyPrint())
}

// parseOTLPEndpoint 把 "localhost:4318" 拆成 host/port，缺省 port 给 4318。
func parseOTLPEndpoint(endpoint string) (host, port string) {
	host = endpoint
	port = "4318"
	if i := strings.LastIndex(endpoint, ":"); i >= 0 {
		host = endpoint[:i]
		port = endpoint[i+1:]
	}
	return
}

func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// otelTracer 实现 port.Tracer，内部委托给全局 TracerProvider。
type otelTracer struct {
	tp *sdktrace.TracerProvider
}

func (t *otelTracer) StartSpan(ctx context.Context, name string) (context.Context, port.Span) {
	ctx, span := otel.Tracer("webreaper").Start(ctx, name)
	return ctx, &otelSpan{span: span}
}

// otelSpan 实现 port.Span，包装 otel trace.Span。
type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) End() { s.span.End() }

func (s *otelSpan) SetAttribute(key string, value any) {
	s.span.SetAttributes(toOTelAttr(key, value))
}

func (s *otelSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

// StartSpan 为适配器层（已在 telemetry 包内或可 import telemetry 的包）创建子 span。
// 用法：ctx, span := telemetry.StartSpan(ctx, "ai.run_task"); defer span.End()
//
// 注意：此便捷函数仅供适配器层使用。用例层应通过注入的 port.Tracer 调用，
// 避免用例直接 import telemetry（保持依赖方向：adapter → usecase）。
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer("webreaper").Start(ctx, name)
}

// 编译期断言：otelTracer 实现 port.Tracer。
var _ port.Tracer = (*otelTracer)(nil)

// toOTelAttr 把 (key, any) 转为 OTel 的强类型 attribute.KeyValue。
// 按 value 的实际类型分派；未覆盖的类型退化为字符串（fmt.Sprint），
// 保证用例层传任意值都不会 panic。
func toOTelAttr(key string, value any) attribute.KeyValue {
	k := attribute.Key(key)
	switch v := value.(type) {
	case string:
		return k.String(v)
	case int:
		return k.Int(v)
	case int64:
		return k.Int64(v)
	case float64:
		return k.Float64(v)
	case bool:
		return k.Bool(v)
	case error:
		return k.String(v.Error())
	default:
		return k.String(fmt.Sprint(v))
	}
}
