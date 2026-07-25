<script setup lang="ts">
import {
  type AtomicDraft,
  CONSTRAINT_COMBINATORS,
  type ConstraintNodeDraft,
  type GroupDraft,
  isGroupDraft,
  newAtomic,
  newGroup,
  type OperandDraftValue,
} from '@template-repository/components/clauses-editor/constraint-draft'
import { ODRL_CONTEXT_OPERANDS, ODRL_OPERATORS } from '@template-repository/utils/odrl-vocabulary'
import { resolveConstraintForLeftOperand } from '@template-repository/utils/value-constraint-catalog'
import {
  formatValueOption,
  groupValueOptions,
  resolveValueOptions,
} from '@template-repository/utils/value-option-catalog'

/**
 * Authors one ODRL constraint group — a combinator over child nodes, each an
 * atomic constraint or a nested group (recursion via the component's own name,
 * an arbitrarily deep constraint tree, ODRL IM §2.6). The rule and every duty
 * embed one root group.
 */

defineProps<{
  /** Fields offered as a constraint's left operand and negotiated boundary. */
  fields: { id: string; label: string }[]
  /** The title on this group's combinator select (targets the top-level one). */
  combineTitle?: string
}>()

const group = defineModel<GroupDraft>({ required: true })

function addConstraint() {
  group.value.children.push(
    newAtomic(ODRL_CONTEXT_OPERANDS[0]?.id ?? 'odrl:spatial', ODRL_OPERATORS[0]?.id ?? 'odrl:eq'),
  )
}
function addGroup() {
  group.value.children.push(newGroup())
}
function removeChild(index: number) {
  group.value.children.splice(index, 1)
}

// Narrows a child node to a group for the recursive editor. The editor mutates
// this same reactive object in place, so a one-way :model-value binding still
// propagates every edit up through the shared draft graph.
function childGroup(child: ConstraintNodeDraft): GroupDraft {
  return child as GroupDraft
}

function isSetOperator(operator: string): boolean {
  return operator === 'odrl:isAnyOf' || operator === 'odrl:isNoneOf' || operator === 'odrl:isAllOf'
}

function valueOptionsFor(child: AtomicDraft) {
  return resolveValueOptions(resolveConstraintForLeftOperand(child.leftOperand))
}

function valueOptionGroupsFor(child: AtomicDraft) {
  return groupValueOptions(valueOptionsFor(child))
}

function optionOperand(optionValue: string, child: AtomicDraft): OperandDraftValue {
  const option = valueOptionsFor(child).find((item) => item.value === optionValue || item.iri === optionValue)
  if (option?.iri) return { '@id': option.iri }
  return { '@value': optionValue, '@type': 'xsd:string' }
}

function operandKey(value: OperandDraftValue): string {
  return '@id' in value ? value['@id'] : String(value['@value'])
}

function selectedOptionValues(child: AtomicDraft): string[] {
  return child.values.map(operandKey)
}

function fixedValueFor(child: AtomicDraft): string {
  const [first] = child.values
  return first ? operandKey(first) : ''
}

function setSingleOption(child: AtomicDraft, event: Event) {
  const value = (event.target as HTMLSelectElement).value
  child.values = value ? [optionOperand(value, child)] : []
  child.value = ''
}

function toggleOption(child: AtomicDraft, optionValue: string) {
  const selected = new Set(selectedOptionValues(child))
  if (selected.has(optionValue)) selected.delete(optionValue)
  else selected.add(optionValue)
  child.values = valueOptionsFor(child)
    .map((option) => option.iri ?? option.value)
    .filter((value) => selected.has(value))
    .map((value) => optionOperand(value, child))
  child.value = ''
}

function clearFixedValues(child: AtomicDraft) {
  child.values = []
}

function resetFixedOperand(child: AtomicDraft) {
  child.value = ''
  child.values = []
}
</script>

