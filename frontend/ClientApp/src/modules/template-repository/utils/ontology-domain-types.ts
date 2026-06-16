import type {
  DomainFieldDefinition,
  SemanticEntityRole,
  SemanticValueConstraint,
} from '@/modules/template-repository/models/contract-template'
import { ONTOLOGY_DOMAIN_FIELDS, ONTOLOGY_ENTITY_ROLES, ONTOLOGY_ENTITY_TYPES } from './ontology-domain-fields'

/** Parameter descriptor used for building ODRL constraints in JSON-LD clauses. */
export interface OntologyConstraintParam {
  /** Compact IRI for odrl:leftOperand, e.g. "dcst:field-company-legalName". */
  iri: string
  /** Short placeholder name for {{placeholder}} in prose and dcs:placeholder. */
  placeholder: string
  type: DomainFieldDefinition['type']
  label: string
  valueConstraint?: DomainFieldDefinition['valueConstraint']
  isRequired: boolean
}

export interface OntologyDomainType {
  id: string
  label: string
  entityType: string
  roleRequired: boolean
  fields: readonly DomainFieldDefinition[]
}

export const ontologyRoleOptions = ONTOLOGY_ENTITY_ROLES

export const ONTOLOGY_DOMAIN_TYPES: readonly OntologyDomainType[] = buildOntologyDomainTypes()
export const ONTOLOGY_DOMAIN_TYPE_FIELD_IRIS: ReadonlySet<string> = new Set(
  ONTOLOGY_DOMAIN_TYPES.flatMap((domainType) => domainType.fields.map((field) => field.iri)),
)

/** @deprecated Use ONTOLOGY_DOMAIN_TYPE_FIELD_IRIS. Kept for legacy SemanticRuleForm compatibility. */
export const ONTOLOGY_DOMAIN_TYPE_FIELD_PATHS: ReadonlySet<string> = new Set(
  ONTOLOGY_DOMAIN_TYPES.flatMap((domainType) => domainType.fields.map((field) => field.semanticPath)),
)

export function buildOntologyDomainTypeParameters(domainType: OntologyDomainType): OntologyConstraintParam[] {
  return domainType.fields.map((field) => ({
    iri: field.iri,
    placeholder: iriToPlaceholder(field.iri),
    type: field.type,
    label: field.label,
    valueConstraint: cloneValueConstraint(field.valueConstraint),
    isRequired: true,
  }))
}

export function buildOntologyDomainTypeClauseText(
  domainType: OntologyDomainType,
  role?: SemanticEntityRole,
): string {
  const roleLabel = role ? roleLabelFor(role) : ''
  const title = roleLabel ? `${roleLabel} ${domainType.label}` : domainType.label
  const fieldLines = domainType.fields.map((field) => buildDomainTypeClauseFieldLine(field))
  return [title, '', ...fieldLines].join('\n')
}

export function roleLabelFor(role: SemanticEntityRole): string {
  return ONTOLOGY_ENTITY_ROLES.find((option) => option.value === role)?.label ?? role
}

function buildDomainTypeClauseFieldLine(field: DomainFieldDefinition): string {
  return `${field.label}: {{${iriToPlaceholder(field.iri)}}}`
}

/** Converts a compact IRI to a camelCase placeholder name.
 *  e.g. "dcst:field-company-legalName" → "companyLegalName" */
export function iriToPlaceholder(iri: string): string {
  const local = iri.replace(/^[^:]+:/, '').replace(/^field-/, '')
  return local.replace(/-([a-z])/g, (_, c: string) => c.toUpperCase())
}

function buildOntologyDomainTypes(): OntologyDomainType[] {
  const domainTypes: OntologyDomainType[] = []
  for (const entityType of ONTOLOGY_ENTITY_TYPES) {
    const fields = fieldsForEntityType(entityType.value)
    if (!fields.length) continue
    domainTypes.push({
      id: entityType.value,
      label: entityType.label,
      entityType: entityType.value,
      roleRequired: fields.some((field) => field.iri.includes('-company-')),
      fields,
    })
  }
  return domainTypes
}

function fieldsForEntityType(entityType: string): DomainFieldDefinition[] {
  const directlyTyped = ONTOLOGY_DOMAIN_FIELDS.filter(
    (field) => localOntologyName(field.statementType ?? '') === entityType,
  )
  const iriPrefixes = new Set(directlyTyped.map((field) => iriGroupPrefix(field.iri)))
  if (!iriPrefixes.size) return []
  return ONTOLOGY_DOMAIN_FIELDS.filter((field) => iriPrefixes.has(iriGroupPrefix(field.iri))).sort(
    (left, right) => left.label.localeCompare(right.label),
  )
}

/** Returns the first two dash-separated segments of the IRI local name after stripping "field-".
 *  e.g. "dcst:field-company-legalName" → "company" */
function iriGroupPrefix(iri: string): string {
  const local = iri.replace(/^[^:]+:/, '').replace(/^field-/, '')
  return local.split('-', 1)[0] ?? local
}

function localOntologyName(resource: string): string {
  return resource.replace(/^.*[:#/]/, '')
}

function cloneValueConstraint(constraint?: SemanticValueConstraint): SemanticValueConstraint | undefined {
  if (!constraint) return undefined
  return {
    ...constraint,
    allowedValues: constraint.allowedValues ? [...constraint.allowedValues] : undefined,
  }
}
