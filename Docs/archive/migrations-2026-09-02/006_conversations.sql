-- 006_conversations.sql  聊天会话与消息持久化
--
-- 设计动机：原先聊天记录仅存前端 localStorage（最多 20 条，换设备即丢）。
-- 现持久化到后端，按 user_id 隔离会话，支持跨设备访问历史。

CREATE TABLE IF NOT EXISTS conversations (
    id         VARCHAR(64) PRIMARY KEY,
    title      VARCHAR(128),
    agent_name VARCHAR(64),
    user_id    VARCHAR(64),
    created_at DATETIME(3),
    updated_at DATETIME(3),
    INDEX idx_conversations_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS messages (
    id              VARCHAR(64) PRIMARY KEY,
    conversation_id VARCHAR(64),
    role            VARCHAR(16),
    content         LONGTEXT,
    tool_calls      LONGTEXT,
    created_at      DATETIME(3),
    INDEX idx_messages_conversation (conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
