-- 053: 发布任务记录实际执行通道（发布域三轴重构：link/rpa/api 多通道共存）
-- 降级链"启动前短路切换"的实际落点；管理后台通道切换后的审计依据。

ALTER TABLE geo_publish_jobs ADD COLUMN transport VARCHAR(16) DEFAULT '' NOT NULL COMMENT '实际执行通道：link/rpa/api（空=历史数据）';
