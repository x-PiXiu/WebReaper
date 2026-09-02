-- 062_extend_generation_specs.sql
-- 扩展generation_specs表，新增provider/is_default/sort_order字段
-- 用于按厂商区分的模型配置管理

-- 新增厂商字段
ALTER TABLE generation_specs ADD COLUMN provider VARCHAR(50) DEFAULT 'vidu' AFTER sub_type;

-- 新增默认模型标记
ALTER TABLE generation_specs ADD COLUMN is_default BOOLEAN DEFAULT FALSE AFTER enabled;

-- 新增排序字段
ALTER TABLE generation_specs ADD COLUMN sort_order INT DEFAULT 0 AFTER is_default;

-- 更新现有数据，设置provider为'vidu'
UPDATE generation_specs SET provider = 'vidu' WHERE provider = '';

-- 设置默认模型（每个端点的第一个模型为默认）
-- 注意：MySQL 不允许在 UPDATE 子查询中引用同一表，使用 JOIN 解决
UPDATE generation_specs gs
INNER JOIN (
    SELECT sub_type, MIN(model) as min_model FROM generation_specs GROUP BY sub_type
) AS defaults ON gs.sub_type = defaults.sub_type AND gs.model = defaults.min_model
SET gs.is_default = TRUE;

-- 添加索引
ALTER TABLE generation_specs ADD INDEX idx_provider (provider);
ALTER TABLE generation_specs ADD INDEX idx_is_default (is_default);