<template>
  <div class="space-y-1">
    <div class="flex items-center gap-1">
      <select
        v-if="group.children.length > 1"
        v-model="group.combine"
        class="select-bordered select select-xs"
        :title="combineTitle ?? 'How this group combines'"
      >
        <option v-for="c in CONSTRAINT_COMBINATORS" :key="c.op" :value="c.op">{{ c.label }}</option>
      </select>
      <button type="button" class="btn btn-ghost btn-xs" @click="addConstraint">+ constraint</button>
      <button type="button" class="btn btn-ghost btn-xs" @click="addGroup">+ group</button>
    </div>

    <template v-for="(child, i) in group.children" :key="i">
      <!-- A nested group: recurse into this same editor, indented. -->
      <div v-if="isGroupDraft(child)" class="ml-3 space-y-1 rounded border border-dashed border-base-300 p-1">
        <div class="flex items-center justify-between">
          <span class="label-text text-2xs opacity-60">group</span>
          <button type="button" class="btn btn-ghost btn-xs" @click="removeChild(i)">✕</button>
        </div>
        <ConstraintGroupEditor :model-value="childGroup(child)" :fields="fields" />
      </div>

      <!-- An atomic constraint row. -->
      <div v-else class="flex flex-wrap items-center gap-1">
        <select v-model="child.leftOperand" class="select-bordered select select-xs" @change="resetFixedOperand(child)">
          <optgroup v-if="fields.length" label="Data fields">
            <option v-for="f in fields" :key="f.id" :value="f.id">{{ f.label }}</option>
          </optgroup>
          <optgroup label="Access context">
            <option v-for="o in ODRL_CONTEXT_OPERANDS" :key="o.id" :value="o.id">{{ o.label }}</option>
          </optgroup>
        </select>
        <select v-model="child.operator" class="select-bordered select select-xs">
          <option v-for="op in ODRL_OPERATORS" :key="op.id" :value="op.id">{{ op.label }}</option>
        </select>
        <select v-model="child.rightSource" class="select-bordered select select-xs" title="What the boundary is">
          <option value="">a fixed value</option>
          <optgroup v-if="fields.length" label="Agreed at negotiation">
            <option v-for="f in fields" :key="f.id" :value="f.id">the “{{ f.label }}”</option>
          </optgroup>
        </select>
        <input
          v-if="!child.rightSource && !valueOptionsFor(child).length"
          v-model="child.value"
          type="text"
          placeholder="value"
          class="input-bordered input input-xs w-28"
          @input="clearFixedValues(child)"
        />
        <select
          v-else-if="!child.rightSource && valueOptionsFor(child).length && !isSetOperator(child.operator)"
          :value="fixedValueFor(child)"
          class="select-bordered select min-w-36 select-xs"
          @change="setSingleOption(child, $event)"
        >
          <option value="">choose value</option>
          <option
            v-for="option in valueOptionsFor(child)"
            :key="option.iri ?? option.value"
            :value="option.iri ?? option.value"
            :selected="fixedValueFor(child) === (option.iri ?? option.value)"
          >
            {{ formatValueOption(option.iri ?? option.value, valueOptionsFor(child)) }}
          </option>
        </select>
        <details v-else-if="!child.rightSource" data-testid="constraint-value-multiselect" class="dropdown max-w-full">
          <summary class="btn min-w-36 btn-outline btn-xs">
            {{ child.values.length ? `${child.values.length} selected` : 'choose values' }}
          </summary>
          <div
            class="dropdown-content z-10 mt-1 max-h-64 w-64 max-w-[calc(100vw-2rem)] overflow-auto rounded-box border border-base-content/10 bg-base-100 p-2 shadow"
          >
            <fieldset v-for="catalog in valueOptionGroupsFor(child)" :key="catalog.iri || 'values'">
              <legend v-if="catalog.label" class="px-2 py-1 text-xs font-semibold opacity-70">
                {{ catalog.label }}
              </legend>
              <label
                v-for="option in catalog.options"
                :key="option.iri ?? option.value"
                class="flex min-h-8 items-center gap-2 rounded px-2 hover:bg-base-200"
              >
                <input
                  type="checkbox"
                  class="checkbox checkbox-sm checkbox-primary"
                  :value="option.iri ?? option.value"
                  :checked="selectedOptionValues(child).includes(option.iri ?? option.value)"
                  @change="toggleOption(child, option.iri ?? option.value)"
                />
                <span class="text-sm">
                  {{ formatValueOption(option.iri ?? option.value, valueOptionsFor(child)) }}
                </span>
              </label>
            </fieldset>
          </div>
        </details>
        <button type="button" class="btn btn-ghost btn-xs" @click="removeChild(i)">✕</button>
      </div>
    </template>
  </div>
</template>
