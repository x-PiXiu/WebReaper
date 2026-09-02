-- 085_voice_vidu_registration.sql
-- 31号计划：音色物化架构 L1——Vidu 注册状态跟踪。
-- 语义：注册关系是"可重建的缓存"（同 ID 复注册幂等）；NULL=未注册或未知（首次使用时注册）。
ALTER TABLE generation_voices
  ADD COLUMN vidu_registered_at DATETIME(3) NULL DEFAULT NULL
    COMMENT 'Vidu 侧注册/最近续期时间；NULL=未注册。同ID复注册幂等，过期前重注册即续期（窗口见 gen_vidu_voice_window）',
  ADD INDEX idx_voices_vidu_reg (vidu_registered_at);
