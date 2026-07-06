package telemetry

import (
	"context"
	"errors"
	"testing"

	"webreaper/internal/usecase/port"
)

// TestInit_Disabled 验证 Enabled=false 时返回 NopTracer，不采集 trace。
func TestInit_Disabled(t *testing.T) {
	shutdown, tracer, err := Init(Config{Enabled: false, Exporter: ExporterStdout})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer shutdown(context.Background())

	if _, ok := tracer.(port.NopTracer); !ok {
		t.Errorf("Enabled=false 时应返回 NopTracer，得到 %T", tracer)
	}

	// NopTracer 的 StartSpan 不应 panic 且返回非 nil span
	_, span := tracer.StartSpan(context.Background(), "test")
	if span == nil {
		t.Error("span 不应为 nil")
	}
	span.End()
	span.SetAttribute("k", "v")
	span.RecordError(errors.New("x"))
}

// TestInit_Stdout 验证 stdout exporter 正常初始化（开发模式）。
func TestInit_Stdout(t *testing.T) {
	shutdown, tracer, err := Init(Config{Enabled: true, Exporter: ExporterStdout, ServiceName: "test-svc"})
	if err != nil {
		t.Fatalf("Init stdout: %v", err)
	}
	defer shutdown(context.Background())

	// 应返回 otelTracer（非 NopTracer）
	if _, ok := tracer.(port.NopTracer); ok {
		t.Error("Enabled=true + stdout 不应返回 NopTracer")
	}

	// 创建一个 span，验证不 panic
	_, span := tracer.StartSpan(context.Background(), "test-span")
	span.SetAttribute("key", "value")
	span.SetAttribute("num", 42)
	span.SetAttribute("flag", true)
	span.RecordError(errors.New("test error"))
	span.End()
}

// TestParseOTLPEndpoint 验证 endpoint 解析（含/不含端口）。
func TestParseOTLPEndpoint(t *testing.T) {
	cases := []struct {
		in       string
		wantPort string
	}{
		{"localhost:4318", "4318"},
		{"jaeger:4318", "4318"},
		{"127.0.0.1", "4318"}, // 无端口默认 4318
	}
	for _, c := range cases {
		_, port := parseOTLPEndpoint(c.in)
		if port != c.wantPort {
			t.Errorf("parseOTLPEndpoint(%q).port = %q, want %q", c.in, port, c.wantPort)
		}
	}
}

// TestToOTelAttr 验证 any→attribute 的类型分派不 panic。
func TestToOTelAttr(t *testing.T) {
	// 覆盖所有类型分支，确保不 panic
	values := []any{
		"str", 42, int64(64), 3.14, true, errors.New("e"),
		[]string{"x"}, // 未覆盖类型，退化为字符串
	}
	for _, v := range values {
		kv := toOTelAttr("k", v)
		if string(kv.Key) != "k" {
			t.Errorf("key 应为 k，得到 %q", kv.Key)
		}
	}
}

// TestNopTracer 通过 port.Tracer 接口验证 NopTracer。
func TestNopTracer(t *testing.T) {
	var tr port.Tracer = port.NewNopTracer()
	ctx, span := tr.StartSpan(context.Background(), "x")
	if ctx == nil {
		t.Error("ctx 不应为 nil")
	}
	span.End()
	span.SetAttribute("a", 1)
	span.RecordError(errors.New("b"))
}
