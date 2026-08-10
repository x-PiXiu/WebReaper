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

// ---- DataItem ----

func dataItemToPO(e entity.DataItem) DataItemPO {
	return DataItemPO{
		ID: e.ID, CollectionID: e.CollectionID, Title: e.Title, Content: e.Content,
		Summary: e.Summary, Tags: toJSON(e.Tags), SourceURL: e.SourceURL, RawContent: e.RawContent,
		Status: string(e.Status), Metadata: metadataToJSON(e.Metadata),
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func dataItemFromPO(p DataItemPO) entity.DataItem {
	return entity.DataItem{
		ID: p.ID, CollectionID: p.CollectionID, Title: p.Title, Content: p.Content,
		Summary: p.Summary, Tags: toStringSlice(p.Tags), SourceURL: p.SourceURL, RawContent: p.RawContent,
		Status: entity.ItemStatus(p.Status), Metadata: metadataFromJSON(p.Metadata),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// ---- Collection ----

func collectionToPO(e entity.Collection) CollectionPO {
	return CollectionPO{
		ID: e.ID, Name: e.Name, AgentName: e.AgentName, TaskID: e.TaskID,
		Status: string(e.Status), ItemCount: e.ItemCount, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func collectionFromPO(p CollectionPO) entity.Collection {
	return entity.Collection{
		ID: p.ID, Name: p.Name, AgentName: p.AgentName, TaskID: p.TaskID,
		Status: entity.CollectionStatus(p.Status), ItemCount: p.ItemCount, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// ---- AgentConfig ----

func agentConfigToPO(e entity.AgentConfig) AgentConfigPO {
	return AgentConfigPO{
		Name: e.Name, SystemPrompt: e.SystemPrompt, Tools: toJSON(e.Tools),
		LLMConfigName: e.LLMConfigName, MaxIterations: e.MaxIterations,
		AutoSave: e.AutoSave, FieldMapping: e.FieldMapping,
	}
}

func agentConfigFromPO(p AgentConfigPO) entity.AgentConfig {
	return entity.AgentConfig{
		Name: p.Name, SystemPrompt: p.SystemPrompt, Tools: toStringSlice(p.Tools),
		LLMConfigName: p.LLMConfigName, MaxIterations: p.MaxIterations,
		AutoSave: p.AutoSave, FieldMapping: p.FieldMapping,
	}
}

// ---- LLMConfig ----

func llmConfigToPO(e entity.LLMConfig) LLMConfigPO {
	return LLMConfigPO{
		Name: e.Name, Provider: e.Provider, APIKey: e.APIKey,
		BaseURL: e.BaseURL, Model: e.Model,
	}
}

func llmConfigFromPO(p LLMConfigPO) entity.LLMConfig {
	return entity.LLMConfig{
		Name: p.Name, Provider: p.Provider, APIKey: p.APIKey,
		BaseURL: p.BaseURL, Model: p.Model,
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

// ---- ExternalSystem ----

func externalSystemToPO(e entity.ExternalSystem) ExternalSystemPO {
	return ExternalSystemPO{
		Name: e.Name, Description: e.Description, Endpoint: e.Endpoint,
		Method: e.Method, Headers: e.Headers, Mode: e.Mode,
		FieldMapping: e.FieldMapping, BodyTemplate: e.BodyTemplate,
		ContentType: e.ContentType, Enabled: e.Enabled,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func externalSystemFromPO(p ExternalSystemPO) entity.ExternalSystem {
	mode := p.Mode
	if mode == "" {
		mode = entity.PublishModeRaw
	}
	return entity.ExternalSystem{
		Name: p.Name, Description: p.Description, Endpoint: p.Endpoint,
		Method: p.Method, Headers: p.Headers, Mode: mode,
		FieldMapping: p.FieldMapping, BodyTemplate: p.BodyTemplate,
		ContentType: p.ContentType, Enabled: p.Enabled,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// ---- PublishRecord（复用 001_init 的 publish_records 表）----

func publishRecordToView(r entity.PublishRecord) map[string]any {
	return map[string]any{
		"id":           r.ID,
		"content_id":   r.ContentID,
		"content_type": r.ContentType,
		"system_name":  r.SystemName,
		"success":      r.Success,
		"external_id":  r.ExternalID,
		"error_msg":    r.ErrorMsg,
		"result_at":    r.ResultAt,
		"created_at":   r.CreatedAt,
	}
}
