-- 089_avatar_video_url_width.sql
-- 真机测试（2026-09-01 链路测试）发现：Vidu 成片签名 URL 超过 512 字符，
-- 形象视频回填报 Error 1406 Data too long——即 00号文档"1条 avatar_video_url 为空"的根因。
-- 拓宽为 VARCHAR(1024)。
ALTER TABLE subject_assets
  MODIFY COLUMN avatar_video_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '链式形象视频产物（Vidu签名URL可达~900字符）';
