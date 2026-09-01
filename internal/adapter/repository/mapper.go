package repository

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"gorm.io/datatypes"

	"webreaper/internal/domain/entity"
)

// ---- 时间辅助 ----
// MySQL 严格模式（NO_ZERO_DATE）拒绝 '0000-00-00' 零日期：
// 可空时间列（DATETIME NULL）的 PO 字段必须用 *time.Time——
// 零值 time.Time 写库会被驱动转成零日期报 Error 1292（云端宝塔 MySQL 已踩）。
// entity 层保持 time.Time，mapper 在此转换（nil ↔ 零值语义等价，业务层零改动）。

// timeToPtr entity(time.Time) → PO(*time.Time)：零值转 nil（写 NULL），有值转指针。
func timeToPtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// ptrToTime PO(*time.Time) → entity(time.Time)：nil 转零值。
func ptrToTime(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}

// ---- JSON 辅助 ----

func toJSON(ss []string) datatypes.JSON {
	if len(ss) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	b, _ := json.Marshal(ss)
	return datatypes.JSON(b)
}

func toStringSlice(j datatypes.JSON) []string {
	var ss []string
	_ = json.Unmarshal(j, &ss)
	if ss == nil {
		return []string{}
	}
	return ss
}

// toFloatMap map[string]float64 → JSON（竞品提及率等）。
func toFloatMap(m map[string]float64) datatypes.JSON {
	if len(m) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	b, _ := json.Marshal(m)
	return datatypes.JSON(b)
}

// toFloatMapFromJSON JSON → map[string]float64。
func toFloatMapFromJSON(j datatypes.JSON) map[string]float64 {
	m := map[string]float64{}
	_ = json.Unmarshal(j, &m)
	return m
}

// toStrMap map[string]string → JSON（竞品情感等字符串映射）。
func toStrMap(m map[string]string) datatypes.JSON {
	if len(m) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	b, _ := json.Marshal(m)
	return datatypes.JSON(b)
}

// toStrMapFromJSON JSON → map[string]string。
func toStrMapFromJSON(j datatypes.JSON) map[string]string {
	m := map[string]string{}
	_ = json.Unmarshal(j, &m)
	return m
}

func metadataToJSON(m map[string]string) datatypes.JSON {
	if len(m) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	b, _ := json.Marshal(m)
	return datatypes.JSON(b)
}

func metadataFromJSON(j datatypes.JSON) map[string]string {
	var m map[string]string
	_ = json.Unmarshal(j, &m)
	if m == nil {
		return map[string]string{}
	}
	return m
}

// ---- User ----

func userToPO(e entity.User) UserPO {
	return UserPO{
		ID: e.ID, Username: e.Username, PasswordHash: e.PasswordHash,
		Role: e.Role, TenantID: e.TenantID, CreatedAt: e.CreatedAt,
	}
}

func userFromPO(p UserPO) entity.User {
	return entity.User{
		ID: p.ID, Username: p.Username, PasswordHash: p.PasswordHash,
		Role: p.Role, TenantID: p.TenantID, CreatedAt: p.CreatedAt,
	}
}

// ---- AgentConfig ----

func agentConfigToPO(e entity.AgentConfig) AgentConfigPO {
	return AgentConfigPO{
		Name: e.Name, SystemPrompt: e.SystemPrompt, Tools: toJSON(e.Tools),
		LLMConfigName: e.LLMConfigName, MaxIterations: e.MaxIterations,
	}
}

func agentConfigFromPO(p AgentConfigPO) entity.AgentConfig {
	return entity.AgentConfig{
		Name: p.Name, SystemPrompt: p.SystemPrompt, Tools: toStringSlice(p.Tools),
		LLMConfigName: p.LLMConfigName, MaxIterations: p.MaxIterations,
	}
}

// ---- LLMConfig ----

func llmConfigToPO(e entity.LLMConfig) LLMConfigPO {
	return LLMConfigPO{
		Name: e.Name, Provider: e.Provider, APIKey: e.APIKey,
		BaseURL: e.BaseURL, Model: e.Model, CostPerMTok: e.CostPerMTok, Usage: e.Usage,
		IsDefault: e.IsDefault,
	}
}

func llmConfigFromPO(p LLMConfigPO) entity.LLMConfig {
	return entity.LLMConfig{
		Name: p.Name, Provider: p.Provider, APIKey: p.APIKey,
		BaseURL: p.BaseURL, Model: p.Model, CostPerMTok: p.CostPerMTok, Usage: p.Usage,
		IsDefault: p.IsDefault,
	}
}

// ---- Conversation / Message ----

func conversationToPO(e entity.Conversation) ConversationPO {
	return ConversationPO{
		ID: e.ID, Title: e.Title, AgentName: e.AgentName, UserID: e.UserID,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func conversationFromPO(p ConversationPO) entity.Conversation {
	return entity.Conversation{
		ID: p.ID, Title: p.Title, AgentName: p.AgentName, UserID: p.UserID,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func messageToPO(e entity.Message) MessagePO {
	return MessagePO{
		ID: e.ID, ConversationID: e.ConversationID, Role: e.Role,
		Content: e.Content, ToolCallsJSON: e.ToolCallsJSON, CreatedAt: e.CreatedAt,
	}
}

func messageFromPO(p MessagePO) entity.Message {
	return entity.Message{
		ID: p.ID, ConversationID: p.ConversationID, Role: p.Role,
		Content: p.Content, ToolCallsJSON: p.ToolCallsJSON, CreatedAt: p.CreatedAt,
	}
}

// ---- 泛型映射工具（27号优化——减少手写映射代码）----

// MapFields 自动映射同名字段（src → dst）。
//
// 规则：
//   - 字段名完全匹配时自动赋值
//   - 支持 `map:"-"` tag 跳过字段
//   - 支持 `map:"other_name"` tag 重命名
//   - 类型必须兼容（相同底层类型或可赋值）
//
// 返回实际映射的字段数。
func MapFields(dst, src any) int {
	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	mapped := 0
	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Type().Field(i)
		srcValue := srcVal.Field(i)

		// 检查 map tag
		tag := srcField.Tag.Get("map")
		if tag == "-" {
			continue
		}

		// 确定目标字段名
		dstFieldName := srcField.Name
		if tag != "" {
			dstFieldName = tag
		}

		// 查找目标字段
		dstField := dstVal.FieldByName(dstFieldName)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}

		// 类型兼容性检查
		if srcValue.Type().AssignableTo(dstField.Type()) {
			dstField.Set(srcValue)
			mapped++
		}
	}
	return mapped
}

// StructToMap 将 struct 转换为 map[string]any（用于 JSON 序列化/调试）。
func StructToMap(v any) map[string]any {
	result := make(map[string]any)
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return result
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		value := val.Field(i)

		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}

		key := field.Name
		if tag != "" {
			key = strings.Split(tag, ",")[0]
		}

		if value.CanInterface() {
			result[key] = value.Interface()
		}
	}
	return result
}

