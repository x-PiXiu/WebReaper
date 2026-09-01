import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Tabs } from 'antd'
import AgentConfigs from './AgentConfigs'
import GenerationTemplates from './GenerationTemplates'
import PromptTemplates from './PromptTemplates'
import Knowledge from './Knowledge'
import Indexing from './Indexing'

/**
 * 内容引擎配置（复合页——AI/生成/知识库统一管理）：
 * - Agent 配置：获客智能体参数
 * - 生成模板：口播/图文/音色的默认参数模板
 * - 提示词模板：系统级 prompt
 * - 知识库：RAG 素材/采集/向量化
 * - 提交渠道：SEO/收录提交配置
 */
function ContentEngine() {
  const [searchParams] = useSearchParams()
  const [tab, setTab] = useState(searchParams.get('tab') || 'agents')
  return (
    <div className="wr-page-content ip-page">
      <Tabs
        activeKey={tab}
        onChange={setTab}
        items={[
          { key: 'agents', label: 'Agent 配置', children: <AgentConfigs embedded /> },
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
