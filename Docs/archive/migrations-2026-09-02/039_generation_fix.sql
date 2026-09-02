-- 039_generation_fix.sql 生成任务表列类型修正
--
-- 背景：038 中 creations_json/params_json 建为 JSON 列——任务创建时
--   creations_json 为空字符串 ""（GORM string 字段默认值），MySQL JSON
--   列拒绝空串（Error 3140 Invalid JSON text: "The document is empty."）。
--   改为 TEXT：GORM string 字段无脑读写，兼容空值。

ALTER TABLE generation_tasks MODIFY creations_json TEXT NULL;
ALTER TABLE generation_tasks MODIFY params_json TEXT NULL;
