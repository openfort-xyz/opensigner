import NodeFS from 'node:fs'
import NodePath from 'node:path'
import Process from 'node:process'
import NodeURL from 'node:url'
import { parse } from 'yaml'

// Reports how completely each vendored OpenAPI spec describes its own contract.
//
// Two thirds of this site's routes are generated from the specs in public/swagger, so a
// gap in a spec is a gap in the published documentation. The failure mode that motivated
// this check is silence rather than error: an operation that omits `security` renders as
// an endpoint needing no credential, which is indistinguishable from one that genuinely
// needs none. Readers cannot tell the two apart, and neither can the build.
//
// The checks below are structural, not stylistic. Each corresponds to a way the rendered
// reference can mislead: an undeclared auth scheme, a response with no documented body, an
// unbounded string presented as if any input were accepted.
//
// IMPORTANT: a constraint added here is an assertion about the service that must be true.
// Nothing validates requests against these specs at runtime, so declaring `maxLength: 255`
// on a field the service accepts 4000 characters in does not harden anything -- it
// publishes a false contract, which is worse than the missing constraint. Constraints
// belong here only when derived from the implementation.
//
// Runs in report-only mode while the specs are brought up to standard; flip ENFORCING once
// the remaining gaps are either closed or recorded in ACCEPTED below. A rule that already
// holds in every spec goes into ENFORCED_RULES so it cannot regress in the meantime.
const ENFORCING = false
const ENFORCED_RULES = new Set(['closed-objects'])

// Gaps deliberately left open, kept visible rather than silently skipped. Each entry needs
// a reason that would still convince someone reading it a year from now.
const ACCEPTED = [
  {
    spec: 'auth_service.yaml',
    rule: 'request-body',
    reason: 'POST /api/auth/sign-out takes no body; a sign-out needs no payload.',
  },
  {
    spec: 'auth_service.yaml',
    rule: 'common-error-codes',
    reason:
      'The public informational endpoints (/health, both JWKS routes, validate-origin) ' +
      'document only the codes they can emit. None can 401/403/404: they are ' +
      '`security: []` routes that exist unconditionally (validate-origin 403s by design ' +
      'and keeps that). Neither /health nor validate-origin can 429: Better Auth rate ' +
      'limiting wraps /api/auth/* only and these are plain Express routes with no ' +
      'limiter. Documenting unreachable codes would be a false contract; every ' +
      'authenticated operation describes the full set.',
  },
]

const HTTP_METHODS = new Set(['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'])
const BODY_METHODS = new Set(['post', 'put', 'patch'])
// A 204 or 304 carries no payload by definition, so absent `content` is correct there.
const BODYLESS_CODES = new Set(['204', '304'])
// The error codes every operation should describe, so a caller can handle them
// without discovering each one in production.
const COMMON_CODES = ['401', '403', '404', '429', '500']

/** Follow a local `#/a/b/c` pointer. Returns the node unchanged when it is not a $ref. */
function deref(node, root, depth = 0) {
  if (typeof node !== 'object' || node === null || typeof node.$ref !== 'string') return node
  // A malformed or cyclic pointer should not hang the build.
  if (depth > 8 || !node.$ref.startsWith('#/')) return node
  let target = root
  for (const segment of node.$ref.slice(2).split('/')) {
    target = target?.[segment.replaceAll('~1', '/').replaceAll('~0', '~')]
    if (target === undefined) return node
  }
  return deref(target, root, depth + 1)
}

// Resolved from this file rather than cwd so the check behaves the same whether it
// is run by `pnpm build` from docs/ or invoked directly from the repository root.
const swaggerDir = NodePath.join(NodePath.dirname(NodeURL.fileURLToPath(import.meta.url)), '..', 'public', 'swagger')

/** Every [path, method, operation] triple in the document. */
function operationsOf(spec) {
  const found = []
  for (const [path, item] of Object.entries(spec.paths ?? {})) {
    if (typeof item !== 'object' || item === null) continue
    for (const [method, operation] of Object.entries(item)) {
      if (!HTTP_METHODS.has(method)) continue
      if (typeof operation !== 'object' || operation === null) continue
      found.push({ path, method, operation })
    }
  }
  return found
}

/** Every JSON-Schema-shaped node in the document, components included. */
function schemasOf(node, seen = new Set(), found = []) {
  if (typeof node !== 'object' || node === null || seen.has(node)) return found
  seen.add(node)
  if (Array.isArray(node)) {
    for (const entry of node) schemasOf(entry, seen, found)
    return found
  }
  if ('type' in node || 'properties' in node) found.push(node)
  for (const value of Object.values(node)) schemasOf(value, seen, found)
  return found
}

/** `satisfied of total`, where total 0 means the rule has nothing to judge and passes. */
function ratio(satisfied, total) {
  return { satisfied, total, ok: satisfied === total }
}

function checkOperationSecurity(spec, operations) {
  // Top-level `security` applies to every operation that does not override it, so an
  // operation inherits rather than fails. An endpoint meant to be public needs an explicit
  // `security: []` -- omitting the key entirely makes it inherit, which is the opposite.
  const inherits = Array.isArray(spec.security)
  const covered = operations.filter((o) => 'security' in o.operation || inherits)
  return ratio(covered.length, operations.length)
}

