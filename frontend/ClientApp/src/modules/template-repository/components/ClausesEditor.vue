<template>
  <div class="space-y-6">
    <!-- Section 1: New clause -->
    <section v-if="uiStore.isTemplateEditable" class="rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm">
      <ClauseEditorForm
        mode="create"
        :initial-title="newClauseTitle"
        :initial-text="newClauseText"
        :semantic-conditions="[]"
        @submit="addClause"
      />
    </section>

    <!-- Section 2: Existing clauses -->
    <section class="rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm">
      <h3 class="mb-4 text-sm font-semibold text-base-content/80">Existing clauses</h3>
      <ExistingClausesList
        :clause-blocks="clauseBlocksForLegacyList"
        :semantic-conditions="[]"
        :block-ids-in-outline="new Set()"
        :editing-block-id="editingBlockId"
        :editable="uiStore.isTemplateEditable"
        @delete="deleteClause"
        @edit="startEditClause"
        @save="saveEditedClause"
        @cancel-edit="cancelEdit"
      />

      <div v-if="uiStore.isTemplateEditable && ontologyDomainTypes.length" class="mt-5 border-t border-base-300 pt-4">
        <h4 class="mb-3 text-xs font-semibold text-base-content/50 uppercase">Domain types</h4>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <div
            v-for="domainType in ontologyDomainTypes"
            :key="domainType.id"
            class="group grid min-h-[88px] grid-cols-1 items-center gap-3 rounded-lg border border-base-300 bg-base-100 px-3 py-3 shadow-sm transition-all hover:border-primary/50 hover:bg-base-200 hover:shadow md:grid-cols-[minmax(0,1fr)_minmax(11rem,14rem)_auto]"
          >
            <div class="min-w-0">
              <span class="block text-sm font-medium text-base-content">{{ domainType.label }}</span>
              <span class="block text-xs text-base-content/50">{{ domainType.fields.length }} domain fields</span>
            </div>
            <label v-if="domainType.roleRequired" class="mx-auto w-full max-w-56">
              <span class="label-text mb-1 block text-xs text-base-content/60">Contract role</span>
              <select
                v-model="selectedDomainTypeRoles[domainType.id]"
                class="select-bordered select w-full select-xs text-left"
              >
                <option value="">Select role</option>
                <option v-for="option in roleOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>
            <span v-else class="hidden md:block" />
            <button
              type="button"
              class="btn justify-self-start transition-transform btn-xs btn-secondary group-hover:translate-x-0.5 md:justify-self-end"
              :disabled="domainType.roleRequired && !selectedDomainTypeRoles[domainType.id]"
              @click="describeNewClauseFromDomainType(domainType.id)"
            >
              Use
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import ExistingClausesList from '@template-repository/components/clauses-editor/ExistingClausesList.vue'
import ClauseEditorForm from '@template-repository/components/clauses-editor/ClauseEditorForm.vue'
import { useTemplateEditorUiStore } from '@template-repository/store/templateEditorUiStore'
import { useDcsDraftStore } from '@template-repository/store/dcsDraftStore'
import {
  ONTOLOGY_DOMAIN_TYPES,
  buildOntologyDomainTypeClauseText,
  buildOntologyDomainTypeParameters,
  ontologyRoleOptions,
  roleLabelFor,
} from '@template-repository/utils/ontology-domain-types'
import type { ClauseBlock } from '@template-repository/models/contract-template'
import type { DcsClause, OdrlConstraint, OdrlRule } from '@/models/dcs-jsonld'
import { clauseConstraints } from '@/models/dcs-jsonld'

const store = useDcsDraftStore()
const uiStore = useTemplateEditorUiStore()

const editingBlockId = ref<string | null>(null)
const selectedDomainTypeRoles = ref<Record<string, string>>({})
const newClauseTitle = ref('')
const newClauseText = ref('')
const ontologyDomainTypes = ONTOLOGY_DOMAIN_TYPES
const roleOptions = ontologyRoleOptions

/**
 * Transform DcsClause[] to ClauseBlock[] so ExistingClausesList can render
 * without changes.  conditionIds is derived from constraint placeholders.
 */
const clauseBlocksForLegacyList = computed((): ClauseBlock[] =>
  store.clauses.map((clause: DcsClause): ClauseBlock => ({
    blockId: clause['@id'],
    type: 'CLAUSE',
    text: clause['dcs:content'],
    title: clause['dcs:title'],
    conditionIds: clauseConstraints(clause)
      .map((c) => c['dcs:placeholder'])
      .filter((p): p is string => !!p),
  })),
)

function addClause(payload: { title: string; text: string }) {
  if (!payload.text.trim()) return
  store.addClause({
    title: payload.title.trim() || undefined,
    content: payload.text,
  })
  newClauseTitle.value = ''
  newClauseText.value = ''
}

function startEditClause(blockId: string) {
  editingBlockId.value = blockId
}

function cancelEdit() {
  editingBlockId.value = null
}

function saveEditedClause(payload: { blockId: string; title: string; text: string }) {
  if (!payload.text.trim()) return
  store.updateClause(payload.blockId, {
    'dcs:title': payload.title.trim() || undefined,
    'dcs:content': payload.text,
  })
  if (editingBlockId.value === payload.blockId) cancelEdit()
}

function deleteClause(blockId: string) {
  store.deleteSection(blockId)
  if (editingBlockId.value === blockId) cancelEdit()
}

function describeNewClauseFromDomainType(domainTypeId: string) {
  const domainType = ontologyDomainTypes.find((item) => item.id === domainTypeId)
  if (!domainType) return
  const role = domainType.roleRequired ? selectedDomainTypeRoles.value[domainType.id] ?? '' : ''
  if (domainType.roleRequired && !role) return

  const roleLabel = role ? roleLabelFor(role) : ''
  const title = roleLabel ? `${roleLabel} ${domainType.label}` : domainType.label
  const parameters = buildOntologyDomainTypeParameters(domainType)
  const text = buildOntologyDomainTypeClauseText(domainType, role)

  // Build ODRL obligation from domain type parameters.
  const constraints: OdrlConstraint[] = parameters.map((p) => ({
    '@type': 'odrl:Constraint',
    'odrl:leftOperand': { '@id': p.iri },
    'odrl:operator': { '@id': 'odrl:eq' },
    'odrl:rightOperand': { '@value': '', '@type': 'xsd:string' },
    'dcs:placeholder': p.placeholder,
    'dcs:parameterType': p.type,
    'dcs:isRequired': p.isRequired,
    'dcs:uiLabel': p.label,
  }))

  const obligation: OdrlRule[] = constraints.length
    ? [{
        '@type': 'odrl:Rule',
        'odrl:action': { '@id': 'odrl:use' },
        ...(role ? { 'odrl:assignee': { '@id': `dcs:role:${role}` } } : {}),
        'odrl:constraint': constraints,
      }]
    : []

  store.addClause({
    title,
    content: text,
    entityType: domainType.entityType,
    obligation,
  })

  newClauseTitle.value = ''
  newClauseText.value = ''
}
</script>
