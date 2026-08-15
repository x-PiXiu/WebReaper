// 受控并发工具：批量监测/批量加词等扇出场景的共享原语。
// 背景（v3 P2）：此前逐项 await 串行——几十个关键词跑数分钟无进度无取消；
// 全量 Promise.all 又会瞬间打满配额。受控并发（默认 3）+ 逐项结算 + 失败聚合。

export type Settled<R> = PromiseSettledResult<R>

/**
 * 受控并发执行：并发度默认 3，逐项结算不抛出（失败聚合在结果里）。
 * onProgress 在每项完成时回调（done/total）——批量场景的进度条数据源。
 */
export async function mapWithConcurrency<T, R>(
  items: T[],
  fn: (item: T, index: number) => Promise<R>,
  concurrency = 3,
  onProgress?: (done: number, total: number) => void,
): Promise<Settled<R>[]> {
  const results = new Array<Settled<R>>(items.length)
  let next = 0
  let done = 0
  const workers = Array.from({ length: Math.min(concurrency, items.length) }, async () => {
    while (next < items.length) {
      const i = next++
      try {
        results[i] = { status: 'fulfilled', value: await fn(items[i], i) }
      } catch (e) {
        results[i] = { status: 'rejected', reason: e }
      }
      done++
      onProgress?.(done, items.length)
    }
  })
  await Promise.all(workers)
  return results
}

/** 结算结果摘要：成功数/失败数（批量完成提示语用）。 */
export function settleSummary<R>(results: Settled<R>[]): { ok: number; failed: number } {
  let ok = 0
  for (const r of results) {
    if (r.status === 'fulfilled') ok++
  }
  return { ok, failed: results.length - ok }
}
