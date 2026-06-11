import NodeFS from 'node:fs'
import NodePath from 'node:path'
import Process from 'node:process'

// Vocs only emits <link rel="canonical"> and <meta property="og:url"> when
// `baseUrl` is set, but `baseUrl` also injects a <base> tag that breaks asset
// resolution on non-prod origins (see vocs.config.ts). This post-build step
// injects both tags into the prerendered HTML instead.
const SITE_URL = 'https://www.opensigner.dev'

const distDir = NodePath.join(Process.cwd(), 'dist')
const htmlRoot = NodeFS.existsSync(NodePath.join(distDir, 'public', 'index.html'))
  ? NodePath.join(distDir, 'public')
  : distDir

function collectHtmlFiles(dir) {
  const files = []
  for (const entry of NodeFS.readdirSync(dir, { withFileTypes: true })) {
    const full = NodePath.join(dir, entry.name)
    if (entry.isDirectory()) files.push(...collectHtmlFiles(full))
    else if (entry.name.endsWith('.html')) files.push(full)
  }
  return files
}

if (!NodeFS.existsSync(NodePath.join(htmlRoot, 'index.html'))) {
  throw new Error(`inject-seo-head: no index.html under ${htmlRoot} — run \`vocs build\` first`)
}

let injected = 0
for (const file of collectHtmlFiles(htmlRoot)) {
  const route = `/${NodePath.relative(htmlRoot, file)}`
    .replace(/index\.html$/, '')
    .replace(/\.html$/, '')
  if (route === '/404') continue
  const url = route === '/' ? `${SITE_URL}/` : `${SITE_URL}${route.replace(/\/$/, '')}`
  const html = NodeFS.readFileSync(file, 'utf8')
  if (html.includes('rel="canonical"')) continue
  const tags = `<link rel="canonical" href="${url}"/><meta property="og:url" content="${url}"/>`
  NodeFS.writeFileSync(file, html.replace('</head>', `${tags}</head>`))
  injected += 1
}

console.log(`inject-seo-head: injected canonical + og:url into ${injected} pages`)
