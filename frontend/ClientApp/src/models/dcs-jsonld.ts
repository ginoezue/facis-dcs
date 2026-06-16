/**
 * DCS JSON-LD document model.
 *
 * These types mirror the JSON-LD structure stored in template_data / contract_data
 * for documents with @type dcs:ContractTemplate or dcs:Contract.
 *
 * Design rules:
 *  - No pointer arrays (documentOutline / conditionIds).  Constraints live
 *    inside the clause they constrain via odrl:policy.
 *  - {{placeholder}} in dcs:content is the human-readable binding.
 *    odrl:Constraint.dcs:placeholder is the machine-readable binding.
 *  - Templates use odrl:Set (no bound parties).
 *    Contracts use odrl:Agreement (bound parties, resolved values).
 */

// ── Context ───────────────────────────────────────────────────────────────────

export const DCS_JSONLD_CONTEXT = {
  dcs:     'https://w3id.org/facis/dcs#',
  odrl:    'http://www.w3.org/ns/odrl/2/',
  sla:     'https://w3id.org/facis/sla/ontology#',
  xsd:     'http://www.w3.org/2001/XMLSchema#',
  dcterms: 'http://purl.org/dc/terms/',
} as const

// ── Shared primitives ─────────────────────────────────────────────────────────

/** JSON-LD IRI reference: { "@id": "..." } */
export interface IriRef {
  '@id': string
}

/** JSON-LD typed literal: { "@value": "...", "@type": "xsd:decimal" } */
export interface TypedLiteral {
  '@value': string
  '@type': string
}

// ── ODRL ─────────────────────────────────────────────────────────────────────

export type OdrlOperatorIri =
  | 'odrl:eq'
  | 'odrl:neq'
  | 'odrl:gt'
  | 'odrl:gteq'
  | 'odrl:lt'
  | 'odrl:lteq'
  | 'odrl:isPartOf'
  | 'odrl:hasPart'
  | (string & {})

export type OdrlParameterType = 'string' | 'decimal' | 'integer' | 'boolean' | 'date' | 'enum'

/**
 * An ODRL Constraint extended with DCS builder metadata.
 * The dcs:placeholder links the constraint to a {{placeholder}} in dcs:content.
 */
export interface OdrlConstraint {
  '@type': 'odrl:Constraint'
  'odrl:leftOperand': IriRef
  'odrl:operator': IriRef
  'odrl:rightOperand': TypedLiteral | string

  /** Maps to {{placeholder}} in dcs:content. Undefined when value is resolved (contract). */
  'dcs:placeholder'?: string
  'dcs:parameterType'?: OdrlParameterType
  'dcs:isRequired'?: boolean
  'dcs:uiLabel'?: string
  'dcs:uiDescription'?: string
  'dcs:uiInput'?: 'text' | 'number' | 'date' | 'select'
  'dcs:allowedValues'?: string[]
}

export interface OdrlRule {
  '@type': 'odrl:Rule'
  'odrl:assigner'?: IriRef
  'odrl:assignee'?: IriRef
  'odrl:action': IriRef
  'odrl:target'?: IriRef
  'odrl:constraint'?: OdrlConstraint[]
}

/** odrl:Set — used in templates (parties not yet bound). */
export interface OdrlSet {
  '@type': 'odrl:Set'
  '@id'?: string
  'odrl:permission'?: OdrlRule[]
  'odrl:prohibition'?: OdrlRule[]
  'odrl:obligation'?: OdrlRule[]
}

/** odrl:Agreement — used in contracts (parties bound, values resolved). */
export interface OdrlAgreement {
  '@type': 'odrl:Agreement'
  '@id'?: string
  'odrl:assigner': IriRef
  'odrl:assignee': IriRef
  'odrl:permission'?: OdrlRule[]
  'odrl:prohibition'?: OdrlRule[]
  'odrl:obligation'?: OdrlRule[]
}

// ── Parties ───────────────────────────────────────────────────────────────────

export interface DcsParty {
  '@type': 'odrl:Party'
  '@id': string
  'dcs:label'?: string
  /** In templates: role IRI (e.g. "dcs:role:provider"). In contracts: org DID. */
  'dcs:role'?: string
}

