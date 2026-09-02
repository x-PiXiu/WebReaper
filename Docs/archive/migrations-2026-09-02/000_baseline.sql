-- ============================================================
-- 000_baseline.sql —— WebReaper 数据库完整基线
-- 从本地 WSL UbuntuOpenclaw MySQL webreaper 库导出（2026-09-02）
-- 53 张表结构 + 生产种子数据（管理员/套餐/平台配置/302条音色库）
-- 使用数据（任务/监测/品牌/作品等 41 张表只结构不数据）
--
-- 迁移器顺序：000 先建全量表+种子 → 001~089 增量 ALTER（幂等跳过已有表）
-- ============================================================
-- ============================================================
-- WebReaper 数据库基线（001）
-- 生成：2026-09-02 从本地 WSL UbuntuOpenclaw MySQL webreaper 库导出
-- 内容：53 张表结构 + 生产种子数据（不含使用数据）
-- 种子数据表：13 张（plans/users/配置/音色库/规格/模板/提示词/LLM/集成/设置）
-- 使用数据表：41 张（只导结构，数据为空——云端从零开始）
-- ============================================================

-- §1 表结构（53 张表，含索引/外键）
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `account_brand_bindings` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '租户ID',
  `account_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '账号ID',
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '品牌ID',
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '平台标识',
  `is_default` tinyint(1) DEFAULT '0' COMMENT '是否默认账号',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_account_brand` (`account_id`,`brand_id`),
  KEY `idx_tenant_brand` (`tenant_id`,`brand_id`),
  KEY `idx_tenant_account` (`tenant_id`,`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `agent_configs` (
  `name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `system_prompt` text COLLATE utf8mb4_unicode_ci,
  `tools` json DEFAULT NULL,
  `model` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `llm_config_name` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `max_iterations` int DEFAULT '10',
  `auto_save` tinyint(1) DEFAULT '0',
  `field_mapping` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `brand_inspirations` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `video_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `search_keyword` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `relevance_score` double NOT NULL DEFAULT '0',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_brand_video` (`brand_id`,`video_id`),
  KEY `idx_brand_id` (`brand_id`),
  KEY `idx_video_id` (`video_id`)
) ENGINE=InnoDB AUTO_INCREMENT=24 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `brand_publish_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '租户ID',
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '品牌ID',
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '平台标识（douyin/kuaishou/xiaohongshu/weixin/bilibili）',
  `account_ids` json DEFAULT NULL COMMENT '绑定的账号ID列表',
  `rate_limit` json DEFAULT NULL COMMENT '限速配置 {"max_per_day":5,"max_per_hour":2,"min_interval":1800}',
  `default_tags` json DEFAULT NULL COMMENT '品牌默认标签',
  `default_persona` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '默认人设ID',
  `is_active` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_brand_platform` (`tenant_id`,`brand_id`,`platform`),
  KEY `idx_tenant_brand` (`tenant_id`,`brand_id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `collections` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `agent_name` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `task_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'collecting',
  `item_count` int DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_collections_task` (`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `conversations` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `title` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `agent_name` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `user_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_conversations_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `crawl_results` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL,
  `crawler_type` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `title` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `raw_content` longtext COLLATE utf8mb4_unicode_ci,
  `summary` longtext COLLATE utf8mb4_unicode_ci,
  `tags` json DEFAULT NULL,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending_review',
  `task_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_crawl_status` (`status`),
  KEY `idx_crawl_task` (`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `crawler_accounts` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `account_name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `cookie_encrypted` text COLLATE utf8mb4_unicode_ci NOT NULL,
  `user_agent` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `proxy_address` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `last_used_at` datetime(3) DEFAULT NULL,
  `last_health_check_at` datetime(3) DEFAULT NULL,
  `health_check_result` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'unknown',
  `daily_usage_count` int NOT NULL DEFAULT '0',
  `daily_usage_limit` int NOT NULL DEFAULT '50',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_platform_status` (`platform`,`status`)
) ENGINE=InnoDB AUTO_INCREMENT=7 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `crawler_configs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `search_keywords` json DEFAULT NULL,
  `extra_keywords` json DEFAULT NULL,
  `keyword_pool` json DEFAULT NULL,
  `last_keyword_index` int NOT NULL DEFAULT '0',
  `crawl_interval_minutes` int NOT NULL DEFAULT '15',
  `max_results` int NOT NULL DEFAULT '20',
  `sort_by` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'popular',
  `publish_time` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'week',
  `enable_comments` tinyint(1) NOT NULL DEFAULT '0',
  `enable_refresh` tinyint(1) NOT NULL DEFAULT '1',
  `refresh_interval_hours` int NOT NULL DEFAULT '12',
  `rate_limit_per_min` int NOT NULL DEFAULT '10',
  `proxy_enabled` tinyint(1) NOT NULL DEFAULT '0',
  `max_retry_count` int NOT NULL DEFAULT '3',
  `last_crawled_at` datetime(3) DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_platform_tenant` (`platform`,`tenant_id`),
  KEY `idx_brand_id` (`brand_id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `crawler_task_logs` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `task_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `trigger_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'scheduled',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'running',
  `keywords_used` json DEFAULT NULL,
  `videos_found` int NOT NULL DEFAULT '0',
  `videos_new` int NOT NULL DEFAULT '0',
  `videos_updated` int NOT NULL DEFAULT '0',
  `error_message` text COLLATE utf8mb4_unicode_ci,
  `started_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `finished_at` datetime(3) DEFAULT NULL,
  `duration_ms` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_platform` (`platform`),
  KEY `idx_status` (`status`),
  KEY `idx_started` (`started_at` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `data_items` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `collection_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `title` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` longtext COLLATE utf8mb4_unicode_ci,
  `summary` text COLLATE utf8mb4_unicode_ci,
  `tags` json DEFAULT NULL,
  `source_url` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `raw_content` longtext COLLATE utf8mb4_unicode_ci,
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT 'pending_review',
  `metadata` json DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_items_collection` (`collection_id`),
  KEY `idx_items_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `external_systems` (
  `name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `description` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `endpoint` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `method` varchar(8) COLLATE utf8mb4_unicode_ci DEFAULT 'POST',
  `headers` text COLLATE utf8mb4_unicode_ci,
  `mode` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT 'raw',
  `field_mapping` text COLLATE utf8mb4_unicode_ci,
  `body_template` text COLLATE utf8mb4_unicode_ci,
  `content_type` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `enabled` tinyint(1) DEFAULT '1',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `generation_specs` (
  `sub_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `provider` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT 'vidu',
  `model` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `endpoint` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `is_default` tinyint(1) DEFAULT '0',
  `sort_order` int DEFAULT '0',
  `capabilities_json` json DEFAULT NULL,
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `cost_credits` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`sub_type`,`model`),
  KEY `idx_provider` (`provider`),
  KEY `idx_is_default` (`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `generation_tasks` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `sub_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `model` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `provider` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'vidu',
  `provider_task_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `state` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'created',
  `err_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `err_msg` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `params_json` text COLLATE utf8mb4_unicode_ci,
  `payload` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `creations_json` text COLLATE utf8mb4_unicode_ci,
  `credits` int NOT NULL DEFAULT '0',
  `off_peak` tinyint(1) NOT NULL DEFAULT '0',
  `watermark` tinyint(1) NOT NULL DEFAULT '0',
  `callback_received` tinyint(1) NOT NULL DEFAULT '0',
  `callback_at` datetime(3) DEFAULT NULL,
  `retry_count` int NOT NULL DEFAULT '0',
  `params_hash` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `finished_at` datetime(3) DEFAULT NULL,
  `timeline_json` text COLLATE utf8mb4_unicode_ci COMMENT 'B-Roll台词时间轴JSON（lines+script_source+align_mode；NULL=未定位）',
  PRIMARY KEY (`id`),
  KEY `idx_gen_tenant` (`tenant_id`,`created_at`),
  KEY `idx_gen_provider_task` (`provider_task_id`),
  KEY `idx_gen_state` (`state`),
  KEY `idx_gen_hash` (`tenant_id`,`params_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `generation_templates` (
  `id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '租户ID（空=全局模板）',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '模板名称',
  `description` text COLLATE utf8mb4_unicode_ci COMMENT '模板描述',
  `icon` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '模板图标',
  `sub_type` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '端点类型（img2video/text2video/digital_human/...）',
  `default_params` json DEFAULT NULL COMMENT '默认参数（duration/resolution/...）',
  `required_materials` json DEFAULT NULL COMMENT '必需素材类型（image/video/audio）',
  `optional_materials` json DEFAULT NULL COMMENT '可选素材类型',
  `sort_order` int DEFAULT '0' COMMENT '排序',
  `enabled` tinyint(1) DEFAULT '1' COMMENT '是否启用',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tenant_id` (`tenant_id`),
  KEY `idx_sub_type` (`sub_type`),
  KEY `idx_enabled` (`enabled`),
  KEY `idx_sort_order` (`sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `generation_voices` (
  `voice_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `language` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sample_url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  `recommend` tinyint(1) NOT NULL DEFAULT '0' COMMENT '精选推荐（口播常用音色）',
  `scope` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'vidu',
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `source_task_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `is_default` tinyint(1) NOT NULL DEFAULT '0' COMMENT '平台默认音色（scope=platform 内仅一条）',
  `vidu_registered_at` datetime(3) DEFAULT NULL COMMENT 'Vidu 侧注册/最近续期时间；NULL=未注册。同ID复注册幂等，过期前重注册即续期（窗口见 gen_vidu_voice_window）',
  PRIMARY KEY (`voice_id`),
  KEY `idx_voices_vidu_reg` (`vidu_registered_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_accounts` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `display_name` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `cookie_encrypted` text COLLATE utf8mb4_unicode_ci,
  `health` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT 'active',
  `bound_at` datetime(3) DEFAULT NULL,
  `last_used_at` datetime(3) DEFAULT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `login_method` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `auth_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'cookie' COMMENT '绑定方式：cookie（扫码浏览器）/ oauth（官方授权）',
  `access_token_enc` text COLLATE utf8mb4_unicode_ci COMMENT 'OAuth access_token 密文（AES-GCM）',
  `refresh_token_enc` text COLLATE utf8mb4_unicode_ci COMMENT 'OAuth refresh_token 密文（AES-GCM）',
  `open_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '平台用户唯一标识（OAuth 授权返回）',
  `refresh_expires_at` datetime(3) DEFAULT NULL COMMENT 'refresh_token 过期时间（OAuth 续期窗口管理）',
  `union_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '开放平台维度用户标识（跨应用稳定——三端账号打通用）',
  `role` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'merchant' COMMENT '账号角色：merchant（商户）/ platform（平台工作账号）',
  PRIMARY KEY (`id`),
  KEY `idx_geo_acc_tenant` (`tenant_id`),
  KEY `idx_geo_acc_platform` (`tenant_id`,`platform`),
  KEY `idx_geo_acc_expires` (`expires_at`),
  KEY `idx_geo_accounts_open_id` (`open_id`),
  KEY `idx_geo_accounts_refresh_expires` (`refresh_expires_at`),
  KEY `idx_geo_accounts_union_id` (`union_id`),
  KEY `idx_geo_accounts_role` (`role`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_ai_rank_probes` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `results` json DEFAULT NULL,
  `sample_count` int NOT NULL DEFAULT '0',
  `probed_at` datetime(3) NOT NULL,
  `expire_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ai_rank_brand` (`brand_id`,`probed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_brands` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `positioning` text COLLATE utf8mb4_unicode_ci,
  `core_selling` json DEFAULT NULL,
  `competitors` json DEFAULT NULL,
  `industry` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT '',
  `created_at` datetime(3) DEFAULT NULL,
  `biz_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'local',
  `website_url` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_geo_brands_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_keywords` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `term` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL,
  `intent` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_geo_keywords_tenant` (`tenant_id`),
  KEY `idx_geo_keywords_brand` (`brand_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_monitoring_results` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `keyword_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `engine_name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `sample_count` int NOT NULL DEFAULT '0',
  `mention_count` int NOT NULL DEFAULT '0',
  `mention_rate` decimal(4,3) NOT NULL DEFAULT '0.000',
  `avg_position` int NOT NULL DEFAULT '0',
  `sentiment` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `competitors` json DEFAULT NULL,
  `confidence` decimal(4,3) NOT NULL DEFAULT '0.000',
  `probed_at` datetime(3) DEFAULT NULL,
  `raw_sample` text COLLATE utf8mb4_unicode_ci,
  `sources` json DEFAULT NULL,
  `self_source_count` int NOT NULL DEFAULT '0',
  `competitor_rates` json DEFAULT NULL,
  `candidate_competitors` json DEFAULT NULL,
  `competitor_sentiments` json DEFAULT NULL,
  `first_pick_count` int NOT NULL DEFAULT '0',
  `semantic_degraded` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_geo_mr_tenant_brand` (`tenant_id`,`brand_id`),
  KEY `idx_geo_mr_keyword` (`keyword_id`),
  KEY `idx_geo_mr_probed` (`probed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_optimized_contents` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `title` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `keyword_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `original_text` longtext COLLATE utf8mb4_unicode_ci,
  `optimized_text` longtext COLLATE utf8mb4_unicode_ci,
  `version` int NOT NULL DEFAULT '1',
  `score_total` decimal(5,2) DEFAULT NULL,
  `authority` decimal(5,2) DEFAULT NULL,
  `specificity` decimal(5,2) DEFAULT NULL,
  `structure` decimal(5,2) DEFAULT NULL,
  `uniqueness` decimal(5,2) DEFAULT NULL,
  `recency` decimal(5,2) DEFAULT NULL,
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'draft',
  `index_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `indexed_at` datetime DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_geo_oc_tenant_brand` (`tenant_id`,`brand_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_publish_jobs` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `account_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `content_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `title` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` longtext COLLATE utf8mb4_unicode_ci,
  `mode` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT 'semi-auto',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT 'pending',
  `external_url` text COLLATE utf8mb4_unicode_ci,
  `error_msg` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `published_at` datetime(3) DEFAULT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `pre_mention_rate` decimal(5,2) DEFAULT '0.00',
  `post_mention_rate` decimal(5,2) DEFAULT '0.00',
  `scheduled_at` datetime DEFAULT NULL,
  `store_address` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT '',
  `content_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `media_urls_json` text COLLATE utf8mb4_unicode_ci,
  `cover_url` text COLLATE utf8mb4_unicode_ci,
  `transport` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '实际执行通道：link/rpa/api（空=历史数据）',
  `tags_json` text COLLATE utf8mb4_unicode_ci COMMENT '标签列表（JSON 数组）',
  `category` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '平台分区（B站投稿必选）',
  `privacy` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '可见性（youtube: public/unlisted/private；空=默认公开）',
  PRIMARY KEY (`id`),
  KEY `idx_geo_pj_tenant` (`tenant_id`),
  KEY `idx_publish_jobs_scheduled_at` (`scheduled_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `geo_store_locations` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `address` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL,
  `city` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `district` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `adcode` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lat` decimal(10,6) DEFAULT NULL,
  `lng` decimal(10,6) DEFAULT NULL,
  `phone` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `hours` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `price_level` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `biz_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'LocalBusiness',
  `geo_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `business_area` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT '',
  PRIMARY KEY (`id`),
  KEY `idx_geo_store_tenant` (`tenant_id`),
  KEY `idx_geo_store_brand` (`brand_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `hot_videos` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `title` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `hot_point` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `topic` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `cover_url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `author` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `play_count` bigint NOT NULL DEFAULT '0',
  `digg_count` bigint NOT NULL DEFAULT '0',
  `comment_count` bigint NOT NULL DEFAULT '0',
  `publish_time` datetime(3) DEFAULT NULL,
  `source` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'search',
  `created_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_brand_url` (`brand_id`,`url`(255)),
  KEY `idx_tenant_brand` (`tenant_id`,`brand_id`),
  KEY `idx_brand_platform` (`brand_id`,`platform`),
  KEY `idx_publish_time` (`publish_time`)
) ENGINE=InnoDB AUTO_INCREMENT=65 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `indexing_submit_logs` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `channel` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `error_msg` text COLLATE utf8mb4_unicode_ci,
  `submitted_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_submit_logs_channel` (`channel`),
  KEY `idx_submit_logs_status` (`status`),
  KEY `idx_submit_logs_time` (`submitted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `inspiration_videos` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `platform` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'douyin',
  `platform_video_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `title` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `description` text COLLATE utf8mb4_unicode_ci,
  `cover_url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `video_url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `author` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `author_avatar` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `duration` int NOT NULL DEFAULT '0',
  `publish_time` datetime(3) DEFAULT NULL,
  `play_count` bigint NOT NULL DEFAULT '0',
  `digg_count` bigint NOT NULL DEFAULT '0',
  `comment_count` bigint NOT NULL DEFAULT '0',
  `share_count` bigint NOT NULL DEFAULT '0',
  `collect_count` bigint NOT NULL DEFAULT '0',
  `topics` json DEFAULT NULL,
  `music_name` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `music_author` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sentiment` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'neutral',
  `viral_score` double NOT NULL DEFAULT '0',
  `is_pinned` tinyint(1) NOT NULL DEFAULT '0',
  `is_recommended` tinyint(1) NOT NULL DEFAULT '0',
  `admin_note` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `last_refreshed_at` datetime(3) DEFAULT NULL,
  `local_video_url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '本地转存地址（空=未转存回落原始链接）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_platform_video` (`platform`,`platform_video_id`),
  KEY `idx_viral_score` (`viral_score` DESC),
  KEY `idx_publish_time` (`publish_time` DESC),
  KEY `idx_play_count` (`play_count` DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `integration_capabilities` (
  `id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `cap_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `vendor_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `endpoint` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `model` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `is_default` tinyint(1) NOT NULL DEFAULT '0',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `extra_json` text COLLATE utf8mb4_unicode_ci,
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_cap_id` (`cap_id`),
  KEY `idx_vendor_id` (`vendor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `integration_vendors` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `base_url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `api_key` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `protocol` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'openai',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `updated_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `interview_questions` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `job_post_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `title` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `answer` longtext COLLATE utf8mb4_unicode_ci,
  `difficulty` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `tags` json DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_questions_job_post` (`job_post_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `job_posts` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `company` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `position` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `requirements` json DEFAULT NULL,
  `salary` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `raw_html` longtext COLLATE utf8mb4_unicode_ci,
  `url` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `fingerprint` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `collected_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `fingerprint` (`fingerprint`),
  KEY `idx_job_posts_source` (`source`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `kb_materials` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `industry` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL,
  `url_fingerprint` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `title` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` mediumtext COLLATE utf8mb4_unicode_ci,
  `summary` text COLLATE utf8mb4_unicode_ci,
  `tags` json DEFAULT NULL,
  `crawl_keyword` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `embedding` json DEFAULT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `created_at` datetime(3) DEFAULT NULL,
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '品牌私有素材归属（空=行业公共池）',
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '租户隔离（品牌私有素材必填）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_kb_fingerprint` (`url_fingerprint`),
  KEY `idx_kb_industry` (`industry`),
  KEY `idx_kb_created` (`created_at`),
  KEY `idx_kb_materials_brand` (`brand_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `knowledge` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `title` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` longtext COLLATE utf8mb4_unicode_ci,
  `summary` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `tags` json DEFAULT NULL,
  `source_url` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `vector_ref` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `llm_configs` (
  `name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `provider` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `api_key` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `base_url` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `model` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `cost_per_mtok` int NOT NULL DEFAULT '100',
  `usage` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用途标签：空=聊天模型，vision=视觉模型',
  `is_default` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`name`),
  KEY `idx_llm_configs_usage_col` (`usage`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `messages` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `conversation_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `role` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content` longtext COLLATE utf8mb4_unicode_ci,
  `tool_calls` longtext COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_messages_conversation` (`conversation_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `notifications` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'system',
  `title` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `content` text COLLATE utf8mb4_unicode_ci,
  `link` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `is_read` tinyint NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_notifications_tenant` (`tenant_id`),
  KEY `idx_notifications_read` (`is_read`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `orders` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `plan_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `amount_cents` int NOT NULL DEFAULT '0',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `payment_gateway` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `payment_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  `paid_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_orders_tenant` (`tenant_id`),
  KEY `idx_orders_status` (`status`),
  KEY `idx_orders_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `plans` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `level` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `price_cents` int NOT NULL DEFAULT '0',
  `quotas` text COLLATE utf8mb4_unicode_ci,
  `features` text COLLATE utf8mb4_unicode_ci,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `sort_order` int NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_plans_level` (`level`),
  KEY `idx_plans_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `platform_content_configs` (
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `config_json` json NOT NULL,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`platform`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `prompt_templates` (
  `key` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `version` int NOT NULL DEFAULT '1',
  `content` longtext COLLATE utf8mb4_unicode_ci,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `provider_configs` (
  `provider` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `api_key` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `base_url` varchar(256) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `extra_json` text COLLATE utf8mb4_unicode_ci,
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`provider`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `publish_records` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `content_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `content_type` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `success` tinyint(1) DEFAULT NULL,
  `external_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `error_msg` text COLLATE utf8mb4_unicode_ci,
  `result_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_publish_dedup` (`content_id`,`content_type`,`platform`),
  KEY `idx_publish_success` (`success`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `publish_usage_stats` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '租户ID',
  `brand_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '品牌ID',
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '平台标识',
  `publish_date` date NOT NULL COMMENT '发布日期',
  `usage_count` int DEFAULT '0' COMMENT '当日发布次数',
  `last_publish_at` datetime DEFAULT NULL COMMENT '最近一次发布时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_brand_platform_date` (`tenant_id`,`brand_id`,`platform`,`publish_date`),
  KEY `idx_tenant_brand` (`tenant_id`,`brand_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `subject_assets` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `scope` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'personal',
  `kind` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'person',
  `name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `server_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `portrait_url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `avatar_video_url` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '链式形象视频产物（Vidu签名URL可达~900字符）',
  `voice_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `tags` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sort_order` int NOT NULL DEFAULT '0',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `source_task_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_server` (`server_id`),
  KEY `idx_tenant_scope` (`tenant_id`,`scope`,`kind`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `subscriptions` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `plan_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `period_start` datetime NOT NULL,
  `period_end` datetime NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_subscriptions_tenant` (`tenant_id`),
  KEY `idx_subscriptions_plan` (`plan_id`),
  KEY `idx_subscriptions_period_end` (`period_end`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `system_settings` (
  `setting_key` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `value` longtext COLLATE utf8mb4_unicode_ci,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`setting_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tasks` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `type` varchar(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `input` text COLLATE utf8mb4_unicode_ci,
  `output` text COLLATE utf8mb4_unicode_ci,
  `progress` text COLLATE utf8mb4_unicode_ci,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `error` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tasks_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `tenant_settings` (
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `setting_key` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `value` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`tenant_id`,`setting_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `usages` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `user_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `scene` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `llm_config_name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `model` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `prompt_tokens` int NOT NULL DEFAULT '0',
  `completion_tokens` int NOT NULL DEFAULT '0',
  `total_tokens` int NOT NULL DEFAULT '0',
  `llm_calls` int NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_usages_tenant` (`tenant_id`),
  KEY `idx_usages_feature` (`scene`),
  KEY `idx_usages_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `video_metrics` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'vm-{nano}',
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `job_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '发布任务 ID',
  `platform` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `video_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '平台内视频 ID（aweme_id）',
  `views` bigint NOT NULL DEFAULT '0' COMMENT '播放',
  `likes` bigint NOT NULL DEFAULT '0' COMMENT '点赞',
  `comments` bigint NOT NULL DEFAULT '0' COMMENT '评论',
  `shares` bigint NOT NULL DEFAULT '0' COMMENT '分享',
  `collected_at` datetime(3) NOT NULL COMMENT '采集时间',
  PRIMARY KEY (`id`),
  KEY `idx_video_metrics_tenant` (`tenant_id`),
  KEY `idx_video_metrics_job_time` (`job_id`,`collected_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='视频互动数据快照（每日回读时间序列）';
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `webreaper_schema_migrations` (
  `version` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `applied_at` datetime(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `webreaper_users` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `username` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `password_hash` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `role` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'admin',
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_webreaper_users_username` (`username`),
  KEY `idx_webreaper_users_role` (`role`),
  KEY `idx_webreaper_users_tenant` (`tenant_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `work_moderations` (
  `id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `work_key` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `work_kind` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'video',
  `tenant_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `action` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'hidden',
  `reason` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `source` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'admin' COMMENT 'admin(人工处置)/machine(机审标记)',
  `appeal_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'none' COMMENT 'none/pending/accepted/rejected',
  `appeal_text` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户申诉理由',
  `appealed_at` datetime(3) DEFAULT NULL COMMENT '最近一次申诉时间（防滥用限频）',
  `operator` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_work_key` (`work_key`),
  KEY `idx_wm_tenant` (`tenant_id`),
  KEY `idx_wm_updated` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

-- §2 生产种子数据
-- 生产套餐（free/pro/team——剔除 plan-linktest 测试档）
INSERT  IGNORE INTO `plans` VALUES ('plan-free','免费版','free',0,'{\"chat\":100,\"content-gen\":5,\"content-opt\":10,\"diagnose\":10,\"generation\":10,\"keyword-distill\":30,\"monitor\":500,\"nearby\":30}','[]','active',1,'2026-08-11 14:15:37','2026-08-20 14:42:49'),('plan-pro','专业版','pro',29900,'{\"chat\":2000,\"content-gen\":50,\"content-opt\":100,\"diagnose\":80,\"generation\":200,\"keyword-distill\":300,\"monitor\":8000,\"nearby\":300}','[\"auto-monitor\",\"scheduled-publish\",\"index-verify\"]','active',2,'2026-08-10 23:29:47','2026-08-20 14:42:49'),('plan-team','团队版','team',89900,'{\"chat\":-1,\"content-gen\":-1,\"content-opt\":-1,\"diagnose\":-1,\"generation\":-1,\"keyword-distill\":-1,\"monitor\":-1,\"nearby\":-1}','[\"auto-monitor\",\"scheduled-publish\",\"index-verify\",\"video\",\"multi-account\",\"rag-enhance\"]','active',3,'2026-08-10 23:29:47','2026-08-20 14:42:49');

-- 管理员账号（admin 由服务端 seed 兜底——此处导出含 alice 共 2 个 admin）
INSERT  IGNORE INTO `webreaper_users` VALUES ('user-1783191813452160400','alice','$2a$10$j/tAunByLyb1.unGCzYXYOZNw9MOmWxlliM9j1darzALxIoHvPFOW','2026-07-05 03:03:33.452','2026-08-10 17:12:21.408','admin','tenant-user-1783191813452160400'),('user-1783606014397225300','admin','$2a$10$wuLLA9MrAoxERWMTx/Q7BelywlNDxmNVWc.Dfb466O46g8teD8Y6C','2026-07-09 22:06:54.397','2026-08-10 16:57:57.606','admin','tenant-user-1783606014397225300');

-- §3 平台配置种子（音色库/规格/模板/提示词/LLM/集成/系统设置——从已过滤种子文件取）
-- WebReaper 基线种子数据（结构见 001_schema.sql）
-- 只含生产基线：admin 账号 + 3 套餐 + 平台配置 + 302 条 Vidu 音色
-- 剔除所有测试用户/测试套餐/测试租户

INSERT  IGNORE INTO `platform_content_configs` VALUES ('bilibili','{\"allow_emoji\": true, \"require_cta\": false, \"default_tags\": [\"#bilibili\"], \"cta_templates\": [], \"emoji_density\": 0.1, \"max_new_lines\": 8, \"max_tag_count\": 3, \"max_title_length\": 50, \"max_description_length\": 5000}','2026-08-31 11:25:40'),('douyin','{\"allow_emoji\": true, \"require_cta\": true, \"default_tags\": [\"#推荐\", \"#种草\", \"#好物\"], \"cta_templates\": [\"觉得有用点个赞吧\", \"喜欢的话关注我\", \"评论区见\"], \"emoji_density\": 0.05, \"max_new_lines\": 5, \"max_tag_count\": 3, \"max_title_length\": 30, \"max_description_length\": 2000}','2026-08-31 11:25:40'),('kuaishou','{\"allow_emoji\": false, \"require_cta\": true, \"default_tags\": [\"#推荐\", \"#好物\"], \"cta_templates\": [\"觉得有用点个赞吧\", \"喜欢的话关注我\"], \"emoji_density\": 0, \"max_new_lines\": 3, \"max_tag_count\": 2, \"max_title_length\": 20, \"max_description_length\": 1500}','2026-08-31 11:25:40'),('weixin','{\"allow_emoji\": false, \"require_cta\": true, \"default_tags\": [], \"cta_templates\": [\"觉得有用点个赞吧\", \"喜欢的话关注我\"], \"emoji_density\": 0, \"max_new_lines\": 0, \"max_tag_count\": 0, \"max_title_length\": 16, \"max_description_length\": 50000}','2026-08-31 11:25:40'),('xiaohongshu','{\"allow_emoji\": true, \"require_cta\": false, \"default_tags\": [\"#推荐\", \"#种草\", \"#好物\"], \"cta_templates\": [], \"emoji_density\": 0.15, \"max_new_lines\": 10, \"max_tag_count\": 0, \"max_title_length\": 20, \"max_description_length\": 1000}','2026-08-31 11:25:40');

INSERT  IGNORE INTO `integration_vendors` VALUES ('minimax','minimax','https://api.minimaxi.com/v1','sk-cp-VOA5idIe2xD6rvPwaVHYgXZKptd_ZXemX8x82slEnWtPlrvKIflA2GJXVqbZtWkzd8MilU8p3vEJsBWmVOKe8hsX__dOussHNZcr1gjqNqQVnkjWuirbjfM','openai',1,'2026-07-05 13:03:35.053'),('openai','OpenAI','https://api.openai.com/v1','','openai',1,'2026-08-23 03:48:31.385'),('other','other','https://apihub.agnes-ai.com/v1/','sk-DqYW9kRAhs6bkNXI3fd3wmMZklVtT6xvN35UdHXhp2wO4H7G','openai',1,'2026-07-06 22:46:21.931'),('siliconflow','硅基流动','https://api.siliconflow.cn/v1','','openai',1,'2026-08-23 03:48:31.294'),('vidu','Vidu','','vda_979105682777182208_0EY069HMBm0wN8ROfYqeVozTsTcKnmW2','vidu',1,'2026-08-26 15:46:28.557'),('xiaomi-mimo','小米MiMo','https://token-plan-cn.xiaomimimo.com/v1','tp-c55wr623lqrqnh4w77lehfrr1695jzkwmew28jsd3tc9ddb6','openai-chat',1,'2026-08-23 15:41:11.389');

INSERT  IGNORE INTO `integration_capabilities` VALUES ('asr#siliconflow','asr','siliconflow','/audio/transcriptions','FunAudioLLM/SenseVoiceSmall',0,1,'','2026-08-23 05:01:57.510'),('asr#xiaomi-mimo','asr','xiaomi-mimo','/chat/completions','mimo-v2.5-asr',1,1,'{\"response_style\":\"chat\",\"asr_options_language\":\"auto\"}','2026-08-23 05:01:57.514'),('audio#vidu','audio','vidu','','',1,1,'','2026-08-26 17:14:13.392'),('digital-human#vidu','digital-human','vidu','','',1,1,'','2026-08-26 17:14:13.069'),('image#vidu','image','vidu','','',1,1,'','2026-08-26 17:14:12.791'),('llm-chat#minimax','llm-chat','minimax','https://api.minimaxi.com/v1/chat/completions','MiniMax-M2.5',0,1,'{}','2026-08-23 05:51:44.862'),('llm-chat#other','llm-chat','other','https://apihub.agnes-ai.com/v1//chat/completions','agnes-2.5-flash',1,1,'{}','2026-08-23 05:51:44.866'),('llm-chat#xiaomi-mimo','llm-chat','xiaomi-mimo','/chat/completions','mimo-v2.5-pro',0,1,'','2026-08-23 03:48:31.678'),('llm#xiaomi-mimo','llm','xiaomi-mimo','/v1/chat/completions','mimo-v2.5-pro',0,1,NULL,'2026-08-23 15:41:11.000'),('tts#xiaomi-mimo','tts','xiaomi-mimo','/chat/completions','mimo-v2.5-tts',1,1,'{\"response_style\":\"chat\",\"audio_format\":\"wav\"}','2026-08-23 05:02:34.931'),('video#vidu','video','vidu','','',1,1,'','2026-08-26 17:14:12.510'),('voice-clone#xiaomi-mimo','voice-clone','xiaomi-mimo','/v1/chat/completions','mimo-v2.5-tts-voiceclone',1,1,NULL,'2026-08-23 15:47:12.754');

INSERT  IGNORE INTO `generation_specs` VALUES ('digital_human','vidu','viduq2-pro','/ent/v2/digital-human',1,1,0,'{\"model\": \"viduq2-pro\", \"family\": \"q2\", \"durations\": [0, 0], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 12:37:02.914',0),('digital_human','vidu','viduq2-turbo','/ent/v2/digital-human',1,0,0,'{\"model\": \"viduq2-turbo\", \"family\": \"q2\", \"durations\": [0, 0], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:42.200',0),('img2video','vidu','vidu2.0','/ent/v2/img2video',1,1,0,'{\"model\": \"vidu2.0\", \"family\": \"vidu2.0\", \"durations\": [4, 8], \"image_slots\": 1, \"resolutions\": [\"360p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-23 12:37:02.914',0),('img2video','vidu','viduq1','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq1\", \"family\": \"q1\", \"durations\": [5, 5], \"image_slots\": 1, \"resolutions\": [\"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-12 16:16:43.360',0),('img2video','vidu','viduq1-classic','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq1-classic\", \"family\": \"q1\", \"durations\": [5, 5], \"image_slots\": 1, \"resolutions\": [\"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-12 16:16:43.456',0),('img2video','vidu','viduq2-pro','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq2-pro\", \"family\": \"q2\", \"durations\": [1, 10], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.070',0),('img2video','vidu','viduq2-pro-fast','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq2-pro-fast\", \"family\": \"q2\", \"durations\": [1, 10], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.167',0),('img2video','vidu','viduq2-turbo','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq2-turbo\", \"family\": \"q2\", \"durations\": [1, 10], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.263',0),('img2video','vidu','viduq3-pro','/ent/v2/img2video',1,1,0,'{\"model\": \"viduq3-pro\", \"family\": \"q3\", \"durations\": [1, 16], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 16:32:45.070',0),('img2video','vidu','viduq3-pro-fast','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq3-pro-fast\", \"family\": \"q3\", \"durations\": [1, 16], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:42.975',0),('img2video','vidu','viduq3-turbo','/ent/v2/img2video',1,0,0,'{\"model\": \"viduq3-turbo\", \"family\": \"q3\", \"durations\": [1, 16], \"image_slots\": 1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:42.878',0),('multiframe','vidu','viduq2-pro','/ent/v2/multiframe',1,1,0,'{\"model\": \"viduq2-pro\", \"family\": \"q2\", \"durations\": [2, 7], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 12:37:02.914',0),('multiframe','vidu','viduq2-turbo','/ent/v2/multiframe',1,0,0,'{\"model\": \"viduq2-turbo\", \"family\": \"q2\", \"durations\": [2, 7], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:42.103',0),('reference2video','vidu','vidu2.0','/ent/v2/reference2video',1,1,0,'{\"model\": \"vidu2.0\", \"family\": \"vidu2.0\", \"durations\": [4, 4], \"image_slots\": -1, \"resolutions\": [\"360p\", \"720p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-23 12:37:02.914',0),('reference2video','vidu','viduq1','/ent/v2/reference2video',1,0,0,'{\"model\": \"viduq1\", \"family\": \"q1\", \"durations\": [5, 5], \"image_slots\": -1, \"resolutions\": [\"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-12 16:16:44.883',0),('reference2video','vidu','viduq2','/ent/v2/reference2video',1,0,0,'{\"model\": \"viduq2\", \"family\": \"q2\", \"durations\": [1, 10], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:44.786',0),('reference2video','vidu','viduq2-pro','/ent/v2/reference2video',1,0,0,'{\"model\": \"viduq2-pro\", \"family\": \"q2\", \"durations\": [0, 10], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 2, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": true}','2026-08-12 16:16:44.690',0),('reference2video','vidu','viduq3','/ent/v2/reference2video',1,0,0,'{\"model\": \"viduq3\", \"family\": \"q3\", \"durations\": [3, 16], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": true, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": true}','2026-08-12 16:16:44.499',0),('reference2video','vidu','viduq3-mix','/ent/v2/reference2video',1,0,0,'{\"model\": \"viduq3-mix\", \"family\": \"q3\", \"durations\": [3, 16], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": true}','2026-08-12 16:16:44.596',0),('reference2video','vidu','viduq3-turbo','/ent/v2/reference2video',1,0,0,'{\"model\": \"viduq3-turbo\", \"family\": \"q3\", \"durations\": [3, 16], \"image_slots\": -1, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"4:3\", \"3:4\", \"1:1\"], \"audio_default\": true, \"max_prompt_len\": 2000, \"supports_movement\": false, \"supports_subjects\": true}','2026-08-12 16:16:44.404',0),('start_end2video','vidu','vidu2.0','/ent/v2/start-end2video',1,1,0,'{\"model\": \"vidu2.0\", \"family\": \"vidu2.0\", \"durations\": [4, 8], \"image_slots\": 2, \"resolutions\": [\"360p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-23 12:37:02.914',0),('start_end2video','vidu','viduq1','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq1\", \"family\": \"q1\", \"durations\": [5, 5], \"image_slots\": 2, \"resolutions\": [\"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-12 16:16:44.119',0),('start_end2video','vidu','viduq1-classic','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq1-classic\", \"family\": \"q1\", \"durations\": [5, 5], \"image_slots\": 2, \"resolutions\": [\"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 0, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-12 16:16:44.215',0),('start_end2video','vidu','viduq2-pro','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq2-pro\", \"family\": \"q2\", \"durations\": [1, 8], \"image_slots\": 2, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.838',0),('start_end2video','vidu','viduq2-pro-fast','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq2-pro-fast\", \"family\": \"q2\", \"durations\": [1, 8], \"image_slots\": 2, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.931',0),('start_end2video','vidu','viduq2-turbo','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq2-turbo\", \"family\": \"q2\", \"durations\": [1, 8], \"image_slots\": 2, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:44.027',0),('start_end2video','vidu','viduq3-pro','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq3-pro\", \"family\": \"q3\", \"durations\": [1, 16], \"image_slots\": 2, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.646',0),('start_end2video','vidu','viduq3-turbo','/ent/v2/start-end2video',1,0,0,'{\"model\": \"viduq3-turbo\", \"family\": \"q3\", \"durations\": [1, 16], \"image_slots\": 2, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-12 16:16:43.743',0),('text2video','','viduq1','/ent/v2/text2video',1,0,0,'{\"model\": \"viduq1\", \"family\": \"q1\", \"durations\": [5, 5], \"image_slots\": 0, \"resolutions\": [\"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"1:1\"], \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": true, \"supports_subjects\": false}','2026-08-23 12:51:57.728',0),('text2video','','viduq2','/ent/v2/text2video',1,0,0,'{\"model\": \"viduq2\", \"family\": \"q2\", \"durations\": [1, 6], \"image_slots\": 0, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 12:51:57.781',0),('text2video','vidu','viduq3-pro','/ent/v2/text2video',1,1,0,'{\"model\": \"viduq3-pro\", \"family\": \"q3\", \"durations\": [1, 16], \"audio_types\": [\"all\", \"speech_only\", \"sound_effect_only\"], \"image_slots\": 0, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"3:4\", \"4:3\", \"1:1\"], \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 16:32:44.898',0),('text2video','','viduq3-turbo','/ent/v2/text2video',1,0,0,'{\"model\": \"viduq3-turbo\", \"family\": \"q3\", \"durations\": [1, 16], \"audio_types\": [\"all\", \"speech_only\", \"sound_effect_only\"], \"image_slots\": 0, \"resolutions\": [\"540p\", \"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"aspect_ratios\": [\"16:9\", \"9:16\", \"3:4\", \"4:3\", \"1:1\"], \"audio_default\": true, \"max_prompt_len\": 5000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 12:51:57.882',0),('text2video','','viduq4-pro','/ent/v2/text2video',1,0,0,'{\"model\": \"viduq4-pro\", \"family\": \"q4\", \"durations\": [1, 20], \"image_slots\": 0, \"resolutions\": [\"720p\", \"1080p\"], \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 6000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 12:51:57.935',0),('tts','xiaomi-mimo','default','/ent/v2/audio-tts',1,1,0,'{\"model\": \"default\", \"family\": \"tts\", \"durations\": [0, 0], \"image_slots\": 0, \"video_slots\": 0, \"supports_bgm\": false, \"audio_default\": false, \"max_prompt_len\": 10000, \"supports_movement\": false, \"supports_subjects\": false}','2026-08-23 17:04:06.285',0);

INSERT  IGNORE INTO `generation_templates` VALUES ('brand_promo','','品牌宣传视频','4秒品牌Logo动画视频，适合社交媒体宣传','🎬','img2video','{\"duration\": 4, \"resolution\": \"720p\"}','[\"image\"]','[]',1,1,'2026-08-23 04:35:39','2026-08-23 04:35:39'),('digital_human','','数字人口播','数字人口播视频，适合产品介绍、客服回复','🤖','digital_human','{\"resolution\": \"720p\"}','[\"image\"]','[\"audio\"]',3,1,'2026-08-23 04:35:39','2026-08-23 04:35:39'),('lip_sync','','对口型视频','真人出镜对口型视频，适合口播内容','🎤','lip_sync','{}','[\"video\"]','[\"audio\"]',4,1,'2026-08-23 04:35:39','2026-08-23 04:35:39'),('product_intro','','产品介绍视频','8秒产品展示视频，详细展示产品特点','📦','text2video','{\"duration\": 8, \"resolution\": \"720p\"}','[]','[\"image\"]',2,1,'2026-08-23 04:35:39','2026-08-23 04:35:39'),('tts_audio','','语音合成','文本转语音，适合旁白、解说','🔊','tts','{\"voice_setting_voice_id\": \"default\"}','[]','[]',5,1,'2026-08-23 04:35:39','2026-08-23 04:35:39');

INSERT  IGNORE INTO `prompt_templates` VALUES ('content-generate',3,'你是一个 GEO（生成式引擎优化）内容创作专家。\n目标：根据品牌信息和关键词，创作一篇容易被 AI 搜索引擎引用的高质量文章。\n要求：\n1. 围绕关键词展开，自然融入（不堆砌）\n2. 结构化：标题层级清晰、有列表/小标题\n3. 有权威性：包含具体观点、方法论、可操作建议\n4. 真实可信：不编造数据，基于品牌真实信息创作\n5. 字数 800-1500 字\n\n硬性要求：\n- 严禁输出任何思考过程、推理过程或 <think> 内容——只输出最终文章\n- 第一行必须输出标题，以 # 开头（如 # 北京装修公司哪家好？10 年老牌真实对比）\n- 只输出文章正文（含标题），不要解释','2026-08-10 23:30:15'),('content-optimize',1,'你是一个 GEO（生成式内容优化）专家。\n目标：把给定内容优化得更可能被 AI 搜索引擎引用。\n优化方向：\n1. 增强权威性：补充具体数据、案例、资质（不可编造）\n2. 增强具体性：用数字、细节、可验证信息替代模糊表述\n3. 结构化：使用标题层级、列表、FAQ 格式\n4. 自然融入关键词：避免堆砌\n5. 保持真实性：绝不编造虚假信息\n\n硬性要求：\n- 严禁输出任何思考过程、推理过程或 <think> 内容——只输出最终文章\n- 第一行必须输出标题，以 # 开头（如 # 北京装修公司哪家好？10 年老牌真实对比）\n- 只输出优化后的内容，不要解释','2026-08-10 21:45:56'),('geo_format_article',1,'输出格式为 SEO 优化文章：标题层级清晰（H1/H2/H3），800-1500 字，结构化列表+小标题，关键词自然分布。适合发布到官网/博客/知乎专栏。','2026-08-12 02:35:01'),('geo_format_citation',1,'输出格式为 AI 高引用友好结构（GEO 可引用素材）：① 结论前置——开头 1-2 句直接给出核心答案（AI 摘录时优先取结论段）；② 每个观点独立成段，段首用小标题（H3）概括该段要点；③ 关键数据必须标注来源（如「据 2025 年行业报告」），可被 AI 验证；④ 包含 2-3 个 FAQ 式问答对（问题即用户搜索句，答案 50-80 字）；⑤ 全文 600-1200 字，段落短（≤4 行），无营销套话。目标：让 AI 引擎在生成回答时最容易摘录、引用并标注你为信源。','2026-08-15 16:53:58'),('geo_format_comparison',1,'输出格式为产品/服务对比评测：表格对比+文字分析，覆盖功能/价格/优缺点/适用场景，客观中立带推荐结论。适合选购决策类内容。','2026-08-12 02:35:01'),('geo_format_faq',1,'输出格式为 FAQ 问答集：5-8 个常见问题+简明回答（每答 50-100 字），覆盖用户搜索意图（是什么/怎么选/多少钱/注意事项）。适合官网帮助中心/知乎。','2026-08-12 02:35:01'),('geo_format_review',1,'输出格式为大众点评/美团风格的探店点评：200-400 字，第一人称体验感，包含环境/口味/服务/性价比评价，结尾带推荐语。口语化、真实感强，适合发到点评平台。','2026-08-12 02:35:01'),('geo_format_script',1,'输出格式为短视频口播脚本：开头 3 秒 hook（制造好奇/痛点），中间痛点+解决方案（200-400 字口播），结尾行动号召（关注/到店/咨询）。标注 [镜头提示] 方便拍摄。适合抖音/视频号。','2026-08-12 02:35:01'),('geo_format_xiaohongshu',1,'输出格式为小红书种草笔记：标题吸睛（带 emoji），300-500 字，分段短句，多 emoji 点缀（✨😋📍💡等），文末带 3-5 个 #话题标签。语气亲切、有画面感。','2026-08-12 02:35:01');

INSERT  IGNORE INTO `llm_configs` VALUES ('Agnes-2.0-Flash','other','sk-DqYW9kRAhs6bkNXI3fd3wmMZklVtT6xvN35UdHXhp2wO4H7G','https://apihub.agnes-ai.com/v1/','agnes-2.0-flash','2026-07-06 22:46:21.991','2026-07-06 22:46:21.931',100,'',0),('agnes-vision','other','sk-DqYW9kRAhs6bkNXI3fd3wmMZklVtT6xvN35UdHXhp2wO4H7G','https://apihub.agnes-ai.cn/v1','agnes-2.5-flash','2026-08-20 17:32:12.841','2026-08-20 17:32:12.782',1,'',0),('default','minimax','sk-cp-VOA5idIe2xD6rvPwaVHYgXZKptd_ZXemX8x82slEnWtPlrvKIflA2GJXVqbZtWkzd8MilU8p3vEJsBWmVOKe8hsX__dOussHNZcr1gjqNqQVnkjWuirbjfM','https://api.minimaxi.com/v1','MiniMax-M2.5','2026-07-05 13:03:35.102','2026-07-05 13:03:35.053',100,'',0),('minimax-m1','minimax','sk-cp-VOA5idIe2xD6rvPwaVHYgXZKptd_ZXemX8x82slEnWtPlrvKIflA2GJXVqbZtWkzd8MilU8p3vEJsBWmVOKe8hsX__dOussHNZcr1gjqNqQVnkjWuirbjfM','https://api.minimaxi.com/v1','MiniMax-M1','2026-08-11 22:46:24.620','2026-08-11 22:46:24.569',60,'',0),('xiaomi-mimo','xiaomi-mimo','tp-c55wr623lqrqnh4w77lehfrr1695jzkwmew28jsd3tc9ddb6','https://token-plan-cn.xiaomimimo.com/v1','mimo-v2.5-pro','0000-00-00 00:00:00.000','2026-08-23 16:54:32.590',100,'',0);

INSERT  IGNORE INTO `provider_configs` VALUES ('vidu','vda_979105682777182208_0EY069HMBm0wN8ROfYqeVozTsTcKnmW2','',1,'','2026-08-26 18:03:00.683');

INSERT  IGNORE INTO `system_settings` VALUES ('auto_monitor_enabled','true','2026-08-10 23:22:52.623'),('browser_headed','false','2026-09-01 23:19:26.585'),('crawl_policy','{\"request_interval_ms\":1000,\"request_timeout_ms\":30000,\"max_retries\":0,\"respect_robots\":false}','2026-07-12 13:12:29.279'),('gen_default_avatar_prompt','形象展示：正面特写，微笑看向镜头，姿态自然大方，缓慢自然的肢体动作','2026-09-02 00:03:00.251'),('gen_moderation_block','false','2026-09-01 22:21:32.561'),('gen_moderation_enabled','false','2026-09-01 22:21:32.610'),('indexing_config','{\"index_now_key\":\"87e387d4-234a-df88-b6e2-44c01fc0c499\",\"baidu_site\":\"\",\"baidu_token\":\"\",\"updated_at\":\"2026-08-10T14:17:17.9163637+08:00\"}','2026-08-11 01:54:18.435'),('kb_crawl_industries','[{\"industry\":\"美业\",\"keywords\":[\"美业拓客\",\"美容院运营\"],\"per_round\":5},{\"industry\":\"餐饮\",\"keywords\":[\"餐饮营销\"],\"per_round\":3}]','2026-08-14 20:50:25.676'),('kb_crawl_interval_minutes','360','2026-08-14 21:03:22.007'),('kb_embedding_config','{\"model\":\"embedding-3\",\"base_url\":\"https://open.bigmodel.cn/api/paas/v4\",\"api_key\":\"395cc02b7c1941a8b6f0637e406087b6.SrdutXmyk45FTcpp\",\"dimensions\":0,\"vector_db\":\"milvus\",\"milvus_host\":\"172.23.34.108\",\"milvus_port\":\"19530\",\"milvus_collection\":\"kb_materials\",\"updated_at\":\"2026-08-14T20:30:01.6075835+08:00\"}','2026-08-14 20:30:01.608'),('payment_config','{\"gateway\":\"mock\",\"key\":\"\",\"notify_url\":\"\",\"pid\":\"\",\"return_url\":\"\"}','2026-08-11 01:56:26.929'),('publish.transport_override','{}','2026-08-22 01:20:34.610');

INSERT  IGNORE INTO `tenant_settings` VALUES ('tenant-user-1783606014397225300','auto_monitor_config','{\"frequency\":\"weekly\",\"sample_size\":3,\"engine_name\":\"doubao\",\"notify_drop_threshold\":15,\"notify_overtake\":false}','2026-08-12 13:18:23'),('tenant-user-1783606014397225300','auto_monitor_enabled','true','2026-08-14 00:54:26'),('tenant-user-1783606014397225300','auto_monitor_last_run','2026-08-12T13:44:32+08:00','2026-08-12 13:44:32'),('tenant-user-1786453035559188100','auto_monitor_config','{\"frequency\":\"half_day\",\"sample_size\":3,\"engine_name\":\"\",\"notify_drop_threshold\":30,\"notify_overtake\":true}','2026-08-12 00:47:18'),('tenant-user-1786453035559188100','auto_monitor_enabled','true','2026-08-12 00:47:18');

INSERT  IGNORE INTO `generation_voices` VALUES ('Arabic_CalmWoman','阿拉伯文','Calm Woman','https://scene.vidu.zone/media-asset/035240-u12liEq8DtchndBf.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Arabic_FriendlyGuy','阿拉伯文','Friendly Guy','https://scene.vidu.zone/media-asset/035240-QHVeX8jxCAHCOM1x.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Arnold','英文','Arnold','https://scene.vidu.zone/media-asset/071753-AxmKuE7wxlY8zvNn.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Arrogant_Miss','中文 (普通话)','嚣张小姐','https://scene.vidu.zone/media-asset/072014-tN236iR4Y4UIYhtu.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Attractive_Girl','英文','Attractive Girl','https://scene.vidu.zone/media-asset/071753-bYCPNmnU1xcDQPtr.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('badao_shaoye','中文 (普通话)','霸道少爷','https://scene.vidu.zone/media-asset/072359-IIt6p1U9NxkMp1Y6.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('bingjiao_didi','中文 (普通话)','病娇弟弟','https://scene.vidu.zone/media-asset/072358-Rv9wMGYjHki3XE0W.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cantonese_CuteGirl','中文 (粤语)','可爱女孩','https://scene.vidu.zone/media-asset/072018-qVDF05yOHcXC2gdh.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cantonese_GentleLady','中文 (粤语)','温柔女声','https://scene.vidu.zone/media-asset/072017-IO6fp0FyZe6xaG49.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cantonese_KindWoman','中文 (粤语)','善良女声','https://scene.vidu.zone/media-asset/072018-8NUCoRQQ6ABIPwJn.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cantonese_PlayfulMan','中文 (粤语)','活泼男声','https://scene.vidu.zone/media-asset/072018-iaGnbP1ycuobRZ9C.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cantonese_ProfessionalHost（F)','中文 (粤语)','专业女主持','https://scene.vidu.zone/media-asset/072017-ynsFX3cldjtl62xi.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cantonese_ProfessionalHost（M)','中文 (粤语)','专业男主持','https://scene.vidu.zone/media-asset/072018-otv08JVnkSB0BLJz.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('cartoon_pig','中文 (普通话)','卡通猪小琪','https://scene.vidu.zone/media-asset/072358-TEMSh2Ofg8Veq7tQ.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Charming_Lady','英文','Charming Lady','https://scene.vidu.zone/media-asset/071753-8K2HCbrry1fMinoe.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Charming_Santa','英文','Charming Santa','https://scene.vidu.zone/media-asset/071753-hxdVnrUyH9XaPFoU.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Crisp_Girl','中文 (普通话)','清脆少女','https://scene.vidu.zone/media-asset/072017-Jj7rRcIUnwJMrD72.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Cute_Spirit','中文 (普通话)','憨憨萌兽','https://scene.vidu.zone/media-asset/072016-kETCNwyPyI1BCqbA.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Gentle_Senior','中文 (普通话)','温柔学姐','https://scene.vidu.zone/media-asset/072016-PaRQa8DijbAn6erR.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Gentle_Youth','中文 (普通话)','温润青年','https://scene.vidu.zone/media-asset/072016-SJM3A4MPLxkwTpPb.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Gentleman','中文 (普通话)','温润男声','https://scene.vidu.zone/media-asset/072015-ireMGTMsZwkcvnpK.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('Chinese (Mandarin)_HK_Flight_Attendant','中文 (普通话)','港普空姐','https://scene.vidu.zone/media-asset/072014-krsfdA78yBt7Xo60.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Humorous_Elder','中文 (普通话)','搞笑大爷','https://scene.vidu.zone/media-asset/072014-cE1nhz9ZGzIdBpzz.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Kind-hearted_Antie','中文 (普通话)','热心大婶','https://scene.vidu.zone/media-asset/072014-A1APFa473HjaBhVL.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Kind-hearted_Elder','中文 (普通话)','花甲奶奶','https://scene.vidu.zone/media-asset/072016-thIywqHMOxldDrHy.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Lyrical_Voice','中文 (普通话)','抒情男声','https://scene.vidu.zone/media-asset/072016-3ZBJxk5fdLq6RekB.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Male_Announcer','中文 (普通话)','播报男声','https://scene.vidu.zone/media-asset/072015-7roXgurt9k7hKzFE.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Mature_Woman','中文 (普通话)','傲娇御姐','https://scene.vidu.zone/media-asset/072014-YRHkapPGWTi6AMpF.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_News_Anchor','中文 (普通话)','新闻女声','https://scene.vidu.zone/media-asset/072014-HO8qN7HowLpDeSxk.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Pure-hearted_Boy','中文 (普通话)','清澈邻家弟弟','https://scene.vidu.zone/media-asset/072017-EA7Z2f1Ou5ZOfWNr.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Radio_Host','中文 (普通话)','电台男主播','https://scene.vidu.zone/media-asset/072016-R9fYJ99T9l4aGNGS.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Reliable_Executive','中文 (普通话)','沉稳高管','https://scene.vidu.zone/media-asset/072014-KyNan6ZbFWullcwA.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Sincere_Adult','中文 (普通话)','真诚青年','https://scene.vidu.zone/media-asset/072016-vzzOfzNfOWtNOyOM.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Soft_Girl','中文 (普通话)','软软女孩','https://scene.vidu.zone/media-asset/072017-zIeJmPyeMsvXTMPN.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Southern_Young_Man','中文 (普通话)','南方小哥','https://scene.vidu.zone/media-asset/072015-IBxrNvCcpPsftZPY.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Straightforward_Boy','中文 (普通话)','率真弟弟','https://scene.vidu.zone/media-asset/072016-qmBBLsW7U5IQd6LN.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Stubborn_Friend','中文 (普通话)','嘴硬竹马','https://scene.vidu.zone/media-asset/072017-x42a4pgimTBDxWpQ.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Sweet_Lady','中文 (普通话)','甜美女声','https://scene.vidu.zone/media-asset/072015-gPhnrlLLSAUkqlbb.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Unrestrained_Young_Man','中文 (普通话)','不羁青年','https://scene.vidu.zone/media-asset/072014-ErSH4gUWyXwoKm0N.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Warm_Bestie','中文 (普通话)','温暖闺蜜','https://scene.vidu.zone/media-asset/072015-gw8VczK4UgGq7PDT.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Warm_Girl','中文 (普通话)','温暖少女','https://scene.vidu.zone/media-asset/072016-YqsIYQx95ejnm60I.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Chinese (Mandarin)_Wise_Women','中文 (普通话)','阅历姐姐','https://scene.vidu.zone/media-asset/072015-cAA4pkMzycpyJnS7.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('chunzhen_xuedi','中文 (普通话)','纯真学弟','https://scene.vidu.zone/media-asset/072358-1mna0aj1O43QFJu4.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('clever_boy','中文 (普通话)','聪明男童','https://scene.vidu.zone/media-asset/072357-iKVi4SXFzAD8hwr2.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('cute_boy','中文 (普通话)','可爱男童','https://scene.vidu.zone/media-asset/072358-74OupDa2hVtAiEC7.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Cute_Elf','英文','Cute Elf','https://scene.vidu.zone/media-asset/071753-DSs6yYFM7nhP3Y73.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('danya_xuejie','中文 (普通话)','淡雅学姐','https://scene.vidu.zone/media-asset/072359-z9HkOg1rpDOTcEF6.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('diadia_xuemei','中文 (普通话)','嗲嗲学妹','https://scene.vidu.zone/media-asset/072359-QKczPrBYdkh7cYnk.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Dutch_bossy_leader','荷兰文','Bossy leader','https://scene.vidu.zone/media-asset/034802-5PctlXu118SHucvd.mp3','2026-08-22 09:20:56.622',0,'vidu','','','active',0,NULL),('Dutch_kindhearted_girl','荷兰文','Kind-hearted girl','https://scene.vidu.zone/media-asset/034802-8UbeAa4QGbB6JaZx.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('English_Aussie_Bloke','英文','Aussie Bloke','https://scene.vidu.zone/media-asset/071754-LZE06DgGPNkTRemt.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('English_Diligent_Man','英文','Diligent Man','https://scene.vidu.zone/media-asset/071754-7VPK3SG8FT2Xuwl1.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('English_Gentle-voiced_man','英文','Gentle-voiced man','https://scene.vidu.zone/media-asset/071754-oWQQfWnzyzVKQRyq.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('English_Graceful_Lady','英文','Graceful Lady','https://scene.vidu.zone/media-asset/071753-Y6s3YATSib937ns2.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('English_Trustworthy_Man','英文','Trustworthy Man','https://scene.vidu.zone/media-asset/071753-4y46Ju6xokARFbnT.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('English_Whispering_girl','英文','Whispering girl','https://scene.vidu.zone/media-asset/071754-x4OKV2I685LLHZlY.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('female-chengshu','中文 (普通话)','成熟女性音色','https://scene.vidu.zone/media-asset/072356-HCbEW2grkWSOVYXB.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('female-chengshu-jingpin','中文 (普通话)','成熟女性音色-beta','https://scene.vidu.zone/media-asset/072357-BLBVdk5lacUOjTlv.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('female-shaonv','中文 (普通话)','少女音色','https://scene.vidu.zone/media-asset/072356-Zp3xnBAgMpkKN0fb.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('female-shaonv-jingpin','中文 (普通话)','少女音色-beta','https://scene.vidu.zone/media-asset/072357-76I89nNs8pI9KTnA.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('female-tianmei','中文 (普通话)','甜美女性音色','https://scene.vidu.zone/media-asset/072356-gc9rn0lig2Phdh1g.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('female-tianmei-jingpin','中文 (普通话)','甜美女性音色-beta','https://scene.vidu.zone/media-asset/072357-aatAGT8Yrj5VlLhB.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('female-yujie','中文 (普通话)','御姐音色','https://scene.vidu.zone/media-asset/072356-YiaFlpiWzo4Twkxb.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('female-yujie-jingpin','中文 (普通话)','御姐音色-beta','https://scene.vidu.zone/media-asset/072357-cnSquEUZyUTm0EM3.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('French_CasualMan','法文','Casual Man','https://scene.vidu.zone/media-asset/035756-UsNNaEJULdSii82b.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('French_Female_News Anchor','法文','Patient Female Presenter','https://scene.vidu.zone/media-asset/035756-yh2jpcYujGATdrnL.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('French_FemaleAnchor','法文','Female Anchor','https://scene.vidu.zone/media-asset/035756-0WMICpjG4qdi5xhp.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('French_Male_Speech_New','法文','Level-Headed Man','https://scene.vidu.zone/media-asset/035756-rxEBofqGwhif5qng.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('French_MaleNarrator','法文','Male Narrator','https://scene.vidu.zone/media-asset/035756-Lhz9XAfklUL4yrLr.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('French_MovieLeadFemale','法文','Movie Lead Female','https://scene.vidu.zone/media-asset/035756-o2mEO0ijNHbHGRwy.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('German_FriendlyMan','德文','Friendly Man','https://scene.vidu.zone/media-asset/035621-aCq5cdqQW43pB8ct.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('German_PlayfulMan','德文','Playful Man','https://scene.vidu.zone/media-asset/035621-E4Ucg8YwiueVTepF.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('German_SweetLady','德文','Sweet Lady','https://scene.vidu.zone/media-asset/035621-LuolmiKVVesy5hTY.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Grinch','英文','Grinch','https://scene.vidu.zone/media-asset/071752-jncRxGL1nuONbclk.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Indonesian_BossyLeader','印尼文','Bossy Leader','https://scene.vidu.zone/media-asset/035654-co5jmEYbQtFRmUqM.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_CalmWoman','印尼文','Calm Woman','https://scene.vidu.zone/media-asset/035653-oweTzCEudUlPnAFI.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_CaringMan','印尼文','Caring Man','https://scene.vidu.zone/media-asset/035653-5IfsR8TbYoQChgKz.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_CharmingGirl','印尼文','Charming Girl','https://scene.vidu.zone/media-asset/035653-ZIwlswQLA9qmTdU7.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_ConfidentWoman','印尼文','Confident Woman','https://scene.vidu.zone/media-asset/035653-czxnaGSlJ3ZWFDQV.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_DeterminedBoy','印尼文','Determined Boy','https://scene.vidu.zone/media-asset/035654-zBp6jTlUpfPPuU0G.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_GentleGirl','印尼文','Gentle Girl','https://scene.vidu.zone/media-asset/035654-IdUBETC49ObxAB4G.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_ReservedYoungMan','印尼文','Reserved Young Man','https://scene.vidu.zone/media-asset/035653-ohNuxDbeZc6uTtoU.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Indonesian_SweetGirl','印尼文','Sweet Girl','https://scene.vidu.zone/media-asset/035653-j1Y3keTo6DShfEzD.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Italian_BraveHeroine','意大利文','Brave Heroine','https://scene.vidu.zone/media-asset/035417-4BRNfRUWeCEqi1OV.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Italian_DiligentLeader','意大利文','Diligent Leader','https://scene.vidu.zone/media-asset/035417-BFDEwSWARMORQEpt.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Italian_Narrator','意大利文','Narrator','https://scene.vidu.zone/media-asset/035417-efHRj1jLnPYICek8.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Italian_WanderingSorcerer','意大利文','Wandering Sorcerer','https://scene.vidu.zone/media-asset/035417-eFDy6VsgM4XQrW6A.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Japanese_CalmLady','日文','Calm Lady','https://scene.vidu.zone/media-asset/071604-wlDxbRTLNaIEbZ5M.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_ColdQueen','日文','Cold Queen','https://scene.vidu.zone/media-asset/071603-J8Y40ENAYvWDSttc.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_DecisivePrincess','日文','Decisive Princess','https://scene.vidu.zone/media-asset/071603-2toFtXbWBYF4gMrJ.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_DependableWoman','日文','Dependable Woman','https://scene.vidu.zone/media-asset/071604-C9RsWj5tlZt36OWh.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_DominantMan','日文','Dominant Man','https://scene.vidu.zone/media-asset/071603-20uK33nB8EFTTBtl.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_GenerousIzakayaOwner','日文','Generous Izakaya Owner','https://scene.vidu.zone/media-asset/071604-tPEQvxWFuQERuC6Q.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_GentleButler','日文','Gentle Butler','https://scene.vidu.zone/media-asset/071604-lWhG00rvnTuPPFSo.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_GracefulMaiden','日文','Graceful Maiden','https://scene.vidu.zone/media-asset/071605-laBcWggCCv2p74YR.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_InnocentBoy','日文','Innocent Boy','https://scene.vidu.zone/media-asset/071605-NVkeELYpcAOUr9wD.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_IntellectualSenior','日文','Intellectual Senior','https://scene.vidu.zone/media-asset/071603-QRh9iySZHS09EzZp.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_KindLady','日文','Kind Lady','https://scene.vidu.zone/media-asset/071604-5jpras5x3poiOSGk.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_LoyalKnight','日文','Loyal Knight','https://scene.vidu.zone/media-asset/071603-4AhKlHBRmVSqzIgT.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_OptimisticYouth','日文','Optimistic Youth','https://scene.vidu.zone/media-asset/071604-vUsH8urJGa3tt7D8.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_SeriousCommander','日文','Serious Commander','https://scene.vidu.zone/media-asset/071603-yqm2eRGX6YN86wSZ.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Japanese_SportyStudent','日文','Sporty Student','https://scene.vidu.zone/media-asset/071605-IKwSl9VKF9rnf8S8.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('junlang_nanyou','中文 (普通话)','俊朗男友','https://scene.vidu.zone/media-asset/072358-apYWF4sTFAu4HT4n.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_AirheadedGirl','韩文','Airheaded Girl','https://scene.vidu.zone/media-asset/070804-ELTyvSCKZEjYl3s0.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_AthleticGirl','韩文','Athletic Girl','https://scene.vidu.zone/media-asset/070806-eH1gvEUmehqm6KAd.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_AthleticStudent','韩文','Athletic Student','https://scene.vidu.zone/media-asset/070803-XWYLsx3kwiZ3b9sS.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_BraveAdventurer','韩文','Brave Adventurer','https://scene.vidu.zone/media-asset/070803-U1hIhDdUyQpqoZpr.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_BraveFemaleWarrior','韩文','Brave Female Warrior','https://scene.vidu.zone/media-asset/070802-ergTgls4uLTyLOCT.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_BraveYouth','韩文','Brave Youth','https://scene.vidu.zone/media-asset/070802-R5En7boVtcqlcJRi.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CalmGentleman','韩文','Calm Gentleman','https://scene.vidu.zone/media-asset/070803-P5NbTLP8jrcp9LDN.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CalmLady','韩文','Calm Lady','https://scene.vidu.zone/media-asset/070802-085EUwwXLvy9nhUa.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CaringWoman','韩文','Caring Woman','https://scene.vidu.zone/media-asset/070805-9feq1GPW2I8TDjvf.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CharmingElderSister','韩文','Charming Elder Sister','https://scene.vidu.zone/media-asset/070805-sj7vyOyPyVWBGwbU.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CharmingSister','韩文','Charming Sister','https://scene.vidu.zone/media-asset/070803-LCF6CkbDlPSKVW14.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CheerfulBoyfriend','韩文','Cheerful Boyfriend','https://scene.vidu.zone/media-asset/070800-mjtmWUxmCKtJOLj5.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_CheerfulCoolJunior','韩文','Cheerful Cool Junior','https://scene.vidu.zone/media-asset/070803-AdzPeNndz7mVhLhQ.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CheerfulLittleSister','韩文','Cheerful Little Sister','https://scene.vidu.zone/media-asset/070804-hghwpVvHn2vYHNWO.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ChildhoodFriendGirl','韩文','Childhood Friend Girl','https://scene.vidu.zone/media-asset/070801-eorNHI2o4GX93Fz1.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_CockyGuy','韩文','Cocky Guy','https://scene.vidu.zone/media-asset/070806-MA182ManXRurk827.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ColdGirl','韩文','Cold Girl','https://scene.vidu.zone/media-asset/070805-9ZjegsY9vM93LevN.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ColdYoungMan','韩文','Cold Young Man','https://scene.vidu.zone/media-asset/070803-vocQv7J6ArLjp0TE.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ConfidentBoss','韩文','Confident Boss','https://scene.vidu.zone/media-asset/070806-QdCZt0Y6POMi00De.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ConsiderateSenior','韩文','Considerate Senior','https://scene.vidu.zone/media-asset/070804-CDvdlovW4I47sCOL.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_DecisiveQueen','韩文','Decisive Queen','https://scene.vidu.zone/media-asset/070803-FVdlsrLJZEHtXZ17.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_DominantMan','韩文','Dominant Man','https://scene.vidu.zone/media-asset/070804-yguQNBsKsSBMPam6.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ElegantPrincess','韩文','Elegant Princess','https://scene.vidu.zone/media-asset/070801-dCGGsWCXUTVj3ctt.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_EnchantingSister','韩文','Enchanting Sister','https://scene.vidu.zone/media-asset/070800-TuZar4xXGxG8BM7g.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_EnthusiasticTeen','韩文','Enthusiastic Teen','https://scene.vidu.zone/media-asset/070802-VaqXim4fv6IoNk7C.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_FriendlyBigSister','韩文','Friendly Big Sister','https://scene.vidu.zone/media-asset/070805-V5evipalTiDD9nvH.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_GentleBoss','韩文','Gentle Boss','https://scene.vidu.zone/media-asset/070805-kGckiiwRRzgO3RVA.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_GentleWoman','韩文','Gentle Woman','https://scene.vidu.zone/media-asset/070806-0PwqHf6TTcXHpgTk.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_HaughtyLady','韩文','Haughty Lady','https://scene.vidu.zone/media-asset/070805-NwKD9mhTxLaLx5D9.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_InnocentBoy','韩文','Innocent Boy','https://scene.vidu.zone/media-asset/070803-rAjCu3yh29pgal9V.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_IntellectualMan','韩文','Intellectual Man','https://scene.vidu.zone/media-asset/070805-HrwswOtNNHbf1WSL.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_IntellectualSenior','韩文','Intellectual Senior','https://scene.vidu.zone/media-asset/070802-njElfn743RgE2jvc.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_LonelyWarrior','韩文','Lonely Warrior','https://scene.vidu.zone/media-asset/070802-ov9mP4QiJIRENPLu.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_MatureLady','韩文','Mature Lady','https://scene.vidu.zone/media-asset/070802-yciPEHaaghSBWR7d.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_MysteriousGirl','韩文','Mysterious Girl','https://scene.vidu.zone/media-asset/070804-B8HBEX6r1e6e46kN.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_OptimisticYouth','韩文','Optimistic Youth','https://scene.vidu.zone/media-asset/070806-4tX7Kfu8PnSWJAzM.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_PlayboyCharmer','韩文','Playboy Charmer','https://scene.vidu.zone/media-asset/070801-QzWsVtLtNttkosnn.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_PossessiveMan','韩文','Possessive Man','https://scene.vidu.zone/media-asset/070806-F8xofvm8HZfePrc4.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_QuirkyGirl','韩文','Quirky Girl','https://scene.vidu.zone/media-asset/070804-54tAQsQkilJYlj5P.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ReliableSister','韩文','Reliable Sister','https://scene.vidu.zone/media-asset/070801-DUF2wBkKhIJvuuU4.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_ReliableYouth','韩文','Reliable Youth','https://scene.vidu.zone/media-asset/070805-FiWsHtV4TP7dzuvz.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_SassyGirl','韩文','Sassy Girl','https://scene.vidu.zone/media-asset/070801-BoJIHHU2CKAlj2wB.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_ShyGirl','韩文','Shy Girl','https://scene.vidu.zone/media-asset/070801-SSFgZHI4BKznJvWV.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_SoothingLady','韩文','Soothing Lady','https://scene.vidu.zone/media-asset/070802-Os0s7Fxt6Rvm09So.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_StrictBoss','韩文','Strict Boss','https://scene.vidu.zone/media-asset/070801-mGkkZFVCEfEQsOqM.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_SweetGirl','韩文','Sweet Girl','https://scene.vidu.zone/media-asset/070800-9UMQFeOvRxi2QjeD.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Korean_ThoughtfulWoman','韩文','Thoughtful Woman','https://scene.vidu.zone/media-asset/070806-ln9rt4eTeJtp8zUc.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_WiseElf','韩文','Wise Elf','https://scene.vidu.zone/media-asset/070803-E9GisuGpHGPN2b6b.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Korean_WiseTeacher','韩文','Wise Teacher','https://scene.vidu.zone/media-asset/070805-V18z7I9BacTBWmdr.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('lengdan_xiongzhang','中文 (普通话)','冷淡学长','https://scene.vidu.zone/media-asset/072358-YzI6fisVFXYBDZj8.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('linktest-vc-1788270112340','克隆音色','克隆-linktest','http://localhost:8082/media/t_7777804c2e7e/2026-09-01/c-7c6227eb8042.mp3','2026-09-01 21:41:54.831',0,'clone','t_7777804c2e7e','gen-1788271493724713600','active',0,NULL),('linktest-vc-1788270774802','克隆音色','克隆-linktest','http://localhost:8082/media/t_7777804c2e7e/2026-09-01/c-81009ae961c5.mp3','2026-09-01 21:53:00.129',0,'clone','t_7777804c2e7e','gen-1788270780768798400','active',0,NULL),('lovely_girl','中文 (普通话)','萌萌女童','https://scene.vidu.zone/media-asset/072358-lG8JoFnhYTMZF0u2.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-badao','中文 (普通话)','霸道青年音色','https://scene.vidu.zone/media-asset/072356-ypZmFjxo84q476Ds.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-badao-jingpin','中文 (普通话)','霸道青年音色-beta','https://scene.vidu.zone/media-asset/072357-HYMATRMvsle2yt76.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-daxuesheng','中文 (普通话)','青年大学生音色','https://scene.vidu.zone/media-asset/072356-T4kIZ1khgduPiW2i.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-daxuesheng-jingpin','中文 (普通话)','青年大学生音色-beta','https://scene.vidu.zone/media-asset/072357-5rEScknS588MBpFa.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-jingying','中文 (普通话)','精英青年音色','https://scene.vidu.zone/media-asset/072356-7hgJgP689lvUsESC.mp3','2026-08-22 09:20:56.463',1,'vidu','','','active',0,NULL),('male-qn-jingying-jingpin','中文 (普通话)','精英青年音色-beta','https://scene.vidu.zone/media-asset/072357-LE3qZSK6GAKVnYeW.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-qingse','中文 (普通话)','青涩青年音色','https://scene.vidu.zone/media-asset/072356-uLeoUwGWiQQZLiKH.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('male-qn-qingse-jingpin','中文 (普通话)','青涩青年音色-beta','https://scene.vidu.zone/media-asset/072356-HnCFvXEmdYsw1aR3.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('platform-1788189228976059600','ƽ̨��ѡ','ƽ̨��������ɫ','http://localhost:8082/media/tenant-user-1783606014397225300/2026-08-31/e89a6174b6ab.mp3','2026-08-31 23:13:49.026',0,'platform','','','active',1,NULL),('platform-1788194919987564600','平台精选','ƽ̨��������������','http://localhost:8082/media/tenant-user-1783606014397225300/2026-09-01/5629e7374696.mp3','2026-09-01 00:48:40.044',0,'platform','','','active',0,NULL),('platform-1788273484262706400','平台精选','链路测试-公网样本音色','https://scene.vidu.zone/media-asset/072015-ireMGTMsZwkcvnpK.mp3','2026-09-01 22:38:04.353',0,'platform','','','active',0,'2026-09-01 22:38:20.504'),('Portuguese_AngryMan','葡萄牙文','Angry Man','https://scene.vidu.zone/media-asset/035855-awE9rIRE6CCX7cr3.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_AnimeCharacter','葡萄牙文','Anime Character','https://scene.vidu.zone/media-asset/035854-n3daCwqnRBEvicdE.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_Arnold','葡萄牙文','Arnold','https://scene.vidu.zone/media-asset/035859-oqiwsebuANXiPWzY.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_AssertiveQueen','葡萄牙文','Assertive Queen','https://scene.vidu.zone/media-asset/035901-f2hIpqsJUw0QAUQ8.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_AttractiveGirl','葡萄牙文','Attractive Girl','https://scene.vidu.zone/media-asset/035856-R4vQmWuBLAgdKhWk.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_BossyLeader','葡萄牙文','Bossy Leader','https://scene.vidu.zone/media-asset/035852-HoggtsELGRNc3GE8.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_CalmLeader','葡萄牙文','Calm Leader','https://scene.vidu.zone/media-asset/035900-yo1DNh5FHmfaLJ8I.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_CaptivatingStoryteller','葡萄牙文','Captivating Storyteller','https://scene.vidu.zone/media-asset/035855-veYMckFU2ePNhhed.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_CaringGirlfriend','葡萄牙文','Caring Girlfriend','https://scene.vidu.zone/media-asset/035901-dYYDcfRCYqaAavGB.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_CharmingLady','葡萄牙文','Charming Lady','https://scene.vidu.zone/media-asset/035859-hutvd6TgGMsoPs52.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_CharmingQueen','葡萄牙文','Charming Queen','https://scene.vidu.zone/media-asset/035858-jL86FYAsVrayEr3t.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_CharmingSanta','葡萄牙文','Charming Santa','https://scene.vidu.zone/media-asset/035900-3XWbqpa4vfztTO4E.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ChattyGirl','葡萄牙文','Chatty Girl','https://scene.vidu.zone/media-asset/035903-KlRb6Hac52sRZpia.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Comedian','葡萄牙文','Comedian','https://scene.vidu.zone/media-asset/035858-v9S79XJMHYC5AMVn.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_CompellingGirl','葡萄牙文','Compelling Girl','https://scene.vidu.zone/media-asset/035902-pMFk9vEGzSahc8oD.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ConfidentWoman','葡萄牙文','Confident Woman','https://scene.vidu.zone/media-asset/035854-tfBs4WaHLxMrxpgY.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_Conscientiousinstructor','葡萄牙文','Conscientious Instructor','https://scene.vidu.zone/media-asset/035903-1qJIzbA1njepzwb0.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Debator','葡萄牙文','Debator','https://scene.vidu.zone/media-asset/035856-wLQkVaEn4ltFKwfY.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Deep-VoicedGentleman','葡萄牙文','Deep-voiced Gentleman','https://scene.vidu.zone/media-asset/035853-wJtnnAnKT6HUGhDq.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_DeterminedManager','葡萄牙文','Determined Manager','https://scene.vidu.zone/media-asset/035904-GeDXhMqAPWrLCtUO.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Dramatist','葡萄牙文','Dramatist','https://scene.vidu.zone/media-asset/035858-xrZyimVJqko3sCMl.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ElegantGirl','葡萄牙文','Elegant Girl','https://scene.vidu.zone/media-asset/035902-vYnTwbBn3YUA5HWB.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_EnergeticBoy','葡萄牙文','Energetic Boy','https://scene.vidu.zone/media-asset/035900-oFr6TDZwXmmO7v2A.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_FascinatingBoy','葡萄牙文','Fascinating Boy','https://scene.vidu.zone/media-asset/035902-HbIyHyv73BjwzX1d.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_FragileBoy','葡萄牙文','Fragile Boy','https://scene.vidu.zone/media-asset/035903-444V6STPDnIA8rlC.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_FrankLady','葡萄牙文','Frank Lady','https://scene.vidu.zone/media-asset/035904-nkMHVAax7JlOaSXK.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_FriendlyNeighbor','葡萄牙文','Friendly Neighbor','https://scene.vidu.zone/media-asset/035901-q7DRgysg08msA6jz.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Fussyhostess','葡萄牙文','Fussy hostess','https://scene.vidu.zone/media-asset/035858-ytmj6ARG8wJY0zgU.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_GentleTeacher','葡萄牙文','Gentle Teacher','https://scene.vidu.zone/media-asset/035900-T3S61GdfYo3HexD0.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Ghost','葡萄牙文','Ghost','https://scene.vidu.zone/media-asset/035900-9xZwPXOCa4T7pQdn.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Godfather','葡萄牙文','Godfather','https://scene.vidu.zone/media-asset/035855-FMP1Um96WiGR5XPi.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_GorgeousLady','葡萄牙文','Gorgeous Lady','https://scene.vidu.zone/media-asset/035857-YumK9obwueApttqM.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_GrimReaper','葡萄牙文','Grim Reaper','https://scene.vidu.zone/media-asset/035901-z8oylmGNYspuGGqJ.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Grinch','葡萄牙文','Grinch','https://scene.vidu.zone/media-asset/035856-iovsMZhW6bIdcOyl.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_HumorousElder','葡萄牙文','Humorous Elder','https://scene.vidu.zone/media-asset/035900-XkTYRMZgfW4kqUwF.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_InspiringLady','葡萄牙文','Inspiring Lady','https://scene.vidu.zone/media-asset/035902-Dq43ofPynPKCWcDV.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Jovialman','葡萄牙文','Jovial Man','https://scene.vidu.zone/media-asset/035859-UAXsPyxBdOfwQBYu.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Kind-heartedGirl','葡萄牙文','Kind-hearted Girl','https://scene.vidu.zone/media-asset/035855-xgWXj5fvfJMFlfcD.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_LovelyLady','葡萄牙文','Lovely Lady','https://scene.vidu.zone/media-asset/035857-LjaJCiavoulZ8qMp.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_MaturePartner','葡萄牙文','Mature Partner','https://scene.vidu.zone/media-asset/035857-TRbIdcLHSjdPrDm4.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Narrator','葡萄牙文','Narrator','https://scene.vidu.zone/media-asset/035858-nZazjRA8D6itSIQa.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_NaughtySchoolgirl','葡萄牙文','Naughty Schoolgirl','https://scene.vidu.zone/media-asset/035857-Zw6CdhgQdMVR2gn6.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_PassionateWarrior','葡萄牙文','Passionate Warrior','https://scene.vidu.zone/media-asset/035854-rMppnUCpkVnOLyS7.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_PlayfulGirl','葡萄牙文','Playful Girl','https://scene.vidu.zone/media-asset/035857-qUC9mEQIQwjkypXB.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_PlayfulSpirit','葡萄牙文','Playful Spirit','https://scene.vidu.zone/media-asset/035902-MBnqocwWFe2h7jiZ.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Pompouslady','葡萄牙文','Pompous lady','https://scene.vidu.zone/media-asset/035856-BSaHTj6rKm5fjzkx.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_PowerfulSoldier','葡萄牙文','Powerful Soldier','https://scene.vidu.zone/media-asset/035902-xvV50q5t9Si9yhz2.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_PowerfulVeteran','葡萄牙文','Powerful Veteran','https://scene.vidu.zone/media-asset/035902-XRyXNCeGd69A6DTh.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_RationalMan','葡萄牙文','Rational Man','https://scene.vidu.zone/media-asset/035904-6tXudawKmYULpkMH.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ReliableMan','葡萄牙文','Reliable Man','https://scene.vidu.zone/media-asset/035900-mOO06UtkR3L3Z5sk.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ReservedYoungMan','葡萄牙文','Reserved Young Man','https://scene.vidu.zone/media-asset/035855-oQnIXx9ucDlTxRFp.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_RomanticHusband','葡萄牙文','Romantic Husband','https://scene.vidu.zone/media-asset/035902-OnCTfG9JOtNWtvpz.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Rudolph','葡萄牙文','Rudolph','https://scene.vidu.zone/media-asset/035859-AjmtEs1i35CNeNtP.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_SadTeen','葡萄牙文','Sad Teen','https://scene.vidu.zone/media-asset/035857-qTI9KBW9aZguBPmg.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_SantaClaus','葡萄牙文','Santa Claus','https://scene.vidu.zone/media-asset/035859-RH6FvmBORiishhBp.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_SensibleManager','葡萄牙文','Sensible Manager','https://scene.vidu.zone/media-asset/035903-J4MWHZ4NdSpD0MSV.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_SentimentalLady','葡萄牙文','Sentimental Lady','https://scene.vidu.zone/media-asset/035852-YlDdiriZ28E7YPD3.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_SereneElder','葡萄牙文','Serene Elder','https://scene.vidu.zone/media-asset/035900-N32UA3kyBBsGOSyF.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_SereneWoman','葡萄牙文','Serene Woman','https://scene.vidu.zone/media-asset/035857-Ofwsa1H50twRBl65.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_SmartYoungGirl','葡萄牙文','Smart Young Girl','https://scene.vidu.zone/media-asset/035855-5h9TxVIUy8yq7ulL.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Steadymentor','葡萄牙文','Steady Mentor','https://scene.vidu.zone/media-asset/035859-1OYHXRajjtcdGHnt.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_StressedLady','葡萄牙文','Stressed Lady','https://scene.vidu.zone/media-asset/035901-uWpA9FlhEmvgR2JN.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_StrictBoss','葡萄牙文','Strict Boss','https://scene.vidu.zone/media-asset/035902-Z8rwVTZrF8aOQqhm.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Strong-WilledBoy','葡萄牙文','Strong-willed Boy','https://scene.vidu.zone/media-asset/035854-FQnA7navsy0NEpPH.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_SweetGirl','葡萄牙文','Sweet Girl','https://scene.vidu.zone/media-asset/035856-XwSlnuwIu8ShiEeh.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_TheatricalActor','葡萄牙文','Theatrical Actor','https://scene.vidu.zone/media-asset/035903-5h7YzW7aBEEZjEyG.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ThoughtfulLady','葡萄牙文','Thoughtful Lady','https://scene.vidu.zone/media-asset/035903-l7SdRhUPCnKkE37N.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ThoughtfulMan','葡萄牙文','Thoughtful Man','https://scene.vidu.zone/media-asset/035856-RudMXh4sUyusfVtR.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_ToughBoss','葡萄牙文','Tough Boss','https://scene.vidu.zone/media-asset/035858-DnCcxoFUSoy9kFWO.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_UpsetGirl','葡萄牙文','Upset Girl','https://scene.vidu.zone/media-asset/035853-jBFprcgHgXk3KHJ0.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_WhimsicalGirl','葡萄牙文','Whimsical Girl','https://scene.vidu.zone/media-asset/035901-5OphYxHPDfwnb9dn.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Portuguese_Wiselady','葡萄牙文','Wise lady','https://scene.vidu.zone/media-asset/035852-kFdHiHIUzA4DqmkY.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Portuguese_WiseScholar','葡萄牙文','Wise Scholar','https://scene.vidu.zone/media-asset/035904-m9OqbjpCSb1ptKNX.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('qiaopi_mengmei','中文 (普通话)','俏皮萌妹','https://scene.vidu.zone/media-asset/072359-Oz5Tzdi3pbCJKq4T.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Robot_Armor','中文 (普通话)','机械战甲','https://scene.vidu.zone/media-asset/072014-QH6ZIG4sFWV5LG1n.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Rudolph','英文','Rudolph','https://scene.vidu.zone/media-asset/071752-5FQ6r55BOWg4ZBPf.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Russian_AmbitiousWoman','俄文','Ambitious Woman','https://scene.vidu.zone/media-asset/035504-UacsdEy0A4ftk3Cv.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_AttractiveGuy','俄文','Attractive Guy','https://scene.vidu.zone/media-asset/035506-XHTn8GohW6Xanqcd.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_Bad-temperedBoy','俄文','Bad-tempered Boy','https://scene.vidu.zone/media-asset/035506-JZ8O4tOrI74ttkS1.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_BrightHeroine','俄文','Bright Queen','https://scene.vidu.zone/media-asset/035504-rz3mB6An408J7O87.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_CrazyQueen','俄文','Crazy Girl','https://scene.vidu.zone/media-asset/035505-FIDsbXD0MlA24Fqd.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_HandsomeChildhoodFriend','俄文','Handsome Childhood Friend','https://scene.vidu.zone/media-asset/035504-F9VKfKDNyrUjIDXp.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_PessimisticGirl','俄文','Pessimistic Girl','https://scene.vidu.zone/media-asset/035505-NbPMW7x5dIOFqoFw.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Russian_ReliableMan','俄文','Reliable Man','https://scene.vidu.zone/media-asset/035505-DyEhHRWGSCmoD7sy.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Serene_Woman','英文','Serene Woman','https://scene.vidu.zone/media-asset/071753-yBDI9GT2weSnwxH0.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('Spanish_AngryMan','西班牙文','Angry Man','https://scene.vidu.zone/media-asset/070353-wMFRpWf3HJtM1UFj.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_AnimeCharacter','西班牙文','Anime Character','https://scene.vidu.zone/media-asset/070350-I7IpRZADL89QmNr4.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Arnold','西班牙文','Arnold','https://scene.vidu.zone/media-asset/070352-s0BE4PMDHdgFM6B7.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_AssertiveQueen','西班牙文','Assertive Queen','https://scene.vidu.zone/media-asset/070353-SOB6ejA1vbx9Ew2p.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_BossyLeader','西班牙文','Bossy Leader','https://scene.vidu.zone/media-asset/070349-4NnLw9Fb47nBWTHl.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_CaptivatingStoryteller','西班牙文','Captivating Storyteller','https://scene.vidu.zone/media-asset/070348-PGyDV29JR9dJ2VgT.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_CaringGirlfriend','西班牙文','Caring Girlfriend','https://scene.vidu.zone/media-asset/070353-z24jAExWjbtgZLeB.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ChattyGirl','西班牙文','Chatty Girl','https://scene.vidu.zone/media-asset/070353-8s7xO5eZuODxqnVp.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Comedian','西班牙文','Comedian','https://scene.vidu.zone/media-asset/070351-kGXsC7vw2Adroyb0.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_CompellingGirl','西班牙文','Compelling Girl','https://scene.vidu.zone/media-asset/070353-jFRPs3IACR6s98FO.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ConfidentWoman','西班牙文','Confident Woman','https://scene.vidu.zone/media-asset/070349-Sc11mXoLeuzpZclC.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Debator','西班牙文','Debator','https://scene.vidu.zone/media-asset/070351-PS3VY7ripY9uOCZS.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Deep-tonedMan','西班牙文','Deep-toned Man','https://scene.vidu.zone/media-asset/070350-JwMDuPnqCNNW5IBh.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_DeterminedManager','西班牙文','Determined Manager','https://scene.vidu.zone/media-asset/070349-7gi92LMSHEzsX9Kj.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_EnergeticBoy','西班牙文','Energetic Boy','https://scene.vidu.zone/media-asset/070352-dXiikyTM3I1PETQJ.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_FrankLady','西班牙文','Frank Lady','https://scene.vidu.zone/media-asset/070351-a8bCoh4IPbRuCM5t.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Fussyhostess','西班牙文','Fussy hostess','https://scene.vidu.zone/media-asset/070350-IGuZpp48aM8yY2Q1.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Ghost','西班牙文','Ghost','https://scene.vidu.zone/media-asset/070352-EFd8ZbN5bkFQLFZD.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_HumorousElder','西班牙文','Humorous Elder','https://scene.vidu.zone/media-asset/070352-doZ3HajXy0fVStme.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Intonategirl','西班牙文','Intonate Girl','https://scene.vidu.zone/media-asset/070352-T7spxLWURyEpAXBU.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Jovialman','西班牙文','Jovial Man','https://scene.vidu.zone/media-asset/070351-agsQVyTWeHc8sg2m.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Kind-heartedGirl','西班牙文','Kind-hearted Girl','https://scene.vidu.zone/media-asset/070349-bYiRLGRYXNE1rk6W.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_MaturePartner','西班牙文','Mature Partner','https://scene.vidu.zone/media-asset/070348-7WnSyhf1GhpCsoMw.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Narrator','西班牙文','Narrator','https://scene.vidu.zone/media-asset/070349-PRZutZ6UeOPHCSYG.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_PassionateWarrior','西班牙文','Passionate Warrior','https://scene.vidu.zone/media-asset/070353-9tKsw355jAMoch00.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_PowerfulSoldier','西班牙文','Powerful Soldier','https://scene.vidu.zone/media-asset/070353-l6f5Aclawcu2Rc2f.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_PowerfulVeteran','西班牙文','Powerful Veteran','https://scene.vidu.zone/media-asset/070353-qTcs6fwjkUCkWoeg.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_RationalMan','西班牙文','Rational Man','https://scene.vidu.zone/media-asset/070350-SqRmKH5xSvmRiFnu.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ReliableMan','西班牙文','Reliable Man','https://scene.vidu.zone/media-asset/070352-2qTxwDoZM5Lfgl8q.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ReservedYoungMan','西班牙文','Reserved Young Man','https://scene.vidu.zone/media-asset/070349-YKV7CZOpLJ9k6SLc.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_RomanticHusband','西班牙文','Romantic Husband','https://scene.vidu.zone/media-asset/070353-97u2UFJuOzP9KuZ5.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Rudolph','西班牙文','Rudolph','https://scene.vidu.zone/media-asset/070351-3sar9F7Mzmb9HxsB.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_SantaClaus','西班牙文','Santa Claus','https://scene.vidu.zone/media-asset/070351-q2Aip3axlveDRoT6.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_SensibleManager','西班牙文','Sensible Manager','https://scene.vidu.zone/media-asset/070354-gnAWCgi02ly0jK1a.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_SereneElder','西班牙文','Serene Elder','https://scene.vidu.zone/media-asset/070352-sPdXlPqdTJ0vbUgl.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_SereneWoman','西班牙文','Serene Woman','https://scene.vidu.zone/media-asset/070348-UoReuAOn6CTUdFKy.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_SincereTeen','西班牙文','Sincere Teen','https://scene.vidu.zone/media-asset/070350-Zx4RHy7L8MXfeaO3.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_SophisticatedLady','西班牙文','Sophisticated Lady','https://scene.vidu.zone/media-asset/070350-a2rPvecdnw2Iw60m.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Steadymentor','西班牙文','Steady Mentor','https://scene.vidu.zone/media-asset/070351-E07E5IJbNT2BpU7i.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_StrictBoss','西班牙文','Strict Boss','https://scene.vidu.zone/media-asset/070352-YGHLV8bEitcCbrVs.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Strong-WilledBoy','西班牙文','Strong-willed Boy','https://scene.vidu.zone/media-asset/070350-A7rCZfmbP6pdrDnR.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ThoughtfulLady','西班牙文','Thoughtful Lady','https://scene.vidu.zone/media-asset/070354-Cog8TJzC2MmtLFub.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ThoughtfulMan','西班牙文','Thoughtful Man','https://scene.vidu.zone/media-asset/070349-fVT2wAMyiFAAb6y6.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_ToughBoss','西班牙文','Tough Boss','https://scene.vidu.zone/media-asset/070351-oFFfNws2IzDsB3mN.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_WhimsicalGirl','西班牙文','Whimsical Girl','https://scene.vidu.zone/media-asset/070352-DlLwt9TbRy8FyJmt.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_Wiselady','西班牙文','Wise Lady','https://scene.vidu.zone/media-asset/070351-rsBnYfrjJEdtwIO0.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Spanish_WiseScholar','西班牙文','Wise Scholar','https://scene.vidu.zone/media-asset/070349-gybZkG82B0KOjBsI.mp3','2026-08-22 09:20:56.517',0,'vidu','','','active',0,NULL),('Sweet_Girl','英文','Sweet Girl','https://scene.vidu.zone/media-asset/071753-e9wzMCB7DmaEGuhu.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('test6666','克隆音色','克隆-test6666','http://localhost:8082/media/t_7777804c2e7e/2026-09-02/c-613ea8acf4de.mp3','2026-09-02 12:09:28.479',0,'clone','t_7777804c2e7e','gen-1788322578478046200','active',0,NULL),('test6668','克隆音色','克隆-test6668','http://localhost:8082/media/tenant-user-1783606014397225300/2026-09-02/c-9f6f66e276a0.mp3','2026-09-02 12:22:36.759',0,'clone','tenant-user-1783606014397225300','gen-1788322950489317100','active',0,NULL),('tianxin_xiaoling','中文 (普通话)','甜心小玲','https://scene.vidu.zone/media-asset/072359-AqyIK6uEnVwlF6jh.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL),('trace99999','克隆音色','克隆-trace999','http://localhost:8082/media/t_7777804c2e7e/2026-09-02/c-8dcd632b7acf.mp3','2026-09-02 11:35:47.362',0,'clone','t_7777804c2e7e','gen-1788320145505483300','active',0,NULL),('Turkish_CalmWoman','土耳其文','Calm Woman','https://scene.vidu.zone/media-asset/035209-qy1MxzMRbLQPEx5f.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Turkish_Trustworthyman','土耳其文','Trustworthy man','https://scene.vidu.zone/media-asset/035209-Md7ztqRkrvs3mXJH.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Ukrainian_CalmWoman','乌克兰文','Calm Woman','https://scene.vidu.zone/media-asset/034930-ELzwPhKz4Q3UiOyA.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Ukrainian_WiseScholar','乌克兰文','Wise Scholar','https://scene.vidu.zone/media-asset/034930-STPANqlkjzZwA87W.mp3','2026-08-22 09:20:56.570',0,'vidu','','','active',0,NULL),('Vietnamese_kindhearted_girl','越南文','Kind-hearted girl','https://scene.vidu.zone/media-asset/034727-vLCYIZiGCweFE3TC.mp3','2026-08-22 09:20:56.622',0,'vidu','','','active',0,NULL),('voice-clone-104','克隆音色','克隆-voice-cl','','2026-08-31 15:06:38.105',0,'clone','tenant-user-1783606014397225300','gen-1787478189971917900','active',0,NULL),('voice-clone-670','克隆音色','克隆-voice-cl','','2026-08-31 15:06:38.105',0,'clone','tenant-user-1783606014397225300','gen-1787478410683635300','active',0,NULL),('wumei_yujie','中文 (普通话)','妩媚御姐','https://scene.vidu.zone/media-asset/072359-9eRzdU0O3KKxnSWR.mp3','2026-08-22 09:20:56.463',0,'vidu','','','active',0,NULL);

