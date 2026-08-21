/** 工作台地图热点类型与数据入口（真实数据后续接 API；演示 mock 不入库） */

export type ProvinceHeat = {
  name: string
  /** 获客热度 0–100 */
  heat: number
  /** 近 7 日线索 */
  leads: number
  /** 近 7 日发布量 */
  posts: number
}

export type CityHotspot = {
  id: string
  name: string
  province: string
  /** [lng, lat] */
  coord: [number, number]
  heat: number
  leads: number
  posts: number
  topic: string
  growth: number
  industry: string
}

/** 省份热力——默认空，由后端或本地 mock 注入 */
export const PROVINCE_HEAT: ProvinceHeat[] = []

/** 城市热点——默认空 */
export const CITY_HOTSPOTS: CityHotspot[] = []

export function provinceByName(name: string): ProvinceHeat | undefined {
  return PROVINCE_HEAT.find((p) => p.name === name || name.startsWith(p.name))
}