// ── Sections ──────────────────────────────────────────────────────────────────

export interface DcsTextBlock {
  '@type': 'dcs:TextBlock'
  '@id': string
  'dcs:content': string
}

export interface DcsClause {
  '@type': 'dcs:Clause'
  '@id': string
  'dcs:title'?: string
  'dcs:content': string
  /** Ontology class this clause is about (e.g. sla:ServiceLevelObjective). */
  'dcs:entityType'?: IriRef
  /** Link back to template clause (only on contract sections). */
  'dcs:derivedFrom'?: IriRef
  'odrl:policy': OdrlSet | OdrlAgreement
}

export type DcsSection = DcsTextBlock | DcsClause

// ── Template document ─────────────────────────────────────────────────────────

export interface DcsTemplateData {
  '@context': typeof DCS_JSONLD_CONTEXT
  '@type': 'dcs:ContractTemplate'
  '@id'?: string
  'dcs:title'?: string
  'dcs:templateType'?: string
  'dcterms:language'?: string
  'dcs:parties': DcsParty[]
  'dcs:sections': DcsSection[]
}

// ── Contract document ─────────────────────────────────────────────────────────

export interface DcsContractData {
  '@context': typeof DCS_JSONLD_CONTEXT
  '@type': 'dcs:Contract'
  '@id'?: string
  'dcs:derivedFrom'?: IriRef
  'dcs:parties': DcsParty[]
  'dcs:sections': DcsSection[]
}

// ── Type guards ───────────────────────────────────────────────────────────────

export function isDcsTemplateData(data: unknown): data is DcsTemplateData {
  return (
    typeof data === 'object' &&
    data !== null &&
    (data as Record<string, unknown>)['@type'] === 'dcs:ContractTemplate'
  )
}

export function isDcsContractData(data: unknown): data is DcsContractData {
  return (
    typeof data === 'object' &&
    data !== null &&
    (data as Record<string, unknown>)['@type'] === 'dcs:Contract'
  )
}

export function isDcsClause(section: DcsSection): section is DcsClause {
  return section['@type'] === 'dcs:Clause'
}

export function isDcsTextBlock(section: DcsSection): section is DcsTextBlock {
  return section['@type'] === 'dcs:TextBlock'
}

// ── Builder helpers ───────────────────────────────────────────────────────────

export function makeSectionId(): string {
  return `did:facis:section:${crypto.randomUUID()}`
}

export function makePolicyId(): string {
  return `did:facis:policy:${crypto.randomUUID()}`
}

/** Returns all odrl:Constraint objects from a clause's policy (all rule types). */
export function clauseConstraints(clause: DcsClause): OdrlConstraint[] {
  const policy = clause['odrl:policy']
  return [
    ...(policy['odrl:permission']?.flatMap((r) => r['odrl:constraint'] ?? []) ?? []),
    ...(policy['odrl:prohibition']?.flatMap((r) => r['odrl:constraint'] ?? []) ?? []),
    ...(policy['odrl:obligation']?.flatMap((r) => r['odrl:constraint'] ?? []) ?? []),
  ]
}

/** Returns placeholders declared in dcs:content as a Set of strings. */
export function contentPlaceholders(content: string): Set<string> {
  const set = new Set<string>()
  for (const m of content.matchAll(/\{\{([^}]+)\}\}/g)) {
    set.add(m[1] ?? '')
  }
  return set
}

/** Empty DcsTemplateData ready for the builder. */
export function emptyDcsTemplate(): DcsTemplateData {
  return {
    '@context': DCS_JSONLD_CONTEXT,
    '@type': 'dcs:ContractTemplate',
    'dcs:parties': [
      { '@type': 'odrl:Party', '@id': 'dcs:role:provider', 'dcs:label': 'Auftragnehmer' },
      { '@type': 'odrl:Party', '@id': 'dcs:role:customer', 'dcs:label': 'Auftraggeber' },
    ],
    'dcs:sections': [],
  }
}
