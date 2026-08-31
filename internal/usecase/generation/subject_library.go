// subject_library.go 官方主体缓存代理（25 号阶段一——23 号计划 §2.2）。
//
// GET /api/v1/subjects?ownership=system：服务端 5min 内存缓存 + 透传分页，
// 防多租户高频刷 Vidu（官方主体基本静态）；不落库（无租户归属）。
package generation

import (
	"context"
	"fmt"
	"time"

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

// InvalidateOfficialSubjectCache 清空官方主体缓存（管理后台切换厂商/Key 后可调用）。
func (uc *GenerationUseCase) InvalidateOfficialSubjectCache() {
	uc.officialSubjectMu.Lock()
	uc.officialSubjectCache = nil
	uc.officialSubjectMu.Unlock()
}
