-- 029_publish_job_store_address.sql 发布任务记录门店地址（本地生活 P3）
--
-- 用途：发布时记录内容对应的门店地址（内容层本地曝光信号）——
--   · 落库留档：发布页展示"发布内容附带地址"
--   · 平台定位（P4 暂缓）：抖音/小红书"添加定位"需要地址参数，先留字段
--   · 视频域（goffmpeg/Vidu 暂缓）：视频文案地址注入复用的同一数据源

ALTER TABLE geo_publish_jobs ADD COLUMN store_address VARCHAR(256) DEFAULT '';
