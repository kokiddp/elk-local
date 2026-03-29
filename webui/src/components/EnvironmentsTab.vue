<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EnvironmentView } from '../api'
import { environmentStateSummary, formatUpdatedAt, statusLabel } from '../lib/display'
import EnvironmentCard from './EnvironmentCard.vue'

const props = defineProps<{
  environments: EnvironmentView[]
  isLoading: boolean
}>()

const emit = defineEmits<{
  'environment-updated': [environment: EnvironmentView]
  'environment-removed': [name: string]
  notify: [payload: { type: 'success' | 'error'; message: string }]
}>()

const filterOptions = [
  { id: 'all', label: 'All' },
  { id: 'running', label: 'Running' },
  { id: 'attention', label: 'Attention' },
  { id: 'offline', label: 'Offline' },
  { id: 'xdebug', label: 'Xdebug' },
] as const

type EnvironmentFilter = (typeof filterOptions)[number]['id']
type SortMode = 'status' | 'updated' | 'name'

const selectedEnvironmentName = ref('')
const searchQuery = ref('')
const activeFilter = ref<EnvironmentFilter>('all')
const sortMode = ref<SortMode>('status')

const statusCounts = computed(() => {
  return props.environments.reduce(
    (counts, environment) => {
      counts.all += 1

      if (environment.status.state === 'running') {
        counts.running += 1
      }

      if (environment.status.state === 'stopped') {
        counts.offline += 1
      }

      if (environment.status.state !== 'running' && environment.status.state !== 'stopped') {
        counts.attention += 1
      }

      if (environment.tooling.xdebug.enabled) {
        counts.xdebug += 1
      }

      return counts
    },
    {
      all: 0,
      running: 0,
      attention: 0,
      offline: 0,
      xdebug: 0,
    },
  )
})

const filteredEnvironments = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()

  return [...props.environments]
    .filter((environment) => matchesFilter(environment, activeFilter.value))
    .filter((environment) => {
      if (!query) {
        return true
      }

      const searchText = [
        environment.name,
        environment.preset,
        environment.application.name,
        environment.application.version,
        environment.projectType,
        environment.projectRoot,
        environment.phpVersion,
        environment.webServer,
        environment.database.engine,
        environment.status.state,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()

      return searchText.includes(query)
    })
    .sort((left, right) => compareEnvironments(left, right, sortMode.value))
})

const selectedEnvironment = computed(() => {
  return filteredEnvironments.value.find((environment) => environment.name === selectedEnvironmentName.value) ?? null
})

watch(
  filteredEnvironments,
  (environments) => {
    if (environments.length === 0) {
      selectedEnvironmentName.value = ''
      return
    }

    const stillExists = environments.some((environment) => environment.name === selectedEnvironmentName.value)
    if (!selectedEnvironmentName.value || !stillExists) {
      selectedEnvironmentName.value = environments[0].name
    }
  },
  { immediate: true },
)

function matchesFilter(environment: EnvironmentView, filter: EnvironmentFilter) {
  switch (filter) {
    case 'running':
      return environment.status.state === 'running'
    case 'attention':
      return environment.status.state !== 'running' && environment.status.state !== 'stopped'
    case 'offline':
      return environment.status.state === 'stopped'
    case 'xdebug':
      return environment.tooling.xdebug.enabled
    default:
      return true
  }
}

function compareEnvironments(left: EnvironmentView, right: EnvironmentView, mode: SortMode) {
  if (mode === 'updated') {
    return right.updatedAt.localeCompare(left.updatedAt) || left.name.localeCompare(right.name)
  }

  if (mode === 'name') {
    return left.name.localeCompare(right.name)
  }

  return statusRank(left.status.state) - statusRank(right.status.state) || right.updatedAt.localeCompare(left.updatedAt) || left.name.localeCompare(right.name)
}

function statusRank(state: string) {
  switch (state) {
    case 'running':
      return 0
    case 'partial':
      return 1
    case 'stopped':
      return 3
    default:
      return 2
  }
}

function filterCount(filter: EnvironmentFilter) {
  return statusCounts.value[filter]
}

function selectEnvironment(name: string) {
  selectedEnvironmentName.value = name
}

function environmentTags(environment: EnvironmentView): string[] {
  return [
    environment.tooling.adminer.enabled ? 'Adminer' : null,
    environment.tooling.mailpit.enabled ? 'Mailpit' : null,
    environment.tooling.xdebug.enabled ? 'Xdebug' : null,
  ].filter((tag): tag is string => Boolean(tag))
}
</script>

