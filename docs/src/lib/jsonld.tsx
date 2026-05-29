type JsonLdData = Record<string, unknown>

const SITE_URL = 'https://opensigner.dev'

export function techArticle(args: { headline: string; description: string; path: string }): JsonLdData {
  return {
    '@context': 'https://schema.org',
    '@type': 'TechArticle',
    headline: args.headline,
    description: args.description,
    url: `${SITE_URL}${args.path}`,
    isPartOf: { '@type': 'WebSite', name: 'OpenSigner', url: SITE_URL },
    publisher: { '@type': 'Organization', name: 'OpenSigner', url: SITE_URL },
  }
}

export function JsonLd({ data }: { data: JsonLdData | JsonLdData[] }) {
  const items = Array.isArray(data) ? data : [data]
  return (
    <>
      {items.map((item) => (
        <script
          key={JSON.stringify(item).slice(0, 64)}
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(item) }}
        />
      ))}
    </>
  )
}
