-- 052: 视频互动数据快照（数据回读）
-- 每日回读任务 + 手动刷新写入：按 job 维度存播放/点赞/评论/分享时间序列，
-- 作品详情 Drawer 画趋势、作品数据页汇总最新值。

CREATE TABLE video_metrics (
  id          VARCHAR(64) PRIMARY KEY COMMENT 'vm-{nano}',
  tenant_id   VARCHAR(64) NOT NULL,
  job_id      VARCHAR(64) NOT NULL COMMENT '发布任务 ID',
  platform    VARCHAR(32) NOT NULL,
  video_id    VARCHAR(64) NOT NULL COMMENT '平台内视频 ID（aweme_id）',
  views       BIGINT NOT NULL DEFAULT 0 COMMENT '播放',
  likes       BIGINT NOT NULL DEFAULT 0 COMMENT '点赞',
  comments    BIGINT NOT NULL DEFAULT 0 COMMENT '评论',
  shares      BIGINT NOT NULL DEFAULT 0 COMMENT '分享',
  collected_at DATETIME(3) NOT NULL COMMENT '采集时间',
  INDEX idx_video_metrics_tenant (tenant_id),
  INDEX idx_video_metrics_job_time (job_id, collected_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='视频互动数据快照（每日回读时间序列）';
