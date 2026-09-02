import { execFileSync } from 'node:child_process'
import NodeFS from 'node:fs'
import NodePath from 'node:path'
import Process from 'node:process'

// Fails a pull request that deletes or renames a documentation page without
// leaving a redirect behind.
//
// A removed route that 404s costs whatever ranking and inbound links the old URL
// had accumulated, and nothing in the build notices: the page is simply gone, and
// the sitemap is regenerated to match. A permanent redirect passes that history on
// to the replacement page, which is why the redirect is required rather than
// suggested. All redirects live in `vercel.json`; `vocs.config.ts` declares none.
//
// Set `DOCS_ROUTE_GUARD_BASE` to the pull request base ref. Outside a pull request
// the check is a no-op. Apply the `docs-removal-ok` label to downgrade a failure to
// a warning, for the case where a route should genuinely start returning 404.
const base = Process.env.DOCS_ROUTE_GUARD_BASE
if (!base) {
  console.log('check-removed-routes: not a pull request, skipping')
  Process.exit(0)
}

const removalApproved = Process.env.DOCS_REMOVAL_OK === 'true'
const pagesPrefix = 'docs/src/pages'

/** Converts a path under `docs/src/pages` into the route it was served at. */
function routeForPageFile(file) {
  const withoutRoot = file.replace(/^.*?src\/pages/, '')
  const withoutExtension = withoutRoot.replace(/\.mdx?$/, '')
  const withoutIndex = withoutExtension.replace(/\/index$/, '')
  return withoutIndex === '' ? '/' : withoutIndex
}

/** Matches a redirect source against a route, honouring Vercel's `:path*` wildcards. */
function sourceMatchesRoute(source, route) {
  if (source === route) return true
  const wildcard = '/:path*'
  if (!source.endsWith(wildcard)) return false
  const prefix = source.slice(0, -wildcard.length)
  return route === prefix || route.startsWith(`${prefix}/`)
}

function deletedPageFiles() {
  const output = execFileSync(
    'git',
    // --no-renames: a moved page must show as a deletion of its old route, not
    // as an `R` entry the D filter would skip.
    ['diff', '--no-renames', '--diff-filter=D', '--name-only', `${base}...HEAD`, '--', pagesPrefix],
    { encoding: 'utf8' },
  )
  return output
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.endsWith('.md') || line.endsWith('.mdx'))
}

const deleted = deletedPageFiles()
if (deleted.length === 0) {
  console.log('check-removed-routes: no documentation pages were removed')
  Process.exit(0)
}

const configPath = NodePath.join(Process.cwd(), 'docs', 'vercel.json')
const redirectSources = JSON.parse(NodeFS.readFileSync(configPath, 'utf8')).redirects.map(
  (redirect) => redirect.source,
)

const missing = deleted
  .map((file) => ({ file, route: routeForPageFile(file) }))
  .filter(({ route }) => !redirectSources.some((source) => sourceMatchesRoute(source, route)))

if (missing.length === 0) {
  console.log(`check-removed-routes: all ${deleted.length} removed page(s) have a redirect`)
  Process.exit(0)
}

const summary = [
  `${missing.length} removed documentation page(s) have no redirect in docs/vercel.json:`,
  ...missing.map(({ file, route }) => `  ${file} (was served at ${route})`),
  '',
  'Add a permanent redirect for each route, or apply the `docs-removal-ok` label if the',
  'route should genuinely start returning 404.',
].join('\n')

// GitHub renders `::error` as an annotation on the pull request; `%0A` keeps the newlines.
const annotation = summary.replace(/\n/g, '%0A')

if (removalApproved) {
  console.log(`::warning title=Removed docs routes::${annotation}`)
  console.log(summary)
  console.log('\nThe `docs-removal-ok` label is applied, so this is a warning.')
  Process.exit(0)
}

console.log(`::error title=Removed docs routes::${annotation}`)
console.error(summary)
Process.exit(1)
