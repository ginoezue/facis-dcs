/**
 * dcsDraftStore — Pinia store for dcs:ContractTemplate JSON-LD documents.
 *
 * State IS the JSON-LD document.  No pointer arrays, no conditionId joins.
 * Constraints live inside the clause they constrain (odrl:policy).
 */
import { defineStore } from 'pinia'
import type {
  DcsTemplateData,
  DcsSection,
  DcsClause,
  DcsTextBlock,
  DcsParty,
  OdrlSet,
  OdrlRule,
  OdrlConstraint,
} from '@/models/dcs-jsonld'
import {
  DCS_JSONLD_CONTEXT,
  emptyDcsTemplate,
  isDcsClause,
  makeSectionId,
  makePolicyId,
  clauseConstraints,
} from '@/models/dcs-jsonld'
import type { ContractTemplateState } from '@/types/contract-template-state'
import type { TemplateType } from '@/types/template-type'
import type { ContractTemplateResponsible } from '@/models/contract-template-responsible'

// ── State ─────────────────────────────────────────────────────────────────────

interface DcsDraftState {
  // JSON-LD document fields
  '@id': string | null
  'dcs:title': string
  'dcs:templateType': string
  'dcterms:language': string
  'dcs:parties': DcsParty[]
  'dcs:sections': DcsSection[]

  // Relational fields (DB row metadata — not part of the JSON-LD document itself)
  did: string | null
  name: string
  description: string
  version: number | null
  updated_at: string | null
  created_by: string
  state: ContractTemplateState | null
  document_number: string | null
  template_type: TemplateType | null
  responsible: ContractTemplateResponsible | null
  workflow: 'template' | 'contract'
}

function initialState(): DcsDraftState {
  const doc = emptyDcsTemplate()
  return {
    '@id': null,
    'dcs:title': '',
    'dcs:templateType': 'dcs:SubContract',
    'dcterms:language': 'de',
    'dcs:parties': [...doc['dcs:parties']],
    'dcs:sections': [],

    did: null,
    name: '',
    description: '',
    version: null,
    updated_at: null,
    created_by: '',
    state: null,
    document_number: null,
    template_type: null,
    responsible: null,
    workflow: 'template',
  }
}

// ── Store ─────────────────────────────────────────────────────────────────────

