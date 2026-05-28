import ChildProcess from 'node:child_process'
import Process from 'node:process'
import NodeFS from 'node:fs'
import NodePath from 'node:path'
import { defineConfig } from 'vocs';
import React from 'react';

import { sidebar } from './sidebar'

const SITE_URL = 'https://opensigner.dev'

const commitSha =
  ChildProcess.execSync('git rev-parse --short HEAD').toString().trim() ||
  Process.env.VERCEL_GIT_COMMIT_SHA?.slice(0, 7)

function collectRoutes(dir: string, prefix = ''): string[] {
  const entries = NodeFS.readdirSync(dir, { withFileTypes: true })
  const routes: string[] = []
  for (const entry of entries) {
    const full = NodePath.join(dir, entry.name)
    if (entry.isDirectory()) {
      routes.push(...collectRoutes(full, `${prefix}/${entry.name}`))
      continue
    }
    if (!entry.name.endsWith('.mdx') && !entry.name.endsWith('.md')) continue
    if (entry.name === 'ui_warning.mdx') continue
    const base = entry.name.replace(/\.(mdx|md)$/, '')
    const path = base === 'index' ? `${prefix}/` : `${prefix}/${base}`
    routes.push(path.replace(/\/+/g, '/'))
  }
  return routes
}

const pagesDir = NodePath.join(Process.cwd(), 'pages')
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

function canonicalUrl(path: string): string {
  const normalized = path === '/' ? '/' : path.replace(/\/$/, '')
  return `${SITE_URL}${normalized}`
}

const organizationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  name: 'OpenSigner',
  url: SITE_URL,
  logo: `${SITE_URL}/icons/open-signer-logo.svg`,
  description:
    'OpenSigner is an open-source, self-hostable wallet key management system that issues non-custodial cryptographic keys using Shamir\'s Secret Sharing scheme.',
  sameAs: [
    'https://github.com/openfort-xyz/opensigner',
    'https://x.com/openfort_hq',
    'https://t.me/openfort',
  ],
}

const softwareApplicationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: 'OpenSigner',
  applicationCategory: 'DeveloperApplication',
  operatingSystem: 'Linux, macOS, Windows',
  url: SITE_URL,
  description:
    'Open-source, non-custodial, self-hostable wallet key management using Shamir\'s Secret Sharing.',
  offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
  license: 'https://github.com/openfort-xyz/opensigner/blob/main/LICENSE',
  codeRepository: 'https://github.com/openfort-xyz/opensigner',
}

function techArticleJsonLd(path: string) {
  return {
    '@context': 'https://schema.org',
    '@type': 'TechArticle',
    headline: path,
    url: canonicalUrl(path),
    isPartOf: { '@type': 'WebSite', name: 'OpenSigner docs', url: SITE_URL },
    publisher: { '@type': 'Organization', name: 'OpenSigner', url: SITE_URL },
  }
}

export default defineConfig({
  rootDir: '.',
  description: 'Open-source and non-custodial and self-hostable private key management.',
  title: 'OpenSigner | Non-Custodial Wallet Key Management',
  logoUrl: {
    light: '/icons/open-signer-logo.svg',
    dark: '/icons/open-signer-logo.svg',
  },
  banner: 'If you like OpenSigner, give it a [star on GitHub ⭐](https://github.com/openfort-xyz/opensigner)!',
  iconUrl: "/icons/icon.svg",
  sidebar,
  aiCta: true,
  socials: [
    {
      icon: 'github',
      link: 'https://github.com/openfort-xyz/opensigner',
    },
    {
      icon: 'telegram',
      link: 'https://t.me/openfort',
    },
    {
      icon: 'x',
      link: 'https://x.com/openfort_hq',
    }
  ],
  theme: {
    accentColor: { light: '#004AAD', dark: '#004AAD' },
  },
  head({ path }) {
    const canonical = canonicalUrl(path)
    const jsonLd =
      path === '/'
        ? [organizationJsonLd, softwareApplicationJsonLd]
        : [techArticleJsonLd(path)]

    return (
      <>
        <meta
          content="width=device-width, initial-scale=1, maximum-scale=1"
          name="viewport"
        />
        <link rel="canonical" href={canonical} />
        <meta content="https://opensigner.dev/og-image.png" property="og:image" />
        <meta content="image/png" property="og:image:type" />
        <meta content="1200" property="og:image:width" />
        <meta content="630" property="og:image:height" />
        <meta content={canonical} property="og:url" />
        <meta content={commitSha} name="x-app-version" />
        <meta
          content={
            process.env.VITE_VERCEL_ENV !== 'production'
              ? 'noindex, nofollow'
              : 'index, follow'
          }
          name="robots"
        />
        {jsonLd.map((data, i) => (
          <script
            key={i}
            type="application/ld+json"
            dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
          />
        ))}
      </>
    )
  },
})
