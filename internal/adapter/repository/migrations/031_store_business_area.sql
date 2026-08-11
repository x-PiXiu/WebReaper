-- 031_store_business_area.sql 门店商圈补全（P1：逆地理编码回填）
--
-- 背景：商圈（business_area）是本地关键词生成/监测问法从"区"级精确到"商圈"级
--   的关键维度（"望京有什么川菜馆" vs "朝阳区有什么川菜馆"）。
-- 数据源：地理编码成功后调用逆地理编码（v3/geocode/regeo + extensions=all）
--   的 businessAreas[0].name 回填；无商圈数据或未配置地图服务时保持空。

ALTER TABLE geo_store_locations ADD COLUMN business_area VARCHAR(64) DEFAULT '';
