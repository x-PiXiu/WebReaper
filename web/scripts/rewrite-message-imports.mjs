import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.resolve(__dirname, '../src')
const target = path.resolve(root, 'utils/antdApp.ts')

function walk(dir, out = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name)
    if (ent.isDirectory()) walk(p, out)
    else if (/\.(tsx?)$/.test(ent.name)) out.push(p)
  }
  return out
}

function relImport(fromFile) {
  let rel = path.relative(path.dirname(fromFile), target).replace(/\\/g, '/').replace(/\.ts$/, '')
  if (!rel.startsWith('.')) rel = './' + rel
  return rel
}

const skip = new Set([
  path.resolve(root, 'utils/antdApp.ts'),
  path.resolve(root, 'components/AntdAppApiBridge.tsx'),
  path.resolve(root, 'api/client.ts'),
])

let changed = 0
for (const file of walk(root)) {
  if (skip.has(file)) continue
  let src = fs.readFileSync(file, 'utf8')
  const orig = src
  if (/from ['"].*utils\/antdApp['"]/.test(src)) continue
  if (!/import\s*\{[^}]*\bmessage\b[^}]*\}\s*from\s*['"]antd['"]/.test(src)) continue

  const usesAntdMessage = /\bmessage\s+as\s+antdMessage\b/.test(src) || /\bantdMessage\b/.test(src)

  src = src.replace(/import\s*\{([^}]*)\}\s*from\s*['"]antd['"]\s*;?/g, (full, inner) => {
    const parts = inner.split(',').map((s) => s.trim()).filter(Boolean)
    const kept = parts.filter((p) => !/^message(\s+as\s+\w+)?$/.test(p))
    if (kept.length === parts.length) return full
    if (kept.length === 0) return ''
    return `import { ${kept.join(', ')} } from 'antd'`
  })

  src = src.replace(/\n{3,}/g, '\n\n')

  const importLine = usesAntdMessage
    ? `import { message as antdMessage } from '${relImport(file)}'`
    : `import { message } from '${relImport(file)}'`

  const lines = src.split('\n')
  let insertAt = 0
  for (let i = 0; i < lines.length; i++) {
    if (/^import\s/.test(lines[i])) {
      let j = i
      while (j < lines.length && !/from\s+['"][^'"]+['"]/.test(lines[j])) j++
      insertAt = j + 1
      i = j
    } else if (insertAt > 0) {
      break
    }
  }
  lines.splice(insertAt, 0, importLine)
  src = lines.join('\n')

  if (src !== orig) {
    fs.writeFileSync(file, src)
    changed++
    console.log('updated', path.relative(root, file))
  }
}
console.log('total', changed)
