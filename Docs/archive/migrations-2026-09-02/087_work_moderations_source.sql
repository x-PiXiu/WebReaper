-- 087_work_moderations_source.sql
-- 32号 P2：机审标记——处置来源区分（admin 人工 / machine 机审）+ flagged 动作（待人工复核）。
ALTER TABLE work_moderations
  ADD COLUMN source VARCHAR(16) NOT NULL DEFAULT 'admin' COMMENT 'admin(人工处置)/machine(机审标记)' AFTER reason;
