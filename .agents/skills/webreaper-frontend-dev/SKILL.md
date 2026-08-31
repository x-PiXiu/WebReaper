---
name: webreaper-frontend-dev
description: WebReaper 前端开发规范。凡是要修改 WebReaper 项目的页面、组件、样式、前端 API 调用、路由等前端代码时使用；明确只允许修改 web/ 目录下的前端代码，禁止改动 Go 后端，遇到后端问题记录到 Docs/问题反馈.md。Use whenever the user asks to fix, add, or change anything in the WebReaper UI, web pages, components, or frontend logic.
---

# WebReaper 前端开发规范

## 边界（最重要）

1. **只允许修改前端**：即 `web/` 目录下的文件（React + Vite + AntD + TypeScript）。
2. **禁止修改后端**：`cmd/`、`internal/`、`configs/`、`go.mod`、`go.sum`、`Dockerfile`、`docker-compose.yml`、`Makefile` 等一律不动，即使改动看起来很小或"顺手"。
3. **遇到后端问题不要修**：包括 API 返回数据不合理、接口缺失、字段错误、CORS、认证逻辑、构建脚本等。把问题记录到 `Docs/问题反馈.md`（格式见下文），然后在能力范围内用前端方式规避或注明等待后端修复。

## 项目结构（web/src/）

- `api/` — axios 封装与按域拆分的接口模块（`client.ts` 统一实例 + `auth.ts`/`business.ts` 等业务模块）。新接口加到对应业务模块，不新建 axios 实例。
- `pages/` — 页面，按 `admin/`、`merchant/` 角色分目录。
- `components/` — 可复用组件，复杂组件可建子目录（如 `charts/`、`wizard/`）。
- `hooks/` — 自定义 hook，命名 `useXxx.ts`；数据获取优先用 `@tanstack/react-query`（见 `queryClient.ts`）。
- `store/` — zustand 全局状态（`auth.ts`、`theme.ts` 等），局部状态不要放这里。
- `types/` — 共享 TS 类型，API 信封类型在 `types/api.ts`。
- `ui/`、`utils/`、`layouts/`、`styles/`、`constants/`、`config/` — 按既有归类放置新文件。

## 技术栈约定

- React 18 + TypeScript（不用 JS）；类型优先从 `types/api.ts` 或现有文件导入。
- UI 库 **antd v5**；全局 message/modal 经 `utils/antdApp`（AntdAppApiBridge），不要直接 `import { message } from 'antd'` 在模块顶层调用。
- 路由 react-router-dom v6；受保护路由用 `components/ProtectedRoute.tsx`。
- 服务端状态用 react-query；客户端全局状态用 zustand。
- 图表用 echarts / @ant-design/charts。

## API 调用规范

- 所有请求走 `api/apiClient.ts` 的实例（已处理 JWT 注入、`{code,msg,data}` 信封解包、401 跳登录、配额 40201 引导）。
- 后端响应信封为 `{code, msg, data}`，`code !== 0` 视为业务失败；不要在页面里重复解包或重复弹错。
- 新增接口：在 `api/business.ts`（或对应模块）加函数 + 在 `types/api.ts` 补类型，页面通过 react-query hook（`hooks/useXxx.ts`）消费。

## 验证流程

改完前端后在 `web/` 目录依次跑（全部通过才算完成）：

```bash
npm run lint
npm run typecheck
npm run build   # tsc && vite build
npm run test    # vitest run（如有相关测试）
```

不要为了通过检查而删除或绕过已有测试。

## 后端问题反馈文档

遇到后端问题（接口 bug、数据不合理、需要新接口等）时，**不要改后端代码**，在 `Docs/问题反馈.md` 追加一条记录（文件不存在则创建，含表头）：

```markdown
# 后端问题反馈

| 编号 | 日期 | 模块/接口 | 问题描述 | 前端临时规避 | 状态 |
|---|---|---|---|---|---|
| 1 | 2026-08-29 | GET /api/xxx | 描述现象、期望行为、实际返回 | 如已做的前端兜底 | 待后端确认 |
```

编号递增，状态取值：待后端确认 / 后端已修复 / 已关闭。
