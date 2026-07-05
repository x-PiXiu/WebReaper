// Package telemetry 提供 OpenTelemetry 可观测性实现（框架与驱动层）。
//
// 整洁架构定位：本层把 OpenTelemetry 的初始化细节封装起来。
// trace 是横切关注点，用例层通过已有的 port.Logger 风格使用（不直接依赖 OTel API）。
// 当前用 stdout exporter，不依赖外部 Collector，本地直接看 trace。
//
// 依赖方向：telemetry → otel（向内）。otel 只出现在本目录和 handler/middleware。
package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer 初始化全局 TracerProvider，使用 stdout exporter 输出到控制台。
//
// 返回 shutdown 函数，进程退出前调用以刷新缓冲。
// enabled=false 时初始化 no-op tracer（不采集 trace）。
func InitTracer(serviceName string, enabled bool) (func(context.Context) error, error) {
	if !enabled {
		// 不启用：返回 no-op provider 的 shutdown
		noopShutdown := func(context.Context) error { return nil }
		return noopShutdown, nil
	}

	exporter, err := stdouttrace.New(stdouttrace.WithWriter(os.Stdout), stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, fmt.Errorf("create stdout exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		// schema 冲突时降级：只用自定义属性，不合并 default resource
		res = resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// StartSpan 为用例/适配器创建一个子 span。
// 用法：ctx, span := telemetry.StartSpan(ctx, "ai.generate_questions"); defer span.End()
// 如果未启用 trace，返回 no-op span（开销极低）。
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer("webreaper").Start(ctx, name)
}
