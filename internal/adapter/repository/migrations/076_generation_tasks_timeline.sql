-- 076: B-Roll 台词时间轴（静音检测定位产物；NULL=未定位——22 号计划 §10.1③）
ALTER TABLE generation_tasks ADD COLUMN timeline_json TEXT NULL COMMENT 'B-Roll台词时间轴JSON（lines+script_source+align_mode；NULL=未定位）';
