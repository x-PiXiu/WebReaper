-- 077: 官方音色精选标记（24 号计划 P4——前端"推荐"从 slice(0,12) 本地截断改为服务端标记）
ALTER TABLE generation_voices ADD COLUMN recommend TINYINT(1) NOT NULL DEFAULT 0 COMMENT '精选推荐（口播常用音色）';
-- 已有部署补标记（新库由 seed 携带；与 voices.go 的 Recommend:true 集合保持一致）
UPDATE generation_voices SET recommend = 1 WHERE voice_id IN (
  'male-qn-jingying',
  'female-shaonv',
  'female-yujie',
  'female-chengshu',
  'female-tianmei',
  'Chinese (Mandarin)_Reliable_Executive',
  'Chinese (Mandarin)_News_Anchor',
  'Chinese (Mandarin)_Gentleman'
);
