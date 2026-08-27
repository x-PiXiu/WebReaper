# 20 — 主体创建（subject）联调问题与修复清单

> **用途**：记录「资产库 → 创建数字分身 / 场景主体」联调时暴露的**前后端缺口**，与 [16-统一生成后端已知问题与修复清单](./16-统一生成后端已知问题与修复清单.md) 并列维护。  
> **记录日期**：2026-08-27  
> **典型现象**：用户已上传 1 张形象照并填写名称，点击「创建」后 toast 报错：  
> `主体需上传 1-3 张形象照或 1 个视频（5 秒内，仅 q2-pro 参考生支持）`

---

## 根因摘要

| 层级 | 问题 |
|------|------|
| **前端（已修）** | `CreateSubjectModal` 仅把图片 **URL** 放进 `materials`，未传上传接口返回的 **素材 ID**；主体视频上传后未参与提交。 |
| **后端（待修）** | `submitSubject` 只从素材库 `List` 反查图片，**不合并**请求体 `params.images` / `params.videos`；**不解析视频素材**；`List` 失败时静默丢弃，最终 `images`/`videos` 为空触发 Vidu 校验错误。 |

错误文案来自 `internal/adapter/provider/viduendpoint/capabilities.go` → `errSubjectMediaRequired`，在 `subjectAdapter.Validate` 中 `len(images)==0 && len(videos)==0` 时返回。

---

## 问题表

| ID | 优先级 | 问题 | 典型现象 | 前端 | 后端 |
|----|--------|------|----------|------|------|
| BE-SUBJ-02 | **P0** | `submitSubject` 忽略 `in.Params["images"]` / `["videos"]` | 前端已传 `params.images`，仍报「需上传形象照」 | ✅ `buildSubjectRegisterPayload` 双写 `materials`+`params` | ❌ 待合并 Params 后再校验 |
| BE-SUBJ-03 | **P0** | `submitSubject` 未把视频素材写入 `params.videos` | 只传 ≤5s 主体视频、无图时创建失败 | ✅ 提交 `params.videos` + 视频 URL 入 `materials` | ❌ 仅扫描 `MaterialTypeImage` |
| BE-SUBJ-04 | P1 | 素材 `List` 失败被静默吞掉 | 上传成功但创建报媒体必填 | — | ❌ `err==nil` 才解析，无日志/无降级 |
| BE-SUBJ-05 | P1 | `uc.asset == nil` 时无法创建主体 | 部署未注入 MediaStore 时必现 | 提示检查后端配置 | ❌ 应支持 URL 直传 `params.images` |
| BE-SUBJ-06 | P2 | URL 匹配仅 `==` 全等 | `PUBLIC_BASE_URL` 与浏览器所见 URL 不一致时反查失败 | ✅ 改传素材 `id` | 建议规范化 URL 或支持 path 后缀匹配 |
| FE-SUBJ-01 | P0 | 创建主体未传素材 ID | 见上文典型现象 | ✅ 已修 | — |
| FE-SUBJ-02 | P1 | 未校验 `brandId` | `brand_id` 空串仍提交 | ✅ 已提示选人设 | — |
| FE-SUBJ-03 | P2 | 主体视频上传未提交 | 选视频仍失败 | ✅ 已接入 `params.videos` | 依赖 BE-SUBJ-03 |

---

## 后端建议修复（`generation.go` → `submitSubject`）

```go
// 伪代码 — 合并顺序建议
params := entity.GenerationParams{"name": in.Text}

// 1) 从 Params 直传（前端 buildSubjectRegisterPayload 已写）
mergeStringSlice(params, "images", getStrings(in.Params, "images"))
mergeStringSlice(params, "videos", getStrings(in.Params, "videos"))

// 2) 再从 materials + asset.List 补全（ID / SourceURL 匹配）
if uc.asset != nil && len(in.Materials) > 0 { ... 现有逻辑 ... }

// 3) List 失败应打日志或返回明确错误，勿静默
if err != nil {
    return entity.GenerationTask{}, fmt.Errorf("读取素材库失败: %w", err)
}

// 4) 视频：除 images 外解析 MaterialTypeVideo → params["videos"]
```

**验收**：

1. 资产库上传 1 张图 + 名称 → 创建成功，`subject` 任务 `state=success`，`creations[0].id` 可作 `server_id`。  
2. 仅上传 ≤5s 主体视频 → 创建成功（q2-pro 参考生路径）。  
3. 口播向导 `SubjectPicker` 能列出新建分身。

---

## 前端已做（2026-08-27）

| 文件 | 改动 |
|------|------|
| `web/src/api/generationSubmit.ts` | 新增 `buildSubjectRegisterPayload()`；`mapLegacyToUnified` 的 `subject` 分支复用 |
| `web/src/pages/merchant/assets/AssetLibrary.tsx` | 上传后保存 `{ id, url }`；提交用素材 ID；校验 `brandId`；视频参与提交 |

---

## 关联

- API：`POST /api/v1/generation/submit`，`sub_type: "subject"`  
- 后端入口：`internal/usecase/generation/generation.go` → `submitSubject`  
- Vidu 校验：`internal/adapter/provider/viduendpoint/endpoints.go` → `subjectAdapter.Validate`  
- 客户端说明：[19-客户端业务逻辑与组件功能说明](./19-客户端业务逻辑与组件功能说明.md) § 口播 / SubjectPicker

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-27 | 初版：联调「上传图不能创建分身」；FE-SUBJ-01~03 已修；BE-SUBJ-02~06 登记待后端 |
