-- 007_task_progress.sql  任务运行进度字段
--
-- 让任务监控页能看到运行中任务的实时进度（如"正在采集..."），
-- 而非只有最终结果。

ALTER TABLE tasks ADD COLUMN progress TEXT AFTER output;
