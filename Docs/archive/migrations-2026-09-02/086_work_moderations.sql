-- 086_work_moderations.sql
-- 32号：作品管理与内容安全——处置表（管理动作与业务核心表解耦）。
-- work_key 与 WorksUseCase 的 WorkItem.ID 同构：g-{taskID}（成片）/ c-{contentID}（文章）。
-- 物理不删源数据（溯源留痕）；deleted 比 hidden 多"不可再发布"语义（发布拦截消费）。
CREATE TABLE work_moderations (
  id         VARCHAR(64) PRIMARY KEY,
  work_key   VARCHAR(64) NOT NULL,
  work_kind  VARCHAR(16) NOT NULL DEFAULT 'video',
  tenant_id  VARCHAR(64) NOT NULL DEFAULT '',
  action     VARCHAR(16) NOT NULL DEFAULT 'hidden',
  reason     VARCHAR(512) NOT NULL DEFAULT '',
  operator   VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_work_key (work_key),
  INDEX idx_wm_tenant (tenant_id),
  INDEX idx_wm_updated (updated_at)
);
