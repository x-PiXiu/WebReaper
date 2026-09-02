-- 删除僵尸表 media_assets（F4 二选一：删——038 建表后无任何仓储/代码读写，
-- 素材库以文件系统/OSS 为唯一事实源（LocalMediaStore/OSSMediaStore 直接扫描），
-- DB 记录与实际存储脱节只会误导后续维护。保留表结构的"资产库"需求由
-- 素材元数据入库特性立项时再正规设计（含归属校验/配额/去重），不在本修复范围）。
DROP TABLE IF EXISTS media_assets;
