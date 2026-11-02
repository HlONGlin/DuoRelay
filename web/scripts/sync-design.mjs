import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

// The design files use only nested scalar maps in their front matter.
function readDesign(name) {
  const source = readFileSync(new URL(`../../${name}`, import.meta.url), 'utf8')
  const header = source.split(/^---\s*$/m)[1]
  if (!header) throw new Error(`Missing front matter: ${name}`)
  const root = {}
  const stack = [{ indent: -1, value: root }]
  for (const line of header.split('\n')) {
    const match = /^( *)([\w-]+):\s*(.*?)\s*$/.exec(line)
    if (!match) continue
    const [, spaces, key, raw] = match
    const indent = spaces.length
    while (stack.at(-1).indent >= indent) stack.pop()
    const parent = stack.at(-1).value
    if (!raw) {
      parent[key] = {}
      stack.push({ indent, value: parent[key] })
    } else parent[key] = raw.replace(/^"(.*)"$/, '$1')
  }
  return root
}

const light = readDesign('DESIGN.md')
const dark = readDesign('DESIGN-dark.md')
function schema(value) {
  return typeof value === 'object' ? Object.keys(value).sort().map(key => [key, schema(value[key])]) : 'scalar'
}
for (const section of ['colors', 'typography', 'rounded', 'spacing', 'motion', 'components']) {
  if (JSON.stringify(schema(light[section])) !== JSON.stringify(schema(dark[section]))) {
    throw new Error(`Light/dark token structure differs in ${section}`)
  }
}
const cssName = (value) => value.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)
const reference = (value) => value.replace(/\{(colors|typography|rounded|spacing)\.([\w-]+)\}/g,
  (_, section, name) => `var(--${section === 'colors' ? '' : section + '-'}${name})`)
const declarations = (theme, shared) => {
  const lines = Object.entries(theme.colors).map(([key, value]) => `  --${key}: ${value};`)
  if (shared) {
    for (const [section, prefix] of [['rounded', 'rounded'], ['spacing', 'spacing'], ['motion', '']]) {
      for (const [key, value] of Object.entries(theme[section])) lines.push(`  --${prefix ? prefix + '-' : ''}${key}: ${value};`)
    }
    for (const [name, styles] of Object.entries(theme.typography)) {
      for (const [key, value] of Object.entries(styles)) {
        lines.push(`  --typography-${name}-${cssName(key)}: ${key === 'fontFamily' ? `var(--font-${value === 'Inter' ? 'sans' : 'mono'})` : value};`)
      }
    }
  }
  for (const [name, styles] of Object.entries(theme.components)) {
    for (const [key, value] of Object.entries(styles)) {
      if (key !== 'typography') lines.push(`  --${name}-${cssName(key)}: ${reference(value)};`)
    }
  }
  return lines.join('\n')
}
let css = '/* Generated from DESIGN.md and DESIGN-dark.md. Run npm run design:sync. */\n'
css += `:root, [data-theme="light"] {\n${declarations(light, true)}\n}\n\n[data-theme="dark"] {\n${declarations(dark, false)}\n}\n`
for (const [name] of Object.entries(light.typography)) {
  css += `\n.type-${name} {\n${['font-family', 'font-size', 'font-weight', 'line-height', 'letter-spacing'].map((key) => `  ${key}: var(--typography-${name}-${key});`).join('\n')}\n}\n`
}
writeFileSync(new URL('../src/assets/styles/tokens.css', import.meta.url), css)
console.log(`Synced ${Object.keys(light.colors).length} color tokens and ${Object.keys(light.typography).length} type scales to ${fileURLToPath(new URL('../src/assets/styles/tokens.css', import.meta.url))}`)
