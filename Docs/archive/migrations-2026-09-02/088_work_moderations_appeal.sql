-- 088_work_moderations_appeal.sql
-- 32号 P2：申诉流程——被处置作品的用户申诉与管理端复核。
-- appeal_status: none(未申诉)/pending(申诉中)/accepted(已采纳=恢复)/rejected(已维持，终审)。
-- 防滥用：appealed_at 支撑"同一作品一天一次"；申诉文本过机审（防申诉通道成为违规内容展示位）。
ALTER TABLE work_moderations
  ADD COLUMN appeal_status VARCHAR(16) NOT NULL DEFAULT 'none' COMMENT 'none/pending/accepted/rejected' AFTER source,
  ADD COLUMN appeal_text VARCHAR(512) NOT NULL DEFAULT '' COMMENT '用户申诉理由' AFTER appeal_status,
  ADD COLUMN appealed_at DATETIME(3) NULL COMMENT '最近一次申诉时间（防滥用限频）' AFTER appeal_text;
