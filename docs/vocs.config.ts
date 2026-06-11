import NodeFS from 'node:fs'
import NodePath from 'node:path'
import Process from 'node:process'
import { defineConfig } from 'vocs/config'

import { sidebar } from './sidebar'

const SITE_URL = 'https://www.opensigner.dev'

// Generate sitemap.xml + robots.txt at build time. We intentionally do NOT set
// the `baseUrl` config option: it injects <base href={SITE_URL}>, which makes
// every relative asset (including the client bundle) resolve against the prod
// domain and breaks hydration on any other origin (local preview, Vercel preview
// deploys). Emitting these files directly keeps absolute SEO URLs without the tag.
// Canonical + og:url tags (which Vocs also gates behind `baseUrl`) are injected
// post-build by scripts/inject-seo-head.mjs.
function collectRoutes(dir: string, prefix = ''): string[] {
  const routes: string[] = []
  for (const entry of NodeFS.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name.startsWith('_')) continue
    const full = NodePath.join(dir, entry.name)
    if (entry.isDirectory()) {
      routes.push(...collectRoutes(full, `${prefix}/${entry.name}`))
      continue
    }
    if (!/\.(mdx|md|tsx)$/.test(entry.name)) continue
    const base = entry.name.replace(/\.(mdx|md|tsx)$/, '')
    const route = base === 'index' ? prefix || '/' : `${prefix}/${base}`
    routes.push(route.replace(/\/{2,}/g, '/'))
  }
  return routes
}

function generateSeoFiles(): void {
  const publicDir = NodePath.join(Process.cwd(), 'public')
  const routes = collectRoutes(NodePath.join(Process.cwd(), 'src', 'pages')).sort()
  const lastmod = new Date().toISOString().slice(0, 10)

  const sitemap = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...routes.map((route) => {
      const loc = route === '/' ? `${SITE_URL}/` : `${SITE_URL}${route}`
      const priority = route === '/' ? '1.0' : '0.7'
      return `  <url>\n    <loc>${loc}</loc>\n    <lastmod>${lastmod}</lastmod>\n    <priority>${priority}</priority>\n  </url>`
    }),
    '</urlset>',
    '',
  ].join('\n')

  const robots = ['User-agent: *', 'Allow: /', `Sitemap: ${SITE_URL}/sitemap.xml`, ''].join('\n')

  NodeFS.mkdirSync(publicDir, { recursive: true })
  NodeFS.writeFileSync(NodePath.join(publicDir, 'sitemap.xml'), sitemap)
  NodeFS.writeFileSync(NodePath.join(publicDir, 'robots.txt'), robots)
}

generateSeoFiles()

export default defineConfig({
  title: 'OpenSigner | Non-Custodial Wallet Key Management',
  titleTemplate: '%s – OpenSigner',
  description: 'Open-source, non-custodial, self-hostable private key management.',
  logoUrl: {
    light: '/icons/open-signer-logo.svg',
    dark: '/icons/open-signer-logo.svg',
  },
  iconUrl: '/icons/icon.svg',
  banner: 'If you like OpenSigner, give it a [star on GitHub ⭐](https://github.com/openfort-xyz/opensigner)!',
  ogImageUrl: 'https://www.opensigner.dev/og-image.png',
  accentColor: '#004AAD',
  renderStrategy: 'full-static',
  sidebar,
  topNav: [
    { text: 'Guides & API', link: '/introduction/setup' },
    {
      text: 'Contributing',
      link: 'https://github.com/openfort-xyz/opensigner?tab=contributing-ov-file',
    },
  ],
  socials: [
    { icon: 'github', link: 'https://github.com/openfort-xyz/opensigner' },
    { icon: 'telegram', link: 'https://t.me/openfort' },
    { icon: 'x', link: 'https://x.com/openfort_hq' },
  ],
})
