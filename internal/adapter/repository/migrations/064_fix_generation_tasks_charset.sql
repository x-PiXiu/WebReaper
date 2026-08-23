-- 064_fix_generation_tasks_charset.sql
-- 修复各表的字符集和排序规则，确保中文参数正确存储
--
-- 问题：原表创建时未指定 collation，使用了服务器默认值，
-- 导致 params_json 中的中文字符在某些连接下显示为乱码。
-- 修复：统一使用 utf8mb4_unicode_ci 排序规则。
--
-- 注意：media_assets 表已在 migration 046 中被 drop，此处跳过。
-- generation_tasks/generation_specs 在 migration 038 中创建但未指定 collation，
-- 此处统一转换。已转换的表再次执行 CONVERT TO 是幂等操作（无数据变化）。

ALTER TABLE generation_tasks CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
ALTER TABLE generation_specs CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
ALTER TABLE generation_templates CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
ALTER TABLE generation_voices CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;