<template>
  <section class="tab-section surface-panel">
    <div class="section-heading environment-browser-heading mb-4">
      <p class="eyebrow mb-2">Managed environments</p>
      <h2 class="h3 mb-2">Operate your stacks with less hunting</h2>
      <p class="environment-browser-copy mb-0">
        Search, filter, and inspect one stack at a time without losing the wider runtime picture.
      </p>
    </div>

    <div v-if="environments.length === 0 && !isLoading" class="empty-inline">
      No environments yet. Use the Create tab to generate one, then come back here for runtime checks.
    </div>

    <div v-else class="environment-browser-shell">
      <div class="environment-browser__summary">
        <article class="browser-stat browser-stat--accent">
          <span>Visible right now</span>
          <strong>{{ filteredEnvironments.length }}</strong>
          <small>{{ environments.length }} total stack{{ environments.length === 1 ? '' : 's' }} in the registry</small>
        </article>

        <article class="browser-stat browser-stat--success">
          <span>Running</span>
          <strong>{{ statusCounts.running }}</strong>
          <small>Open app and tooling links directly from the detail pane</small>
        </article>

        <article class="browser-stat browser-stat--warning">
          <span>Needs attention</span>
          <strong>{{ statusCounts.attention }}</strong>
          <small>Partial or failed stacks are grouped here for quicker triage</small>
        </article>

        <article class="browser-stat browser-stat--cool">
          <span>Xdebug enabled</span>
          <strong>{{ statusCounts.xdebug }}</strong>
          <small>Ready for the managed VS Code launch configuration</small>
        </article>
      </div>

      <div class="environment-browser">
        <aside class="environment-browser__list">
          <div class="environment-browser__list-head">
            <strong>{{ environments.length }} stack{{ environments.length === 1 ? '' : 's' }}</strong>
            <span class="micro-copy">Select one to inspect, operate, and open supporting tools.</span>
          </div>

          <div class="browser-toolbar">
            <label class="browser-search">
              <span class="tracking">Search</span>
              <input
                v-model="searchQuery"
                type="search"
                class="form-control"
                placeholder="Search name, preset, runtime, or path"
              />
            </label>

            <div class="browser-controls">
              <div class="browser-filter-row">
                <button
                  v-for="option in filterOptions"
                  :key="option.id"
                  type="button"
                  class="filter-pill"
                  :class="{ 'filter-pill--active': option.id === activeFilter }"
                  @click="activeFilter = option.id"
                >
                  <span>{{ option.label }}</span>
                  <small>{{ filterCount(option.id) }}</small>
                </button>
              </div>

              <label class="browser-sort">
                <span class="tracking">Sort</span>
                <select v-model="sortMode" class="form-select">
                  <option value="status">By health</option>
                  <option value="updated">By recent update</option>
                  <option value="name">By name</option>
                </select>
              </label>
            </div>
          </div>

          <div v-if="filteredEnvironments.length === 0" class="empty-inline">
            No environments match the current filters. Clear the search or switch the active filter.
          </div>

          <button
            v-for="environment in filteredEnvironments"
            :key="environment.name"
            type="button"
            class="environment-list-item"
            :class="{ 'environment-list-item--active': environment.name === selectedEnvironmentName }"
            @click="selectEnvironment(environment.name)"
          >
            <div class="environment-list-item__head">
              <div class="environment-list-item__identity">
                <strong>{{ environment.name }}</strong>
                <span class="micro-copy">{{ environment.application.name || environment.preset }} • {{ environment.projectType }}</span>
              </div>

              <span class="environment-status-pill" :class="`environment-status-pill--${environment.status.state}`">
                {{ statusLabel(environment.status.state) }}
              </span>
            </div>

            <p class="environment-list-item__summary mb-0">{{ environmentStateSummary(environment) }}</p>

            <div class="environment-list-item__meta">
              <span>PHP {{ environment.phpVersion }}</span>
              <span>{{ environment.webServer }}</span>
              <span>HTTP {{ environment.network.httpPort }}</span>
              <span>{{ environment.status.containers.length }} container{{ environment.status.containers.length === 1 ? '' : 's' }}</span>
            </div>

            <div class="environment-list-item__footer">
              <div class="environment-list-item__tags">
                <span v-for="tag in environmentTags(environment)" :key="tag" class="tool-tag tool-tag--active">{{ tag }}</span>
                <span v-if="environmentTags(environment).length === 0" class="tool-tag tool-tag--muted">Core only</span>
              </div>
              <span class="micro-copy">Updated {{ formatUpdatedAt(environment.updatedAt) }}</span>
            </div>
          </button>
        </aside>

        <div class="environment-browser__detail">
          <div v-if="selectedEnvironment" class="environment-browser__detail-note">
            <span class="tracking">Selected stack</span>
            <strong>{{ selectedEnvironment.name }}</strong>
            <small>{{ selectedEnvironment.projectRoot }}</small>
          </div>

          <EnvironmentCard
            v-if="selectedEnvironment"
            :environment="selectedEnvironment"
            @environment-updated="emit('environment-updated', $event)"
            @environment-removed="emit('environment-removed', $event)"
            @notify="emit('notify', $event)"
          />

          <div v-else class="empty-inline">
            Select an environment from the list to see its runtime details.
          </div>
        </div>
      </div>
    </div>
  </section>
</template>