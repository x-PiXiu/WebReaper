package knowledge

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"webreaper/internal/adapter/repository"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// searchScanLimit Search 的行业预取上限（防全表扫描失控；素材量级内足够）。
// 素材量进入万级后可平滑迁移 Milvus（VectorStore 接口已就位）。
const searchScanLimit = 500

// MySQLVectorStore 是 port.VectorStore 的 MySQL 实现
// （kb_materials.embedding JSON 列 + Go 侧余弦相似度）。
//
// 设计动机（开闭原则——仓储与向量存储解耦）：
//   余弦计算从 GormKnowledgeMaterialRepository 迁入本实现，
//   仓储改为依赖 VectorStore 接口——换 Milvus 不改仓储，只换驱动。
//
// 性能边界（诚实界定）：行业过滤后最多扫描 searchScanLimit 行（500），
// 素材量进入万级后可平滑迁移 Milvus（VectorStore 接口已就位）。
type MySQLVectorStore struct {
	db *gorm.DB
}

// NewMySQLVectorStore 创建 MySQL 向量存储。
func NewMySQLVectorStore(db *gorm.DB) *MySQLVectorStore {
	return &MySQLVectorStore{db: db}
}

// vectorFilterKeys 允许的 metadata 过滤键白名单（防 SQL 注入：键拼接、值参数化）。
var vectorFilterKeys = map[string]bool{"industry": true}

// Store 保存向量（写入 kb_materials.embedding 列；metadata 仅记录 industry 过滤用，不入列）。
func (s *MySQLVectorStore) Store(ctx context.Context, id string, vector []float32, _ map[string]string) error {
	emb := toFloat32JSON(vector)
	return s.db.WithContext(ctx).Model(&repository.KnowledgeMaterialPO{}).
		Where("id = ?", id).Update("embedding", emb).Error
}

// Search 按行业过滤 + 余弦相似度 topK（仅 active 且带向量；余弦 ≤ 0 视为无关剔除）。
func (s *MySQLVectorStore) Search(ctx context.Context, filter map[string]string, queryVector []float32, topK int) ([]port.VectorSearchResult, error) {
	q := s.db.WithContext(ctx).Where("status = ?", entity.MaterialStatusActive)
	for k, v := range filter {
		if !vectorFilterKeys[k] {
			continue // 白名单外键忽略（防注入）
		}
		q = q.Where(k+" = ?", v)
	}
	var pos []repository.KnowledgeMaterialPO
	if err := q.Order("created_at DESC").Limit(searchScanLimit).Find(&pos).Error; err != nil {
		return nil, err
	}

	results := make([]port.VectorSearchResult, 0, len(pos))
	for i := range pos {
		emb := fromFloat32JSON(pos[i].Embedding)
		if len(emb) == 0 {
			continue // 无向量（embedding 失败入库的素材）不可检索
		}
		score := cosine(queryVector, emb)
		if score <= 0 {
			continue // 余弦自然边界：无关内容不返回（业务阈值由检索器过滤）
		}
		results = append(results, port.VectorSearchResult{
			ID:    pos[i].ID,
			Score: score,
			Metadata: map[string]string{
				"industry": pos[i].Industry,
			},
		})
	}
	// 余弦降序，取 topK
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// Delete 删除向量（embedding 置 NULL；行数据由仓储负责）。
func (s *MySQLVectorStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Model(&repository.KnowledgeMaterialPO{}).
		Where("id = ?", id).Update("embedding", nil).Error
}

// IsAvailable MySQL 常驻可用（连接问题由上层 DB 处理）。
func (s *MySQLVectorStore) IsAvailable() bool { return true }

var _ port.VectorStore = (*MySQLVectorStore)(nil)

// toFloat32JSON []float32 → datatypes.JSON（nil/空 → "null"）。
func toFloat32JSON(v []float32) datatypes.JSON {
	if len(v) == 0 {
		return nil
	}
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

// fromFloat32JSON datatypes.JSON → []float32（nil/非法 → nil）。
func fromFloat32JSON(j datatypes.JSON) []float32 {
	if len(j) == 0 {
		return nil
	}
	var out []float32
	if err := json.Unmarshal(j, &out); err != nil {
		return nil
	}
	return out
}

// cosine 余弦相似度（维度不一致或空向量返回 0）。
func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
