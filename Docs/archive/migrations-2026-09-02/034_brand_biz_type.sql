-- 034_brand_biz_type.sql 品牌业务类型（local/online 分流）
--
-- 背景：原系统所有品牌都按"本地生意"处理（门店必填+附近同行 POI 对比）。
-- 但线上业务（SaaS/工具/网络公司/内容站）同样需要 AI 可见度，路径完全不同——
-- 无地理约束、不该做"附近网络公司 POI"对比（荒谬）、问法是品类词而非本地词。
-- 加 biz_type 字段区分：local=本地生意 / online=线上业务（空=local 兼容存量）。

ALTER TABLE geo_brands ADD COLUMN biz_type VARCHAR(16) NOT NULL DEFAULT 'local';
