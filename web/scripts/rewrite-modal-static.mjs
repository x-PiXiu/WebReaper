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

let changed = 0
for (const file of walk(root)) {
  if (file.endsWith('antdApp.ts') || file.endsWith('client.ts')) continue
  let src = fs.readFileSync(file, 'utf8')
  const orig = src

  // Only files that call Modal.confirm / Modal.info etc as static API
  if (!/\bModal\.(confirm|info|success|error|warning)\s*\(/.test(src)) continue

  // Replace Modal.xxx( with modal.xxx(
  src = src.replace(/\bModal\.(confirm|info|success|error|warning)\s*\(/g, 'modal.$1(')

  // If modal already imported from antdApp, done for calls
  const hasBridgeModal = /import\s*\{[^}]*\bmodal\b[^}]*\}\s*from\s*['"].*antdApp['"]/.test(src)

  if (!hasBridgeModal) {
    // Remove Modal from antd import only if Modal is no longer referenced as JSX/component
    // Keep Modal in antd import if <Modal appears
    const needsModalComponent = /<Modal[\s>]/.test(src)

    if (!needsModalComponent) {
      src = src.replace(/import\s*\{([^}]*)\}\s*from\s*['"]antd['"]\s*;?/g, (full, inner) => {
        const parts = inner.split(',').map((s) => s.trim()).filter(Boolean)
        const kept = parts.filter((p) => p !== 'Modal')
        if (kept.length === parts.length) return full
        if (kept.length === 0) return ''
        return `import { ${kept.join(', ')} } from 'antd'`
      })
      src = src.replace(/\n{3,}/g, '\n\n')
    }

    // Add modal import from bridge
    if (/import\s*\{([^}]*)\}\s*from\s*['"]([^'"]*antdApp)['"]/.test(src)) {
      src = src.replace(/import\s*\{([^}]*)\}\s*from\s*['"]([^'"]*antdApp)['"]/, (full, inner, from) => {
        const parts = inner.split(',').map((s) => s.trim()).filter(Boolean)
        if (!parts.includes('modal')) parts.push('modal')
        return `import { ${parts.join(', ')} } from '${from}'`
      })
    } else {
      const importLine = `import { modal } from '${relImport(file)}'`
      const lines = src.split('\n')
      let insertAt = 0
      for (let i = 0; i < lines.length; i++) {
        if (/^import\s/.test(lines[i])) {
          let j = i
          while (j < lines.length && !/from\s+['"][^'"]+['"]/.test(lines[j])) j++
          insertAt = j + 1
          i = j
        } else if (insertAt > 0) break
      }
      lines.splice(insertAt, 0, importLine)
      src = lines.join('\n')
    }
  }

  if (src !== orig) {
    fs.writeFileSync(file, src)
    changed++
    console.log('updated', path.relative(root, file))
  }
}
console.log('total', changed)
