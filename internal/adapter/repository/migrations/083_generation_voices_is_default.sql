-- 083: 音色白牌化（用户确认 2026-09-01）——平台默认音色标记
-- scope=platform 的音色中仅一条 is_default=true；用户不选音色时后端 fallback 到此音色。
ALTER TABLE generation_voices ADD COLUMN is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '平台默认音色（scope=platform 内仅一条）';
