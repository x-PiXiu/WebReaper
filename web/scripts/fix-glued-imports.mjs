import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../src')

function walk(dir, out = []) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, ent.name)
    if (ent.isDirectory()) walk(p, out)
    else if (/\.tsx?$/.test(ent.name)) out.push(p)
  }
  return out
}

let n = 0
for (const file of walk(root)) {
  let src = fs.readFileSync(file, 'utf8')
  const orig = src

  // from 'antd'import → newline
  src = src.replace(/from ['"]antd['"]\s*import/g, (m) => {
    const q = m.includes('"') ? '"' : "'"
    return `from ${q}antd${q}\nimport`
  })

  // Broken: import {\nimport { message } from 'path'\n  Icons...
  src = src.replace(
    /import\s*\{\s*\nimport\s*\{\s*(message(?:\s+as\s+\w+)?)\s*\}\s*from\s*['"]([^'"]+)['"]\s*\n/g,
    (_m, msgImport, from) => `import { ${msgImport} } from '${from}'\nimport {\n`,
  )

  if (src !== orig) {
    fs.writeFileSync(file, src)
    n++
    console.log('fixed', path.relative(root, file))
  }
}
console.log('fixed count', n)
