package mock

import (
	"context"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// ---- User 仓储 ----

type MockUserRepository struct {
	mu   sync.Mutex
	byUN map[string]entity.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{byUN: make(map[string]entity.User)}
}

func (r *MockUserRepository) Save(_ context.Context, u entity.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byUN[u.Username] = u
	return nil
}

func (r *MockUserRepository) FindByUsername(_ context.Context, username string) (entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byUN[username]
	if !ok {
		return entity.User{}, pkg.ErrNotFound
	}
	return u, nil
}

func (r *MockUserRepository) FindByID(_ context.Context, id string) (entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.byUN {
		if u.ID == id {
			return u, nil
		}
	}
	return entity.User{}, pkg.ErrNotFound
}

func (r *MockUserRepository) List(_ context.Context) ([]entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]entity.User, 0, len(r.byUN))
	for _, u := range r.byUN {
		out = append(out, u)
	}
	return out, nil
}

func (r *MockUserRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, u := range r.byUN {
		if u.ID == id {
			delete(r.byUN, k)
			return nil
		}
	}
	return nil
}

func (r *MockUserRepository) Count(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byUN), nil
}

// ---- AgentConfig 仓储 ----

type MockAgentConfigRepository struct {
	mu     sync.Mutex
	byName map[string]entity.AgentConfig
}

func NewMockAgentConfigRepository() *MockAgentConfigRepository {
	return &MockAgentConfigRepository{byName: make(map[string]entity.AgentConfig)}
}

func (r *MockAgentConfigRepository) Save(_ context.Context, cfg entity.AgentConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[cfg.Name] = cfg
	return nil
}

func (r *MockAgentConfigRepository) FindByName(_ context.Context, name string) (entity.AgentConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg, ok := r.byName[name]
	if !ok {
		return entity.AgentConfig{}, pkg.ErrNotFound
	}
	return cfg, nil
}

func (r *MockAgentConfigRepository) List(_ context.Context) ([]entity.AgentConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]entity.AgentConfig, 0, len(r.byName))
	for _, cfg := range r.byName {
		result = append(result, cfg)
	}
	return result, nil
}

func (r *MockAgentConfigRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byName, name)
	return nil
}

// ---- LLMConfig 仓储 ----

type MockLLMConfigRepository struct {
	mu     sync.Mutex
	byName map[string]entity.LLMConfig
}

func NewMockLLMConfigRepository() *MockLLMConfigRepository {
	return &MockLLMConfigRepository{byName: make(map[string]entity.LLMConfig)}
}

func (r *MockLLMConfigRepository) Save(_ context.Context, cfg entity.LLMConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[cfg.Name] = cfg
	return nil
}

func (r *MockLLMConfigRepository) FindByName(_ context.Context, name string) (entity.LLMConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg, ok := r.byName[name]
	if !ok {
		return entity.LLMConfig{}, pkg.ErrNotFound
	}
	return cfg, nil
}

func (r *MockLLMConfigRepository) List(_ context.Context) ([]entity.LLMConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]entity.LLMConfig, 0, len(r.byName))
	for _, cfg := range r.byName {
		result = append(result, cfg)
	}
	return result, nil
}

func (r *MockLLMConfigRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byName, name)
	return nil
}

func (r *MockLLMConfigRepository) FindByUsage(_ context.Context, usage string) (entity.LLMConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cfg := range r.byName {
		if cfg.Usage == usage || (usage == "" && cfg.Usage == "") {
			return cfg, nil
		}
	}
	return entity.LLMConfig{}, pkg.ErrNotFound
}

// ---- Conversation 仓储 ----

type MockConversationRepository struct {
	mu   sync.Mutex
	byID map[string]entity.Conversation
}

func NewMockConversationRepository() *MockConversationRepository {
	return &MockConversationRepository{byID: make(map[string]entity.Conversation)}
}

func (r *MockConversationRepository) Save(_ context.Context, conv entity.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[conv.ID] = conv
	return nil
}

func (r *MockConversationRepository) FindByID(_ context.Context, id string) (entity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok {
		return entity.Conversation{}, pkg.ErrNotFound
	}
	return c, nil
}

func (r *MockConversationRepository) ListByUser(_ context.Context, userID string) ([]entity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]entity.Conversation, 0, len(r.byID))
	for _, c := range r.byID {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *MockConversationRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

func (r *MockConversationRepository) UpdateTitle(_ context.Context, id, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.byID[id]; ok {
		c.Title = title
		r.byID[id] = c
	}
	return nil
}

// ---- Message 仓储 ----

type MockMessageRepository struct {
	mu   sync.Mutex
	msgs map[string][]entity.Message // key = conversationID
}

func NewMockMessageRepository() *MockMessageRepository {
	return &MockMessageRepository{msgs: make(map[string][]entity.Message)}
}

func (r *MockMessageRepository) Save(_ context.Context, msg entity.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs[msg.ConversationID] = append(r.msgs[msg.ConversationID], msg)
	return nil
}

func (r *MockMessageRepository) ListByConversation(_ context.Context, convID string) ([]entity.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]entity.Message, len(r.msgs[convID]))
	copy(out, r.msgs[convID])
	return out, nil
}

