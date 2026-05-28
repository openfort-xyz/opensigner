'use client'

import type { ReactNode } from 'react'
import { useRouter } from 'waku'
import { MdxPageContext } from 'vocs'

const SITE_URL = 'https://opensigner.dev'
const SITE_TITLE = 'OpenSigner | Non-Custodial Wallet Key Management'

const organizationJsonLd = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  name: 'OpenSigner',
  url: SITE_URL,
  logo: `${SITE_URL}/icons/open-signer-logo.svg`,
  description:
    "OpenSigner is an open-source, self-hostable wallet key management system that issues non-custodial cryptographic keys using Shamir's Secret Sharing scheme.",
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
    "Open-source, non-custodial, self-hostable wallet key management using Shamir's Secret Sharing.",
  offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
  license: 'https://github.com/openfort-xyz/opensigner/blob/main/LICENSE',
  codeRepository: 'https://github.com/openfort-xyz/opensigner',
}

function techArticleJsonLd(path: string, frontmatter: { title?: string; description?: string }) {
  return {
    '@context': 'https://schema.org',
    '@type': 'TechArticle',
    headline: frontmatter.title ?? path,
    description: frontmatter.description,
    url: `${SITE_URL}${path}`,
    isPartOf: { '@type': 'WebSite', name: 'OpenSigner docs', url: SITE_URL },
    publisher: { '@type': 'Organization', name: 'OpenSigner', url: SITE_URL },
  }
}

export default function MdxWrapper({ children }: { children: ReactNode }) {
  const { path } = useRouter()
  const { frontmatter } = MdxPageContext.use()

  const jsonLd =
    path === '/'
      ? [organizationJsonLd, softwareApplicationJsonLd]
      : [techArticleJsonLd(path, frontmatter ?? {})]

  const robots =
    import.meta.env.VITE_VERCEL_ENV && import.meta.env.VITE_VERCEL_ENV !== 'production'
      ? 'noindex, nofollow'
      : 'index, follow'

  const pageTitle = frontmatter?.title
  const fullTitle = pageTitle && !SITE_TITLE.includes(pageTitle) ? `${pageTitle} – ${SITE_TITLE}` : SITE_TITLE
  const description = frontmatter?.description

  const canonical = `${SITE_URL}${path === '/' ? '/' : path.replace(/\/$/, '')}`

  return (
    <>
      <title>{fullTitle}</title>
      {description && <meta name="description" content={description} />}
      {description && <meta property="og:description" content={description} />}
      {pageTitle && <meta property="og:title" content={pageTitle} />}
      <link rel="canonical" href={canonical} />
      <meta property="og:url" content={canonical} />
      <meta name="robots" content={robots} />
      {jsonLd.map((data, i) => (
        <script
          key={i}
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
        />
      ))}
      {children}
    </>
  )
}
