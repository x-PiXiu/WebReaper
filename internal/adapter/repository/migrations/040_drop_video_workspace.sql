-- 040_drop_video_workspace 废弃旧视频生成链路（021_video_workspace 的产物）
--
-- 背景：统一生成任务（Vidu 全量接入：视频/图片/音频/数字人，038_generation.sql）
-- 取代了旧"视频工作台"链路（video_tasks/video_jobs + usecase/video + /api/v1/video/*）。
-- 旧链路无真实数据（演示/开发用），为消除双链路运维成本直接 DROP，
-- 勿在 040 之后回滚使用旧 video API（代码已删除）。

DROP TABLE IF EXISTS video_tasks;
DROP TABLE IF EXISTS video_jobs;
