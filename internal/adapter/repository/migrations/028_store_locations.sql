-- 028_store_locations.sql  门店档案域（本地生活 GEO 地基）
--
-- 背景：Brand 只有定位/卖点/竞品，缺位置维度。门店档案为本地化改造提供
--   NAP（Name/Address/Phone）信号、地理编码结果（经纬度/区划）与后续
--   周边竞品搜索/发布定位的数据源。
--
-- 多租户：tenant_id + 索引，强制按租户隔离。

CREATE TABLE IF NOT EXISTS geo_store_locations (
    id           VARCHAR(64) PRIMARY KEY,
    tenant_id    VARCHAR(64) NOT NULL,
    brand_id     VARCHAR(64) NOT NULL,
    name         VARCHAR(128),            -- 门店名（默认品牌名）
    address      VARCHAR(256) NOT NULL,   -- 详细地址（地理编码输入）
    city         VARCHAR(64),             -- 城市（地理编码回填）
    district     VARCHAR(64),             -- 区/县（地理编码回填）
    adcode       VARCHAR(16),             -- 行政区划代码（地理编码回填）
    lat          DECIMAL(10,6),           -- 纬度（地理编码回填）
    lng          DECIMAL(10,6),           -- 经度（地理编码回填）
    phone        VARCHAR(32),             -- 联系电话
    hours        VARCHAR(64),             -- 营业时间（如 10:00-22:00）
    price_level  VARCHAR(16),             -- 人均消费档位
    biz_type     VARCHAR(32) NOT NULL DEFAULT 'LocalBusiness', -- 业态（LocalBusiness/Restaurant/Cafe/Bar/Store，JSON-LD 用）
    geo_status   VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending/ok/failed
    created_at   DATETIME(3),
    updated_at   DATETIME(3),
    INDEX idx_geo_store_tenant (tenant_id),
    INDEX idx_geo_store_brand (brand_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