export const useDcsDraftStore = defineStore('dcsDraft', {
  state: (): DcsDraftState => initialState(),

  getters: {
    // ── Document assembly ──────────────────────────────────────────────────
    /** The full dcs:ContractTemplate JSON-LD document, ready for the API. */
    templateDocument(state): DcsTemplateData {
      return {
        '@context': DCS_JSONLD_CONTEXT,
        '@type': 'dcs:ContractTemplate',
        ...(state['@id'] ? { '@id': state['@id'] } : {}),
        'dcs:title': state['dcs:title'],
        'dcs:templateType': state['dcs:templateType'],
        'dcterms:language': state['dcterms:language'],
        'dcs:parties': state['dcs:parties'],
        'dcs:sections': state['dcs:sections'],
      }
    },

    // ── Section accessors ─────────────────────────────────────────────────
    clauses(state): DcsClause[] {
      return state['dcs:sections'].filter(isDcsClause)
    },

    textBlocks(state): DcsTextBlock[] {
      return state['dcs:sections'].filter((s): s is DcsTextBlock => s['@type'] === 'dcs:TextBlock')
    },

    hasId(state): boolean {
      return !!state.did
    },

    /** All constraints across all clauses — useful for validation. */
    allConstraints(): OdrlConstraint[] {
      return this.clauses.flatMap(clauseConstraints)
    },

    /** All placeholders declared across all clause constraints. */
    allPlaceholders(): Set<string> {
      const set = new Set<string>()
      for (const c of this.allConstraints) {
        if (c['dcs:placeholder']) set.add(c['dcs:placeholder'])
      }
      return set
    },
  },

  actions: {
    // ── Lifecycle ─────────────────────────────────────────────────────────

    reset() {
      Object.assign(this, initialState())
    },

    /** Load a dcs:ContractTemplate document from the API into store state. */
    loadDocument(doc: DcsTemplateData, meta: {
      did: string
      name?: string
      description?: string
      version: number
      updated_at: string
      created_by: string
      state: ContractTemplateState
      document_number?: string
      template_type?: TemplateType
      responsible?: ContractTemplateResponsible
    }) {
      this['@id'] = doc['@id'] ?? meta.did
      this['dcs:title'] = doc['dcs:title'] ?? meta.name ?? ''
      this['dcs:templateType'] = doc['dcs:templateType'] ?? 'dcs:SubContract'
      this['dcterms:language'] = doc['dcterms:language'] ?? 'de'
      this['dcs:parties'] = doc['dcs:parties'] ?? []
      this['dcs:sections'] = doc['dcs:sections'] ?? []

      this.did = meta.did
      this.name = meta.name ?? ''
      this.description = meta.description ?? ''
      this.version = meta.version
      this.updated_at = meta.updated_at
      this.created_by = meta.created_by
      this.state = meta.state
      this.document_number = meta.document_number ?? null
      this.template_type = meta.template_type ?? null
      this.responsible = meta.responsible ?? null
    },

    // ── Parties ───────────────────────────────────────────────────────────

    updateParty(id: string, patch: Partial<DcsParty>) {
      const idx = this['dcs:parties'].findIndex((p) => p['@id'] === id)
      if (idx < 0) return
      const existing = this['dcs:parties'][idx]!
      this['dcs:parties'][idx] = { ...existing, ...patch, '@type': 'odrl:Party', '@id': patch['@id'] ?? existing['@id'] }
    },

    // ── Sections (text blocks) ────────────────────────────────────────────

    addTextBlock(content: string): string {
      const id = makeSectionId()
      const block: DcsTextBlock = { '@type': 'dcs:TextBlock', '@id': id, 'dcs:content': content }
      this['dcs:sections'].push(block)
      return id
    },

    updateTextBlock(id: string, content: string) {
      const section = this['dcs:sections'].find((s) => s['@id'] === id)
      if (section?.['@type'] === 'dcs:TextBlock') section['dcs:content'] = content
    },

    // ── Sections (clauses) ────────────────────────────────────────────────

    /**
     * Add a clause with an embedded odrl:Set policy.
     * @param title  Human-readable clause title.
     * @param content  Prose text with {{placeholder}} markers.
     * @param obligation  ODRL obligation rule(s) — constraints carry dcs:placeholder.
     * @param entityType  Ontology class IRI this clause is about (e.g. "sla:ServiceLevelObjective").
     */
    addClause(payload: {
      title?: string
      content: string
      obligation?: OdrlRule[]
      permission?: OdrlRule[]
      prohibition?: OdrlRule[]
      entityType?: string
    }): string {
      const id = makeSectionId()
      const policy: OdrlSet = {
        '@type': 'odrl:Set',
        '@id': makePolicyId(),
        ...(payload.permission?.length ? { 'odrl:permission': payload.permission } : {}),
        ...(payload.prohibition?.length ? { 'odrl:prohibition': payload.prohibition } : {}),
        ...(payload.obligation?.length ? { 'odrl:obligation': payload.obligation } : {}),
      }
      const clause: DcsClause = {
        '@type': 'dcs:Clause',
        '@id': id,
        'dcs:content': payload.content,
        'odrl:policy': policy,
        ...(payload.title ? { 'dcs:title': payload.title } : {}),
        ...(payload.entityType ? { 'dcs:entityType': { '@id': payload.entityType } } : {}),
      }
      this['dcs:sections'].push(clause)
      return id
    },

    updateClause(id: string, patch: Partial<Pick<DcsClause, 'dcs:title' | 'dcs:content' | 'dcs:entityType'>>) {
      const section = this['dcs:sections'].find((s) => s['@id'] === id)
      if (section?.['@type'] !== 'dcs:Clause') return
      Object.assign(section, patch)
    },

    /** Replace the entire odrl:policy on a clause (e.g. after constraint editor saves). */
    setClausePolicy(id: string, policy: OdrlSet) {
      const section = this['dcs:sections'].find((s) => s['@id'] === id)
      if (section?.['@type'] !== 'dcs:Clause') return
      section['odrl:policy'] = policy
    },

    /** Add / update a single constraint within an existing obligation rule.
     *  If no obligation rule exists, one is created. */
    upsertConstraint(clauseId: string, constraint: OdrlConstraint) {
      const section = this['dcs:sections'].find((s) => s['@id'] === clauseId)
      if (section?.['@type'] !== 'dcs:Clause') return
      const policy = section['odrl:policy'] as OdrlSet

      let obligations = policy['odrl:obligation']
      if (!obligations?.length) {
        const rule: OdrlRule = {
          '@type': 'odrl:Rule',
          'odrl:action': { '@id': 'odrl:use' },
          'odrl:constraint': [constraint],
        }
        policy['odrl:obligation'] = [rule]
        return
      }

      const rule = obligations[0]!
      const existing = rule['odrl:constraint'] ?? []
      const idx = existing.findIndex(
        (c) => c['dcs:placeholder'] === constraint['dcs:placeholder'],
      )
      if (idx >= 0) {
        existing[idx] = constraint
      } else {
        existing.push(constraint)
      }
      rule['odrl:constraint'] = existing
    },

    removeConstraint(clauseId: string, placeholder: string) {
      const section = this['dcs:sections'].find((s) => s['@id'] === clauseId)
      if (section?.['@type'] !== 'dcs:Clause') return
      const policy = section['odrl:policy'] as OdrlSet
      for (const rule of [...(policy['odrl:obligation'] ?? []), ...(policy['odrl:permission'] ?? []), ...(policy['odrl:prohibition'] ?? [])]) {
        rule['odrl:constraint'] = (rule['odrl:constraint'] ?? []).filter(
          (c) => c['dcs:placeholder'] !== placeholder,
        )
      }
    },

    // ── Section ordering ──────────────────────────────────────────────────

    moveSection(id: string, direction: 'up' | 'down') {
      const sections = this['dcs:sections']
      const idx = sections.findIndex((s) => s['@id'] === id)
      if (idx < 0) return
      const targetIdx = direction === 'up' ? idx - 1 : idx + 1
      if (targetIdx < 0 || targetIdx >= sections.length) return
      const [item] = sections.splice(idx, 1)
      sections.splice(targetIdx, 0, item!)
    },

    deleteSection(id: string) {
      this['dcs:sections'] = this['dcs:sections'].filter((s) => s['@id'] !== id)
    },

    // ── Basic info ────────────────────────────────────────────────────────

    updateTitle(title: string) {
      this['dcs:title'] = title
      this.name = title
    },

    updateDescription(description: string) {
      this.description = description
    },

    updateTemplateType(type: string) {
      this['dcs:templateType'] = type
    },
  },
})
