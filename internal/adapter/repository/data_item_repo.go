package repository

import (
	"context"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

type GormDataItemRepository struct{ db *gorm.DB }

// 编译期断言：实现 port.DataItemRepository。
var _ port.DataItemRepository = (*GormDataItemRepository)(nil)

func NewGormDataItemRepository(db *gorm.DB) *GormDataItemRepository {
	return &GormDataItemRepository{db: db}
}

func (r *GormDataItemRepository) Save(ctx context.Context, item entity.DataItem) error {
	po := dataItemToPO(item)
	return r.db.WithContext(ctx).Save(&po).Error
}

func (r *GormDataItemRepository) SaveBatch(ctx context.Context, items []entity.DataItem) error {
	pos := make([]DataItemPO, 0, len(items))
	for _, item := range items {
		pos = append(pos, dataItemToPO(item))
	}
	return r.db.WithContext(ctx).CreateInBatches(pos, 100).Error
}

func (r *GormDataItemRepository) FindByID(ctx context.Context, id string) (entity.DataItem, error) {
	var po DataItemPO
	if err := r.db.WithContext(ctx).First(&po, "id = ?", id).Error; err != nil {
		return entity.DataItem{}, err
	}
	return dataItemFromPO(po), nil
}

func (r *GormDataItemRepository) List(ctx context.Context, limit int) ([]entity.DataItem, error) {
	if limit <= 0 { limit = 50 }
	var pos []DataItemPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.DataItem, 0, len(pos))
	for _, p := range pos { result = append(result, dataItemFromPO(p)) }
	return result, nil
}

func (r *GormDataItemRepository) ListByCollection(ctx context.Context, collectionID string) ([]entity.DataItem, error) {
	var pos []DataItemPO
	if err := r.db.WithContext(ctx).Where("collection_id = ?", collectionID).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.DataItem, 0, len(pos))
	for _, p := range pos { result = append(result, dataItemFromPO(p)) }
	return result, nil
}

func (r *GormDataItemRepository) ListByStatus(ctx context.Context, status entity.ItemStatus) ([]entity.DataItem, error) {
	var pos []DataItemPO
	if err := r.db.WithContext(ctx).Where("status = ?", string(status)).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	result := make([]entity.DataItem, 0, len(pos))
	for _, p := range pos { result = append(result, dataItemFromPO(p)) }
	return result, nil
}

func (r *GormDataItemRepository) UpdateStatus(ctx context.Context, id string, status entity.ItemStatus) error {
	return r.db.WithContext(ctx).Model(&DataItemPO{}).Where("id = ?", id).Update("status", string(status)).Error
}

// Delete 删除数据项。
func (r *GormDataItemRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&DataItemPO{}).Error
}

// ---- 统计聚合（仪表盘用）----
//
// 诚实边界：JSON 列聚合用 MySQL 原生函数（JSON_EXTRACT / JSON_TABLE）。
// SQLite 不支持这些函数，故 SQLite 测试用 mock 仓储绕过（与 repository 测试边界声明一致）。

// CountByStatus 按状态分组计数。
func (r *GormDataItemRepository) CountByStatus(ctx context.Context) (map[string]int, error) {
	var rows []struct {
		Status string
		Cnt    int
	}
	if err := r.db.WithContext(ctx).Model(&DataItemPO{}).
		Select("status, count(*) as cnt").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int, len(rows))
	for _, row := range rows {
		result[row.Status] = row.Cnt
	}
	return result, nil
}

// DailyCounts 近 days 天每日新增量（按日期升序）。
// 用 DATE(created_at) 分组，兼容 MySQL/SQLite（DATE 是通用函数）。
func (r *GormDataItemRepository) DailyCounts(ctx context.Context, days int) ([]port.DailyCount, error) {
	var rows []struct {
		DateStr string
		Cnt     int
	}
	if err := r.db.WithContext(ctx).Model(&DataItemPO{}).
		Select("date(created_at) as date_str, count(*) as cnt").
		Where("created_at >= ?", gorm.Expr("CURRENT_DATE - INTERVAL ? DAY", days)).
		Group("date_str").Order("date_str ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]port.DailyCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, port.DailyCount{Date: row.DateStr, Count: row.Cnt})
	}
	return result, nil
}

// GroupByMetaKey 按 metadata 的某个 key 分组计数（MySQL JSON 函数）。
// 用于统计"数据源分布"等（key 如 crawler_type）。
func (r *GormDataItemRepository) GroupByMetaKey(ctx context.Context, key string) ([]port.GroupCount, error) {
	var rows []struct {
		Name string
		Cnt  int
	}
	// MySQL JSON path 格式：$.key —— 直接拼到字符串里
	path := `$.` + key
	if err := r.db.WithContext(ctx).Model(&DataItemPO{}).
		Select("JSON_UNQUOTE(JSON_EXTRACT(metadata, ?)) as name, count(*) as cnt", path).
		Where("JSON_EXTRACT(metadata, ?) IS NOT NULL", path).
		Group("name").Order("cnt DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]port.GroupCount, 0, len(rows))
	for _, row := range rows {
		if row.Name != "" {
			result = append(result, port.GroupCount{Name: row.Name, Count: row.Cnt})
		}
	}
	return result, nil
}

// TopTags 标签频次 Top N。
// tags 是 JSON 数组，用 MySQL JSON_TABLE 展开统计。
func (r *GormDataItemRepository) TopTags(ctx context.Context, limit int) ([]port.GroupCount, error) {
	if limit <= 0 {
		limit = 8
	}
	type tagRow struct {
		Tag string
		Cnt int
	}
	var rows []tagRow
	// MySQL JSON_TABLE 把 tags 数组展开成行再 GROUP BY
	sql := `SELECT t.tag, COUNT(*) as cnt FROM data_items,
		JSON_TABLE(tags, '$[*]' COLUMNS (tag VARCHAR(255) PATH '$')) t
		WHERE t.tag IS NOT NULL AND t.tag != ''
		GROUP BY t.tag ORDER BY cnt DESC LIMIT ?`
	if err := r.db.WithContext(ctx).Raw(sql, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]port.GroupCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, port.GroupCount{Name: row.Tag, Count: row.Cnt})
	}
	return result, nil
}
