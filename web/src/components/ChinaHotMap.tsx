import { useEffect, useMemo, useState } from 'react'
import { Spin, Typography } from 'antd'
import ReactECharts from 'echarts-for-react'
import * as echarts from 'echarts'
import { CITY_HOTSPOTS, PROVINCE_HEAT, type CityHotspot } from '../data/hotspots'

const { Text } = Typography

const GEO_URL = 'https://geo.datav.aliyun.com/areas_v3/bound/100000_full.json'

type Props = {
  height?: number
  selectedId?: string
  onSelectHotspot?: (h: CityHotspot | null) => void
  onSelectProvince?: (name: string) => void
}

/**
 * 中国地图：省份获客热力 + 城市热点气泡（参数：热度/线索/发布/话题）。
 */
export default function ChinaHotMap({
  height = 520,
  selectedId,
  onSelectHotspot,
  onSelectProvince,
}: Props) {
  const [mapReady, setMapReady] = useState(false)
  const [mapError, setMapError] = useState('')

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        if (echarts.getMap('china')) {
          if (!cancelled) setMapReady(true)
          return
        }
        const res = await fetch(GEO_URL)
        if (!res.ok) throw new Error(`地图数据加载失败 ${res.status}`)
        const geo = await res.json()
        echarts.registerMap('china', geo)
        if (!cancelled) setMapReady(true)
      } catch (e) {
        if (!cancelled) setMapError(e instanceof Error ? e.message : '地图加载失败')
      }
    }
    void load()
    return () => { cancelled = true }
  }, [])

  const option = useMemo(() => {
    const isDark = typeof document !== 'undefined'
      && document.documentElement.getAttribute('data-theme') !== 'light'

    const textMuted = isDark ? 'rgba(255,255,255,0.45)' : 'rgba(15,23,42,0.45)'
    const border = isDark ? 'rgba(94,234,212,0.25)' : 'rgba(15,118,110,0.2)'
    const area = isDark ? '#12161f' : '#e8eef2'
    const emphasis = isDark ? '#1a3a3a' : '#cce8e4'

    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        backgroundColor: isDark ? 'rgba(12,14,20,0.94)' : 'rgba(255,255,255,0.96)',
        borderColor: border,
        textStyle: { color: isDark ? '#e8eaed' : '#0f172a', fontSize: 12 },
        formatter: (p: { seriesType?: string; name?: string; data?: Record<string, unknown>; value?: number | number[] }) => {
          if (p.seriesType === 'effectScatter' || p.seriesType === 'scatter') {
            const d = p.data as {
              name: string
              heat: number
              leads: number
              posts: number
              topic: string
              growth: number
              industry: string
            }
            return [
              `<b>${d.name}</b> · ${d.industry}`,
              `热度 ${d.heat} · 线索 ${d.leads} · 发布 ${d.posts}`,
              `话题：${d.topic}`,
              `环比 ${d.growth > 0 ? '+' : ''}${d.growth}%`,
            ].join('<br/>')
          }
          const heat = typeof p.value === 'number' ? p.value : 0
          const row = PROVINCE_HEAT.find((x) => x.name === p.name)
          return [
            `<b>${p.name}</b>`,
            `获客热度 ${heat}`,
            row ? `线索 ${row.leads} · 发布 ${row.posts}` : '',
          ].filter(Boolean).join('<br/>')
        },
      },
      visualMap: {
        min: 0,
        max: 100,
        left: 12,
        bottom: 12,
        text: ['高热', '低热'],
        textStyle: { color: textMuted, fontSize: 11 },
        inRange: {
          color: isDark
            ? ['#1e293b', '#0f766e', '#2dd4bf', '#d4a574']
            : ['#e2e8f0', '#99f6e4', '#0d9488', '#b45309'],
        },
        calculable: true,
        itemWidth: 10,
        itemHeight: 80,
      },
      geo: {
        map: 'china',
        roam: true,
        scaleLimit: { min: 0.8, max: 4 },
        zoom: 1.15,
        label: { show: false },
        itemStyle: {
          areaColor: area,
          borderColor: border,
          borderWidth: 0.8,
        },
        emphasis: {
          itemStyle: { areaColor: emphasis },
          label: { show: true, color: isDark ? '#5eead4' : '#0f766e', fontSize: 11 },
        },
      },
      series: [
        {
          name: '省份热力',
          type: 'map',
          geoIndex: 0,
          data: PROVINCE_HEAT.map((p) => ({ name: p.name, value: p.heat })),
        },
        {
          name: '城市热点',
          type: 'effectScatter',
          coordinateSystem: 'geo',
          zlevel: 2,
          rippleEffect: { brushType: 'stroke', scale: 3.2 },
          symbolSize: (val: number[]) => Math.max(8, Math.min(22, (val[2] || 50) / 5)),
          itemStyle: {
            color: '#d4a574',
            shadowBlur: 12,
            shadowColor: 'rgba(212,165,116,0.55)',
          },
          emphasis: { scale: true },
          data: CITY_HOTSPOTS.map((h) => ({
            name: h.name,
            value: [...h.coord, h.heat],
            id: h.id,
            heat: h.heat,
            leads: h.leads,
            posts: h.posts,
            topic: h.topic,
            growth: h.growth,
            industry: h.industry,
            itemStyle: selectedId === h.id
              ? { color: '#5eead4', borderColor: '#fff', borderWidth: 2 }
              : undefined,
          })),
        },
      ],
    }
  }, [selectedId])

  if (mapError) {
    return (
      <div style={{ height, display: 'grid', placeItems: 'center', padding: 24 }}>
        <Text type="secondary">中国地图暂不可用：{mapError}（请检查网络后刷新）</Text>
      </div>
    )
  }

  if (!mapReady) {
    return (
      <div style={{ height, display: 'grid', placeItems: 'center' }}>
        <Spin tip="加载中国地图…" />
      </div>
    )
  }

  return (
    <ReactECharts
      option={option}
      style={{ height, width: '100%' }}
      opts={{ renderer: 'canvas' }}
      onEvents={{
        click: (params: { seriesType?: string; name?: string; data?: { id?: string } }) => {
          if (params.seriesType === 'effectScatter' && params.data?.id) {
            const hit = CITY_HOTSPOTS.find((h) => h.id === params.data?.id) || null
            onSelectHotspot?.(hit)
            return
          }
          if (params.name) onSelectProvince?.(params.name)
        },
      }}
    />
  )
}
