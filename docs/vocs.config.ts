import Process from 'node:process'
import NodeFS from 'node:fs'
import NodePath from 'node:path'
import { defineConfig } from 'vocs/config'

import { sidebar } from './sidebar'

const SITE_URL = 'https://opensigner.dev'

function collectRoutes(dir: string, prefix = ''): string[] {
  const entries = NodeFS.readdirSync(dir, { withFileTypes: true })
  const routes: string[] = []
  for (const entry of entries) {
    const full = NodePath.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name.startsWith('_')) continue
      routes.push(...collectRoutes(full, `${prefix}/${entry.name}`))
      continue
    }
    if (entry.name.startsWith('_')) continue
    if (!entry.name.endsWith('.mdx') && !entry.name.endsWith('.md')) continue
    const base = entry.name.replace(/\.(mdx|md)$/, '')
    const path = base === 'index' ? `${prefix}/` : `${prefix}/${base}`
    routes.push(path.replace(/\/+/g, '/'))
  }
  return routes
}

const pagesDir = NodePath.join(Process.cwd(), 'src', 'pages')
const routes = collectRoutes(pagesDir).sort()

const lastmod = new Date().toISOString().slice(0, 10)
const sitemap = [
  '<?xml version="1.0" encoding="UTF-8"?>',
  '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
  ...routes.map((path) => {
    const loc = `${SITE_URL}${path}`.replace(/\/$/, '/')
    const priority = path === '/' ? '1.0' : '0.7'
    return `  <url>\n    <loc>${loc}</loc>\n    <lastmod>${lastmod}</lastmod>\n    <priority>${priority}</priority>\n  </url>`
  }),
  '</urlset>',
  '',
].join('\n')

NodeFS.writeFileSync(NodePath.join(Process.cwd(), 'public', 'sitemap.xml'), sitemap)

export default defineConfig({
  baseUrl: SITE_URL,
  description: 'Open-source and non-custodial and self-hostable private key management.',
  title: 'OpenSigner | Non-Custodial Wallet Key Management',
  logoUrl: {
    light: '/icons/open-signer-logo.svg',
    dark: '/icons/open-signer-logo.svg',
  },
  banner: 'If you like OpenSigner, give it a [star on GitHub ⭐](https://github.com/openfort-xyz/opensigner)!',
  iconUrl: '/icons/icon.svg',
  sidebar,
  socials: [
    { icon: 'github', link: 'https://github.com/openfort-xyz/opensigner' },
    { icon: 'telegram', link: 'https://t.me/openfort' },
    { icon: 'x', link: 'https://x.com/openfort_hq' },
  ],
  accentColor: '#004AAD',
  ogImageUrl: `${SITE_URL}/og-image.png`,
  renderStrategy: 'full-static',
})