function checkResponseContent(spec, operations) {
  let total = 0
  let withContent = 0
  for (const { operation } of operations) {
    for (const [code, ref] of Object.entries(operation.responses ?? {})) {
      // A response is routinely a $ref into components.responses; the `content`
      // lives on the target, so comparing against the pointer would report every
      // reusable response as empty.
      const response = deref(ref, spec)
      if (typeof response !== 'object' || response === null) continue
      if (BODYLESS_CODES.has(String(code))) continue
      total += 1
      if (response.content && Object.keys(response.content).length > 0) withContent += 1
    }
  }
  return ratio(withContent, total)
}

/** Every operation should describe the common failure codes it can emit. */
function checkCommonCodes(operations) {
  let expected = 0
  let described = 0
  for (const { operation } of operations) {
    const codes = new Set(Object.keys(operation.responses ?? {}))
    for (const code of COMMON_CODES) {
      expected += 1
      if (codes.has(code)) described += 1
    }
  }
  return ratio(described, expected)
}

/**
 * OpenAPI requires operationIds to be unique across the document; a duplicate breaks
 * codegen and anchors silently. Only ids that are present are judged — presence itself
 * is not this rule's concern.
 */
function checkUniqueOperationIds(operations) {
  const counts = new Map()
  for (const { operation } of operations) {
    if (typeof operation.operationId !== 'string') continue
    counts.set(operation.operationId, (counts.get(operation.operationId) ?? 0) + 1)
  }
  const total = [...counts.values()].reduce((sum, n) => sum + n, 0)
  const unique = [...counts.values()].filter((n) => n === 1).length
  return ratio(unique, total)
}

function checkRequestBody(operations) {
  const relevant = operations.filter((o) => BODY_METHODS.has(o.method))
  return ratio(relevant.filter((o) => 'requestBody' in o.operation).length, relevant.length)
}

function checkSchemaKeyword(schemas, predicate, matches) {
  const relevant = schemas.filter(predicate)
  return ratio(relevant.filter(matches).length, relevant.length)
}

function evaluate(spec) {
  const operations = operationsOf(spec)
  const schemas = schemasOf(spec)
  const components = spec.components ?? {}
  const objects = (s) => s.type === 'object' || 'properties' in s

  return {
    'global-security': ratio(Array.isArray(spec.security) ? 1 : 0, 1),
    servers: ratio(Array.isArray(spec.servers) && spec.servers.length > 0 ? 1 : 0, 1),
    'components-responses': ratio(components.responses ? 1 : 0, 1),
    'components-parameters': ratio(components.parameters ? 1 : 0, 1),
    'operation-security': checkOperationSecurity(spec, operations),
    'response-content': checkResponseContent(spec, operations),
    'common-error-codes': checkCommonCodes(operations),
    'unique-operation-ids': checkUniqueOperationIds(operations),
    'request-body': checkRequestBody(operations),
    'default-response': ratio(
      operations.filter((o) => 'default' in (o.operation.responses ?? {})).length,
      operations.length,
    ),
    'closed-objects': checkSchemaKeyword(schemas, objects, (s) => s.additionalProperties === false),
    // An `enum` constrains a string more tightly than any `pattern` could, so it
    // satisfies this rule rather than needing a redundant regex alongside it.
    'string-pattern': checkSchemaKeyword(
      schemas,
      (s) => s.type === 'string',
      (s) => 'pattern' in s || Array.isArray(s.enum),
    ),
    'string-max-length': checkSchemaKeyword(
      schemas,
      (s) => s.type === 'string',
      (s) => 'maxLength' in s,
    ),
    'array-max-items': checkSchemaKeyword(
      schemas,
      (s) => s.type === 'array',
      (s) => 'maxItems' in s,
    ),
  }
}

const specFiles = NodeFS.readdirSync(swaggerDir)
  .filter((name) => name.endsWith('.yaml'))
  .sort()

if (specFiles.length === 0) {
  console.error(`check-openapi: no specs found in ${swaggerDir}`)
  Process.exit(1)
}

const isAccepted = (spec, rule) => ACCEPTED.some((a) => a.spec === spec && a.rule === rule)

const failures = []
const rows = []

for (const name of specFiles) {
  const spec = parse(NodeFS.readFileSync(NodePath.join(swaggerDir, name), 'utf8'))
  for (const [rule, result] of Object.entries(evaluate(spec))) {
    rows.push({ name, rule, result })
    if (result.ok || isAccepted(name, rule)) continue
    failures.push({ name, rule, result })
  }
}

const ruleWidth = Math.max(...rows.map((r) => r.rule.length))
for (const name of specFiles) {
  console.log(`\ncheck-openapi: ${name}`)
  for (const row of rows.filter((r) => r.name === name)) {
    const mark = row.result.ok ? 'ok  ' : isAccepted(name, row.rule) ? 'acc ' : 'GAP '
    console.log(`  ${mark} ${row.rule.padEnd(ruleWidth)}  ${row.result.satisfied}/${row.result.total}`)
  }
}

console.log(
  `\ncheck-openapi: ${failures.length} gap(s) across ${specFiles.length} spec(s)` +
    `${ACCEPTED.length > 0 ? `, ${ACCEPTED.length} accepted` : ''}`,
)

const enforced = ENFORCING ? failures : failures.filter((f) => ENFORCED_RULES.has(f.rule))
if (enforced.length > 0) {
  console.error('\ncheck-openapi: the specs above do not describe their own contract.')
  console.error('Close the gap, or add an entry to ACCEPTED with a reason.')
  Process.exit(1)
}

if (failures.length > enforced.length) {
  console.log('check-openapi: remaining gaps are report-only, not failing the build (see ENFORCING)')
}
