package mock

import (
	"context"
	"sort"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/domain/valueobject"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
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
	r.mu.Lock(); defer r.mu.Unlock()
	r.byUN[u.Username] = u
	return nil
}

func (r *MockUserRepository) FindByUsername(_ context.Context, username string) (entity.User, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	u, ok := r.byUN[username]
	if !ok { return entity.User{}, pkg.ErrNotFound }
	return u, nil
}

func (r *MockUserRepository) FindByID(_ context.Context, id string) (entity.User, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	for _, u := range r.byUN {
		if u.ID == id { return u, nil }
	}
	return entity.User{}, pkg.ErrNotFound
}

func (r *MockUserRepository) List(_ context.Context) ([]entity.User, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	out := make([]entity.User, 0, len(r.byUN))
	for _, u := range r.byUN { out = append(out, u) }
	return out, nil
}

func (r *MockUserRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	for k, u := range r.byUN {
		if u.ID == id { delete(r.byUN, k); return nil }
	}
	return nil
}

// ---- Task 仓储 ----

type MockTaskRepository struct {
	mu       sync.Mutex
	byID     map[string]entity.Task
}

func NewMockTaskRepository() *MockTaskRepository {
	return &MockTaskRepository{byID: make(map[string]entity.Task)}
}

func (r *MockTaskRepository) Save(_ context.Context, t entity.Task) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.byID[t.ID] = t
	return nil
}

func (r *MockTaskRepository) FindByID(_ context.Context, id string) (entity.Task, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok { return entity.Task{}, pkg.ErrNotFound }
	return t, nil
}

func (r *MockTaskRepository) List(_ context.Context, limit int) ([]entity.Task, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	if limit <= 0 { limit = 50 }
	result := make([]entity.Task, 0, len(r.byID))
	for _, t := range r.byID {
		result = append(result, t)
		if len(result) >= limit { break }
	}
	return result, nil
}

func (r *MockTaskRepository) UpdateStatus(_ context.Context, id string, status valueobject.TaskStatus, errMsg string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok { return pkg.ErrNotFound }
	t.Status = status; t.Error = errMsg
	r.byID[id] = t
	return nil
}

func (r *MockTaskRepository) UpdateOutput(_ context.Context, id string, output string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok { return pkg.ErrNotFound }
	t.Output = output
	r.byID[id] = t
	return nil
}

func (r *MockTaskRepository) UpdateProgress(_ context.Context, id string, progress string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok { return pkg.ErrNotFound }
	t.Progress = progress
	r.byID[id] = t
	return nil
}

// ---- DataItem 仓储 ----

type MockDataItemRepository struct {
	mu   sync.Mutex
	byID map[string]entity.DataItem
}

func NewMockDataItemRepository() *MockDataItemRepository {
	return &MockDataItemRepository{byID: make(map[string]entity.DataItem)}
}

func (r *MockDataItemRepository) Save(_ context.Context, item entity.DataItem) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.byID[item.ID] = item
	return nil
}

func (r *MockDataItemRepository) SaveBatch(_ context.Context, items []entity.DataItem) error {
	r.mu.Lock(); defer r.mu.Unlock()
	for _, item := range items { r.byID[item.ID] = item }
	return nil
}

func (r *MockDataItemRepository) FindByID(_ context.Context, id string) (entity.DataItem, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	item, ok := r.byID[id]
	if !ok { return entity.DataItem{}, pkg.ErrNotFound }
	return item, nil
}

func (r *MockDataItemRepository) List(_ context.Context, limit int) ([]entity.DataItem, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	if limit <= 0 { limit = 50 }
	result := make([]entity.DataItem, 0, len(r.byID))
	for _, item := range r.byID {
		result = append(result, item)
		if len(result) >= limit { break }
	}
	return result, nil
}

func (r *MockDataItemRepository) ListByCollection(_ context.Context, collectionID string) ([]entity.DataItem, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	var result []entity.DataItem
	for _, item := range r.byID {
		if item.CollectionID == collectionID { result = append(result, item) }
	}
	return result, nil
}

func (r *MockDataItemRepository) ListByStatus(_ context.Context, status entity.ItemStatus) ([]entity.DataItem, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	var result []entity.DataItem
	for _, item := range r.byID {
		if item.Status == status { result = append(result, item) }
	}
	return result, nil
}

func (r *MockDataItemRepository) UpdateStatus(_ context.Context, id string, status entity.ItemStatus) error {
	r.mu.Lock(); defer r.mu.Unlock()
	if item, ok := r.byID[id]; ok { item.Status = status; r.byID[id] = item }
	return nil
}

