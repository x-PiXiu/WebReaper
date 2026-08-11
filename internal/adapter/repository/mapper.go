package repository

import (
	"encoding/json"

	"gorm.io/datatypes"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
)

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

// ---- Task ----

func taskToPO(e entity.Task) TaskPO {
	return TaskPO{
		ID: e.ID, Type: string(e.Type), Input: e.Input, Output: e.Output, Progress: e.Progress,
		Status: string(e.Status), Error: e.Error, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func taskFromPO(p TaskPO) entity.Task {
	return entity.Task{
		ID: p.ID, Type: entity.TaskType(p.Type), Input: p.Input, Output: p.Output, Progress: p.Progress,
		Status: valueobject.TaskStatus(p.Status), Error: p.Error, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
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
		BaseURL: e.BaseURL, Model: e.Model, CostPerMTok: e.CostPerMTok,
	}
}

func llmConfigFromPO(p LLMConfigPO) entity.LLMConfig {
	return entity.LLMConfig{
		Name: p.Name, Provider: p.Provider, APIKey: p.APIKey,
		BaseURL: p.BaseURL, Model: p.Model, CostPerMTok: p.CostPerMTok,
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

