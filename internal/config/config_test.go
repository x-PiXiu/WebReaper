package config

import (
	"os"
	"strings"
	"testing"
)

// 测试策略：
//   - DSN() / IsConfigured() 是纯函数，直接断言
//   - Load() 涉及环境变量，通过 t.Setenv 设置后验证（自动还原，不污染其他测试）

func TestDBConfig_DSN(t *testing.T) {
	c := DBConfig{
		User: "root", Password: "pass", Host: "localhost", Port: "3306", Name: "agentcore",
	}
	dsn := c.DSN()
	// DSN 格式：user:pass@tcp(host:port)/name?...
	if !strings.Contains(dsn, "root:pass@tcp(localhost:3306)/agentcore") {
		t.Errorf("DSN format wrong: %s", dsn)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") {
		t.Errorf("DSN missing charset: %s", dsn)
	}
	if !strings.Contains(dsn, "parseTime=True") {
		t.Errorf("DSN missing parseTime: %s", dsn)
	}
}

func TestDBConfig_IsConfigured(t *testing.T) {
	if (DBConfig{Password: "", Name: "x"}).IsConfigured() {
		t.Error("empty password should not be configured")
	}
	if (DBConfig{Password: "p", Name: ""}).IsConfigured() {
		t.Error("empty name should not be configured")
	}
	if !(DBConfig{Password: "p", Name: "x"}).IsConfigured() {
		t.Error("password+name should be configured")
	}
}

func TestLLMConfig_IsConfigured(t *testing.T) {
	if (LLMConfig{APIKey: ""}).IsConfigured() {
		t.Error("empty APIKey should not be configured")
	}
	if !(LLMConfig{APIKey: "sk-xxx"}).IsConfigured() {
		t.Error("non-empty APIKey should be configured")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// 清空所有相关环境变量，验证默认值（Load 内部会尝试加载 configs/.env，
	// 但在测试环境该文件可能不存在，godotenv.Load 会安静失败）
	clearEnv(t)

	cfg := Load()

	if cfg.Server.Port != "8082" {
		t.Errorf("default Port = %q, want 8082", cfg.Server.Port)
	}
	if cfg.Server.Env != "development" {
		t.Errorf("default Env = %q", cfg.Server.Env)
	}
	if cfg.DB.Name != "agentcore" {
		t.Errorf("default DB Name = %q, want agentcore", cfg.DB.Name)
	}
	if cfg.LLM.Provider != "minimax" {
		t.Errorf("default LLM Provider = %q", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "MiniMax-M2.5" {
		t.Errorf("default LLM Model = %q", cfg.LLM.Model)
	}
	// 密码/Key 默认应为空（降级标记）
	if cfg.DB.IsConfigured() {
		t.Error("DB should not be configured without password")
	}
	if cfg.LLM.IsConfigured() {
		t.Error("LLM should not be configured without API key")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("LLM_API_KEY", "sk-test")
	t.Setenv("LLM_MODEL", "gpt-4o")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("JWT_EXPIRATION", "7200")

	cfg := Load()

	if cfg.Server.Port != "9999" {
		t.Errorf("Port = %q, want 9999 (env override)", cfg.Server.Port)
	}
	if !cfg.DB.IsConfigured() {
		t.Error("DB should be configured with password")
	}
	if cfg.DB.Name != "mydb" {
		t.Errorf("DB Name = %q, want mydb", cfg.DB.Name)
	}
	if !cfg.LLM.IsConfigured() {
		t.Error("LLM should be configured with API key")
	}
	if cfg.LLM.Model != "gpt-4o" {
		t.Errorf("LLM Model = %q, want gpt-4o", cfg.LLM.Model)
	}
	if cfg.JWT.Expiration != 7200 {
		t.Errorf("JWT Expiration = %d, want 7200", cfg.JWT.Expiration)
	}
}

func TestGetenvInt_InvalidFormat(t *testing.T) {
	t.Setenv("TEST_BAD_INT", "not-a-number")
	if got := getenvInt("TEST_BAD_INT", 42); got != 42 {
		t.Errorf("getenvInt with bad format = %d, want default 42", got)
	}
}

// clearEnv 清空所有配置相关环境变量，确保测试从干净状态开始。
// t.Setenv 会在测试结束自动还原，不会污染其他测试。
func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"SERVER_PORT", "APP_ENV",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"LLM_PROVIDER", "LLM_API_KEY", "LLM_BASE_URL", "LLM_MODEL",
		"EMBEDDING_MODEL", "EMBEDDING_BASE_URL", "EMBEDDING_API_KEY",
		"MILVUS_HOST", "MILVUS_PORT",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB",
		"JWT_SECRET", "JWT_EXPIRATION",
	}
	for _, k := range keys {
		// t.Setenv 设为空等价于清空（对 os.Getenv 而言返回空串）
		os.Unsetenv(k)
	}
}
