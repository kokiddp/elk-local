<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EnvironmentView } from '../api'
import { statusLabel } from '../lib/display'
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
  { id: 'attention', label: 'Attn' },
  { id: 'offline', label: 'Offline' },
  { id: 'xdebug', label: 'Xdebug' },
] as const

type EnvironmentFilter = (typeof filterOptions)[number]['id']
type SortMode = 'status' | 'updated' | 'name'

const selectedEnvironmentName = ref('')
const searchQuery = ref('')
const activeFilter = ref<EnvironmentFilter>('all')
const sortMode = ref<SortMode>('status')

const statusCounts = computed(() =>
  props.environments.reduce(
    (counts, e) => {
      counts.all += 1
      if (e.status.state === 'running') counts.running += 1
      if (e.status.state === 'stopped') counts.offline += 1
      if (e.status.state !== 'running' && e.status.state !== 'stopped') counts.attention += 1
      if (e.tooling.xdebug.enabled) counts.xdebug += 1
      return counts
    },
    { all: 0, running: 0, attention: 0, offline: 0, xdebug: 0 },
  )
)

const filteredEnvironments = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return [...props.environments]
    .filter(e => matchesFilter(e, activeFilter.value))
    .filter(e => {
      if (!query) return true
      return [e.name, e.preset, e.application.name, e.projectType, e.phpVersion, e.webServer, e.status.state]
        .filter(Boolean).join(' ').toLowerCase().includes(query)
    })
    .sort((a, b) => compareEnvironments(a, b, sortMode.value))
})

const selectedEnvironment = computed(() =>
  filteredEnvironments.value.find(e => e.name === selectedEnvironmentName.value) ?? null
)

watch(
  filteredEnvironments,
  (envs) => {
    if (envs.length === 0) { selectedEnvironmentName.value = ''; return }
    const still = envs.some(e => e.name === selectedEnvironmentName.value)
    if (!selectedEnvironmentName.value || !still) selectedEnvironmentName.value = envs[0].name
  },
  { immediate: true },
)

function matchesFilter(e: EnvironmentView, f: EnvironmentFilter) {
  switch (f) {
    case 'running':   return e.status.state === 'running'
    case 'attention': return e.status.state !== 'running' && e.status.state !== 'stopped'
    case 'offline':   return e.status.state === 'stopped'
    case 'xdebug':    return e.tooling.xdebug.enabled
    default:          return true
  }
}

function compareEnvironments(a: EnvironmentView, b: EnvironmentView, mode: SortMode) {
  if (mode === 'updated') return b.updatedAt.localeCompare(a.updatedAt)
  if (mode === 'name') return a.name.localeCompare(b.name)
  return statusRank(a.status.state) - statusRank(b.status.state) || b.updatedAt.localeCompare(a.updatedAt)
}

function statusRank(s: string) {
  return s === 'running' ? 0 : s === 'partial' ? 1 : s === 'stopped' ? 3 : 2
}

function dotClass(state: string) {
  if (state === 'running') return 'dot--running'
  if (state === 'partial') return 'dot--partial'
  if (state === 'stopped') return 'dot--stopped'
  return 'dot--unknown'
}
</script>

<template>
  <div v-if="environments.length === 0 && !isLoading" class="empty-state">
    No environments. Use Create to bootstrap one.
  </div>

  <div v-else class="split-pane">
    <!-- Left: list -->
    <div class="flex-col gap-8">
      <!-- Toolbar -->
      <div class="card">
        <div class="card__body flex-col gap-8">
          <!-- Search -->
          <div class="search-wrap">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
              <circle cx="7" cy="7" r="4.5"/>
              <line x1="10.5" y1="10.5" x2="14" y2="14"/>
            </svg>
            <input v-model="searchQuery" type="search" placeholder="Search…"/>
          </div>

          <!-- Filters -->
          <div class="filter-bar">
            <button
              v-for="opt in filterOptions"
              :key="opt.id"
              type="button"
              class="filter-btn"
              :class="{ 'filter-btn--active': opt.id === activeFilter }"
              @click="activeFilter = opt.id"
            >
              {{ opt.label }}
              <span class="filter-btn__count">{{ statusCounts[opt.id] }}</span>
            </button>
          </div>

          <!-- Sort -->
          <div class="flex items-center gap-6">
            <span class="text-xs text-muted">Sort</span>
            <select v-model="sortMode" style="flex:1">
              <option value="status">By health</option>
              <option value="updated">By updated</option>
              <option value="name">By name</option>
            </select>
          </div>
        </div>
      </div>

      <!-- Env list -->
      <div class="list-box list-box--scroll">
        <div v-if="filteredEnvironments.length === 0" class="empty-state">No matches.</div>
        <button
          v-for="env in filteredEnvironments"
          :key="env.name"
          type="button"
          class="env-list-item"
          :class="{ 'env-list-item--active': env.name === selectedEnvironmentName }"
          @click="selectedEnvironmentName = env.name"
        >
          <span class="dot" :class="dotClass(env.status.state)"/>
          <span class="env-list-item__name">{{ env.name }}</span>
          <span class="env-list-item__meta">:{{ env.network.httpPort }}</span>
        </button>
      </div>
    </div>

    <!-- Right: detail -->
    <div>
      <div v-if="selectedEnvironment">
        <EnvironmentCard
          :environment="selectedEnvironment"
          @environment-updated="emit('environment-updated', $event)"
          @environment-removed="emit('environment-removed', $event)"
          @notify="emit('notify', $event)"
        />
      </div>
      <div v-else class="empty-state">Select an environment.</div>
    </div>
  </div>
</template>