func (r *MockMessageRepository) DeleteByConversation(_ context.Context, convID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.msgs, convID)
	return nil
}

// ---- SystemSetting 仓储 ----

type MockSystemSettingRepository struct {
	mu       sync.Mutex
	settings map[string]entity.SystemSetting
}

func NewMockSystemSettingRepository() *MockSystemSettingRepository {
	return &MockSystemSettingRepository{settings: make(map[string]entity.SystemSetting)}
}

func (r *MockSystemSettingRepository) Get(_ context.Context, key string) (entity.SystemSetting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.settings[key]
	if !ok {
		return entity.SystemSetting{}, pkg.ErrNotFound
	}
	return s, nil
}

func (r *MockSystemSettingRepository) Save(_ context.Context, setting entity.SystemSetting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings[setting.Key] = setting
	return nil
}

// ---- GEO 内容仓储 mock（公开站点查询支持）----

// MockOptimizedContentRepository 是 port.OptimizedContentRepository 的内存实现。
type MockOptimizedContentRepository struct {
	mu   sync.Mutex
	recs map[string]entity.OptimizedContent
}

func NewMockOptimizedContentRepository() *MockOptimizedContentRepository {
	return &MockOptimizedContentRepository{recs: make(map[string]entity.OptimizedContent)}
}

func (r *MockOptimizedContentRepository) Save(_ context.Context, c entity.OptimizedContent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs[c.ID] = c
	return nil
}

func (r *MockOptimizedContentRepository) ListByBrand(_ context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entity.OptimizedContent
	for _, c := range r.recs {
		if c.TenantID == tenantID && c.BrandID == brandID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *MockOptimizedContentRepository) FindByID(_ context.Context, tenantID, id string) (entity.OptimizedContent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.recs[id]
	if !ok || c.TenantID != tenantID {
		return entity.OptimizedContent{}, pkg.ErrNotFound
	}
	return c, nil
}

func (r *MockOptimizedContentRepository) FindMaxVersion(_ context.Context, tenantID, brandID, keywordID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	maxV := 0
	for _, c := range r.recs {
		if c.TenantID == tenantID && c.BrandID == brandID &&
			(keywordID == "" || c.KeywordID == keywordID) && c.Version > maxV {
			maxV = c.Version
		}
	}
	return maxV, nil
}

// FindPublishedByID 公开查询：仅返回已发布内容。
// Delete 删除优化内容（mock：从 map 移除）。
func (r *MockOptimizedContentRepository) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.recs[id]
	if !ok || c.TenantID != tenantID {
		return nil
	}
	delete(r.recs, id)
	return nil
}

func (r *MockOptimizedContentRepository) FindPublishedByID(_ context.Context, id string) (entity.OptimizedContent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.recs[id]
	if !ok || c.Status != "published" {
		return entity.OptimizedContent{}, pkg.ErrNotFound
	}
	return c, nil
}

// ListPublished 公开查询：全部已发布内容。
func (r *MockOptimizedContentRepository) ListPublished(_ context.Context) ([]entity.OptimizedContent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entity.OptimizedContent
	for _, c := range r.recs {
		if c.Status == "published" {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *MockOptimizedContentRepository) Count(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.recs), nil
}

func (r *MockOptimizedContentRepository) CountPublished(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.recs {
		if c.Status == "published" {
			n++
		}
	}
	return n, nil
}

// ListAll 全平台内容（mock：不按租户过滤）。
func (r *MockOptimizedContentRepository) ListAll(_ context.Context, status string, limit int) ([]entity.OptimizedContent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entity.OptimizedContent
	for _, c := range r.recs {
		if status != "" && c.Status != status {
			continue
		}
		out = append(out, c)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// UpdateIndexStatus 更新内容收录状态（mock：直接改内存记录）。
func (r *MockOptimizedContentRepository) UpdateIndexStatus(_ context.Context, tenantID, id, status string, indexedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.recs[id]
	if !ok || c.TenantID != tenantID {
		return nil
	}
	c.IndexStatus = status
	c.IndexedAt = indexedAt
	r.recs[id] = c
	return nil
}

// ---- 收录提交日志 mock ----

// MockIndexingLogRepository 是 port.IndexingLogRepository 的内存实现。
type MockIndexingLogRepository struct {
	mu   sync.Mutex
	recs []entity.IndexingSubmitLog
}

func NewMockIndexingLogRepository() *MockIndexingLogRepository {
	return &MockIndexingLogRepository{}
}

func (r *MockIndexingLogRepository) Save(_ context.Context, log entity.IndexingSubmitLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, log)
	return nil
}

func (r *MockIndexingLogRepository) ListRecent(_ context.Context, limit int) ([]entity.IndexingSubmitLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]entity.IndexingSubmitLog, 0, len(r.recs))
	for i := len(r.recs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.recs[i])
	}
	return out, nil
}
