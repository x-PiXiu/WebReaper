// subject_library.go 主体库代理（25 号阶段一——23 号计划 §2.2）。
//
// 官方主体：GET /api/v1/subjects?ownership=system —— 服务端 5min 内存缓存 + 透传分页，
// 防多租户高频刷 Vidu（官方主体基本静态）；不落库（无租户归属）。
// 个人分身：GET /api/v1/subjects?ownership=private —— 从本地 generation_tasks 表聚合
// sub_type=subject AND state=success 的已注册主体，返回 SubjectInfo 格式供前端统一渲染。
package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

const officialSubjectCacheTTL = 5 * time.Minute

type officialSubjectCacheEntry struct {
	result port.SubjectListResult
	at     time.Time
}

// ListOfficialSubjects 官方主体分页查询（缓存键=page_token）。
func (uc *GenerationUseCase) ListOfficialSubjects(ctx context.Context, pageToken string, count int) (port.SubjectListResult, error) {
	provider, err := uc.getProvider(ctx, "reference2video")
	if err != nil {
		return port.SubjectListResult{}, err
	}
	lister, ok := provider.(port.SubjectLister)
	if !ok {
		return port.SubjectListResult{}, fmt.Errorf("当前生成厂商（%s）不支持主体库查询——官方主体暂未开放", provider.Name())
	}

	key := pageToken
	uc.officialSubjectMu.Lock()
	if uc.officialSubjectCache == nil {
		uc.officialSubjectCache = map[string]officialSubjectCacheEntry{}
	}
	if hit, ok := uc.officialSubjectCache[key]; ok && time.Since(hit.at) < officialSubjectCacheTTL {
		uc.officialSubjectMu.Unlock()
		return hit.result, nil
	}
	uc.officialSubjectMu.Unlock()

	res, err := lister.ListSubjects(ctx, "system", pageToken, count)
	if err != nil {
		return port.SubjectListResult{}, err
	}
	uc.officialSubjectMu.Lock()
	uc.officialSubjectCache[key] = officialSubjectCacheEntry{result: res, at: time.Now()}
	uc.officialSubjectMu.Unlock()
	return res, nil
}

// ListPersonalSubjects 个人分身列表（本地聚合——sub_type=subject AND state=success）。
//
// 数据来源：generation_tasks 表中该租户已注册成功的主体任务。
// 转换逻辑：
//   - server_id = ProviderTaskID（Vidu 主体 API 返回的资源 ID）优先，
//     空则取 creations[0].id（兜底——同步端点可能 creations 直接携带）
//   - name = params.name（提交时用户填写的主体名称）
//   - images = params.images（注册时上传的参考图）
//   - voice_id = params.voice_id（绑定音色，可选）
//
// 排除环境主体（params.kind=scene）——环境主体是场景，不是人物分身。
func (uc *GenerationUseCase) ListPersonalSubjects(ctx context.Context, tenantID string, limit int) (port.SubjectListResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tasks, err := uc.repo.ListBySubType(ctx, tenantID, "subject", entity.TaskStateSuccess, limit)
	if err != nil {
		return port.SubjectListResult{}, fmt.Errorf("查询个人分身失败: %w", err)
	}

	var subjects []port.SubjectInfo
	for _, t := range tasks {
		info := taskToSubjectInfo(t)
		if info == nil {
			continue // 环境主体或解析失败——跳过
		}
		subjects = append(subjects, *info)
	}
	return port.SubjectListResult{
		Subjects: subjects,
		Count:    len(subjects),
	}, nil
}

// taskToSubjectInfo 将已注册主体任务转换为 SubjectInfo。
// 返回 nil 表示应跳过（环境主体或数据不完整）。
func taskToSubjectInfo(t entity.GenerationTask) *port.SubjectInfo {
	var p map[string]any
	if err := json.Unmarshal([]byte(t.ParamsJSON), &p); err != nil {
		return nil
	}
	// 排除环境主体
	if k, _ := p["kind"].(string); k == "scene" {
		return nil
	}

	serverID := t.ProviderTaskID
	if serverID == "" {
		serverID = firstCreationID(t.CreationsJSON)
	}
	if serverID == "" {
		return nil // 无 server_id 不可用
	}

	name, _ := p["name"].(string)
	if name == "" {
		name = "未命名分身"
	}
	voiceID, _ := p["voice_id"].(string)

	// 解析 images
	var images []string
	if raw, ok := p["images"]; ok {
		switch v := raw.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					images = append(images, s)
				}
			}
		case []string:
			images = v
		}
	}

	return &port.SubjectInfo{
		ServerID: serverID,
		Name:     name,
		Images:   images,
		VoiceID:  voiceID,
	}
}

// InvalidateOfficialSubjectCache 清空官方主体缓存（管理后台切换厂商/Key 后可调用）。
func (uc *GenerationUseCase) InvalidateOfficialSubjectCache() {
	uc.officialSubjectMu.Lock()
	uc.officialSubjectCache = nil
	uc.officialSubjectMu.Unlock()
}