func (r *MockDataItemRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

// ---- 统计聚合（mock 实现：在内存数据上计算，不依赖 SQL）----

func (r *MockDataItemRepository) CountByStatus(_ context.Context) (map[string]int, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	m := map[string]int{}
	for _, item := range r.byID {
		m[string(item.Status)]++
	}
	return m, nil
}

func (r *MockDataItemRepository) DailyCounts(_ context.Context, days int) ([]port.DailyCount, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	m := map[string]int{}
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, item := range r.byID {
		if item.CreatedAt.After(cutoff) {
			m[item.CreatedAt.Format("2006-01-02")]++
		}
	}
	result := make([]port.DailyCount, 0, len(m))
	for d, c := range m {
		result = append(result, port.DailyCount{Date: d, Count: c})
	}
	// 按日期排序
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result, nil
}

func (r *MockDataItemRepository) GroupByMetaKey(_ context.Context, key string) ([]port.GroupCount, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	m := map[string]int{}
	for _, item := range r.byID {
		if v, ok := item.Metadata[key]; ok && v != "" {
			m[v]++
		}
	}
	result := make([]port.GroupCount, 0, len(m))
	for name, cnt := range m {
		result = append(result, port.GroupCount{Name: name, Count: cnt})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	return result, nil
}

func (r *MockDataItemRepository) TopTags(_ context.Context, limit int) ([]port.GroupCount, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	if limit <= 0 {
		limit = 8
	}
	m := map[string]int{}
	for _, item := range r.byID {
		for _, t := range item.Tags {
			if t != "" {
				m[t]++
			}
		}
	}
	result := make([]port.GroupCount, 0, len(m))
	for tag, cnt := range m {
		result = append(result, port.GroupCount{Name: tag, Count: cnt})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// ---- Collection 仓储 ----

type MockCollectionRepository struct {
	mu   sync.Mutex
	byID map[string]entity.Collection
}

func NewMockCollectionRepository() *MockCollectionRepository {
	return &MockCollectionRepository{byID: make(map[string]entity.Collection)}
}

func (r *MockCollectionRepository) Save(_ context.Context, c entity.Collection) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.byID[c.ID] = c
	return nil
}

func (r *MockCollectionRepository) FindByID(_ context.Context, id string) (entity.Collection, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok { return entity.Collection{}, pkg.ErrNotFound }
	return c, nil
}

func (r *MockCollectionRepository) List(_ context.Context, limit int) ([]entity.Collection, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	if limit <= 0 { limit = 50 }
	result := make([]entity.Collection, 0, len(r.byID))
	for _, c := range r.byID {
		result = append(result, c)
		if len(result) >= limit { break }
	}
	return result, nil
}

func (r *MockCollectionRepository) UpdateStatus(_ context.Context, id string, status entity.CollectionStatus) error {
	r.mu.Lock(); defer r.mu.Unlock()
	if c, ok := r.byID[id]; ok { c.Status = status; r.byID[id] = c }
	return nil
}

// ---- AgentConfig 仓储 ----

type MockAgentConfigRepository struct {
	mu       sync.Mutex
	byName   map[string]entity.AgentConfig
}

func NewMockAgentConfigRepository() *MockAgentConfigRepository {
	return &MockAgentConfigRepository{byName: make(map[string]entity.AgentConfig)}
}

func (r *MockAgentConfigRepository) Save(_ context.Context, cfg entity.AgentConfig) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.byName[cfg.Name] = cfg
	return nil
}

func (r *MockAgentConfigRepository) FindByName(_ context.Context, name string) (entity.AgentConfig, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	cfg, ok := r.byName[name]
	if !ok { return entity.AgentConfig{}, pkg.ErrNotFound }
	return cfg, nil
}

func (r *MockAgentConfigRepository) List(_ context.Context) ([]entity.AgentConfig, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	result := make([]entity.AgentConfig, 0, len(r.byName))
	for _, cfg := range r.byName { result = append(result, cfg) }
	return result, nil
}

func (r *MockAgentConfigRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock(); defer r.mu.Unlock()
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
	r.mu.Lock(); defer r.mu.Unlock()
	r.byName[cfg.Name] = cfg
	return nil
}

func (r *MockLLMConfigRepository) FindByName(_ context.Context, name string) (entity.LLMConfig, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	cfg, ok := r.byName[name]
	if !ok { return entity.LLMConfig{}, pkg.ErrNotFound }
	return cfg, nil
}

func (r *MockLLMConfigRepository) List(_ context.Context) ([]entity.LLMConfig, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	result := make([]entity.LLMConfig, 0, len(r.byName))
	for _, cfg := range r.byName { result = append(result, cfg) }
	return result, nil
}

func (r *MockLLMConfigRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	delete(r.byName, name)
	return nil
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
	r.mu.Lock(); defer r.mu.Unlock()
	r.byID[conv.ID] = conv
	return nil
}

func (r *MockConversationRepository) FindByID(_ context.Context, id string) (entity.Conversation, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok { return entity.Conversation{}, pkg.ErrNotFound }
	return c, nil
}

func (r *MockConversationRepository) ListByUser(_ context.Context, userID string) ([]entity.Conversation, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	result := make([]entity.Conversation, 0, len(r.byID))
	for _, c := range r.byID {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *MockConversationRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

func (r *MockConversationRepository) UpdateTitle(_ context.Context, id, title string) error {
	r.mu.Lock(); defer r.mu.Unlock()
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
	r.mu.Lock(); defer r.mu.Unlock()
	r.msgs[msg.ConversationID] = append(r.msgs[msg.ConversationID], msg)
	return nil
}

func (r *MockMessageRepository) ListByConversation(_ context.Context, convID string) ([]entity.Message, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	out := make([]entity.Message, len(r.msgs[convID]))
	copy(out, r.msgs[convID])
	return out, nil
}

func (r *MockMessageRepository) DeleteByConversation(_ context.Context, convID string) error {
	r.mu.Lock(); defer r.mu.Unlock()
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
	r.mu.Lock(); defer r.mu.Unlock()
	s, ok := r.settings[key]
	if !ok { return entity.SystemSetting{}, pkg.ErrNotFound }
	return s, nil
}

func (r *MockSystemSettingRepository) Save(_ context.Context, setting entity.SystemSetting) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.settings[setting.Key] = setting
	return nil
}

// ---- ExternalSystem 仓储 ----

type MockExternalSystemRepository struct {
	mu    sync.Mutex
	byName map[string]entity.ExternalSystem
}

func NewMockExternalSystemRepository() *MockExternalSystemRepository {
	return &MockExternalSystemRepository{byName: make(map[string]entity.ExternalSystem)}
}

func (r *MockExternalSystemRepository) Save(_ context.Context, sys entity.ExternalSystem) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.byName[sys.Name] = sys
	return nil
}

func (r *MockExternalSystemRepository) FindByName(_ context.Context, name string) (entity.ExternalSystem, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	s, ok := r.byName[name]
	if !ok { return entity.ExternalSystem{}, pkg.ErrNotFound }
	return s, nil
}

func (r *MockExternalSystemRepository) List(_ context.Context) ([]entity.ExternalSystem, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	result := make([]entity.ExternalSystem, 0, len(r.byName))
	for _, s := range r.byName { result = append(result, s) }
	return result, nil
}

func (r *MockExternalSystemRepository) Delete(_ context.Context, name string) error {
	r.mu.Lock(); defer r.mu.Unlock()
	delete(r.byName, name)
	return nil
}

// ---- PublishRecord 仓储 ----

type MockPublishRecordRepository struct {
	mu   sync.Mutex
	recs []entity.PublishRecord
}

func NewMockPublishRecordRepository() *MockPublishRecordRepository {
	return &MockPublishRecordRepository{}
}

func (r *MockPublishRecordRepository) Save(_ context.Context, rec entity.PublishRecord) error {
	r.mu.Lock(); defer r.mu.Unlock()
	r.recs = append(r.recs, rec)
	return nil
}

func (r *MockPublishRecordRepository) ListByContent(_ context.Context, contentID string) ([]entity.PublishRecord, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	var result []entity.PublishRecord
	for _, rec := range r.recs {
		if rec.ContentID == contentID { result = append(result, rec) }
	}
	return result, nil
}

func (r *MockPublishRecordRepository) FindDedup(_ context.Context, contentID, systemName string) (entity.PublishRecord, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	for _, rec := range r.recs {
		if rec.ContentID == contentID && rec.SystemName == systemName && rec.Success {
			return rec, nil
		}
	}
	return entity.PublishRecord{}, pkg.ErrNotFound
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
	r.mu.Lock(); defer r.mu.Unlock()
	r.recs[c.ID] = c
	return nil
}

func (r *MockOptimizedContentRepository) ListByBrand(_ context.Context, tenantID, brandID string) ([]entity.OptimizedContent, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	var out []entity.OptimizedContent
	for _, c := range r.recs {
		if c.TenantID == tenantID && c.BrandID == brandID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *MockOptimizedContentRepository) FindByID(_ context.Context, tenantID, id string) (entity.OptimizedContent, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	c, ok := r.recs[id]
	if !ok || c.TenantID != tenantID {
		return entity.OptimizedContent{}, pkg.ErrNotFound
	}
	return c, nil
}

func (r *MockOptimizedContentRepository) FindMaxVersion(_ context.Context, tenantID, brandID, keywordID string) (int, error) {
	r.mu.Lock(); defer r.mu.Unlock()
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
func (r *MockOptimizedContentRepository) FindPublishedByID(_ context.Context, id string) (entity.OptimizedContent, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	c, ok := r.recs[id]
	if !ok || c.Status != "published" {
		return entity.OptimizedContent{}, pkg.ErrNotFound
	}
	return c, nil
}

// ListPublished 公开查询：全部已发布内容。
func (r *MockOptimizedContentRepository) ListPublished(_ context.Context) ([]entity.OptimizedContent, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	var out []entity.OptimizedContent
	for _, c := range r.recs {
		if c.Status == "published" {
			out = append(out, c)
		}
	}
	return out, nil
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
	r.mu.Lock(); defer r.mu.Unlock()
	r.recs = append(r.recs, log)
	return nil
}

func (r *MockIndexingLogRepository) ListRecent(_ context.Context, limit int) ([]entity.IndexingSubmitLog, error) {
	r.mu.Lock(); defer r.mu.Unlock()
	out := make([]entity.IndexingSubmitLog, 0, len(r.recs))
	for i := len(r.recs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.recs[i])
	}
	return out, nil
}
