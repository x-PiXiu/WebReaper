/**
 * 创意工作室素材。
 * 优先用 picsum（比 Unsplash 更稳）；失败时 CSS 假屏仍可呈现效果。
 */
const pic = (id: number, w = 640, h = 960) =>
  `https://picsum.photos/id/${id}/${w}/${h}`

export const CREATIVE_CDN = {
  pipeline: {
    copy: pic(180, 720, 900),
    voice: pic(64, 640, 800),
    mic: pic(26, 480, 720),
    film: pic(1015, 640, 960),
    publish: pic(201, 640, 960),
  },
  avatars: [
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Ada',
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Ben',
    'https://api.dicebear.com/7.x/avataaars/svg?seed=Cora',
  ],
} as const
