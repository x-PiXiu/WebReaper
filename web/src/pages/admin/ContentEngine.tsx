import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import GenerationTemplates from './GenerationTemplates'
import PromptTemplates from './PromptTemplates'
import Knowledge from './Knowledge'
import Indexing from './Indexing'

/**
 * 内容引擎配置（复合页——生成/知识库/收录统一管理）：
 * - 生成模板：口播/图文/音色的默认参数模板
 * - 提示词模板：系统级 prompt
 * - 知识库：RAG 素材/采集/向量化
 * - 提交渠道：SEO/收录提交配置
 *
 * Agent 配置 Tab 已移除（2026-09-02）：agent_configs 表只有管理端 CRUD、
 * 业务运行时（内容生成编排/chat/发布）零消费——纯死配置，避免误导。
 * 后端 /agents CRUD 保留为未接线代码，将来接多 Agent 编排时恢复 Tab 即用。
 */
function ContentEngine() {
  const [searchParams] = useSearchParams()
  const [tab, setTab] = useState(searchParams.get('tab') || 'templates')
  return (
    <div className="wr-page-content ip-page">
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          { key: 'templates', label: '生成模板', children: <GenerationTemplates embedded /> },
          { key: 'prompts', label: '提示词模板', children: <PromptTemplates embedded /> },
          { key: 'knowledge', label: '知识库', children: <Knowledge embedded /> },
          { key: 'indexing', label: '提交渠道', children: <Indexing embedded /> },
        ]}
      />
    </div>
  )
}

export default ContentEngine
