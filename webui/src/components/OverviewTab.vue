<script setup lang="ts">
import { computed } from 'vue'
import type { EnvironmentView, PresetOption } from '../api'
import { formatUpdatedAt, statusLabel } from '../lib/display'

const props = defineProps<{
  environments: EnvironmentView[]
  presets: PresetOption[]
  runningCount: number
}>()

defineEmits<{ 'tab-change': [tab: string] }>()

const recentEnvironments = computed(() =>
  [...props.environments]
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
    .slice(0, 5)
)

const stoppedCount = computed(() => props.environments.filter(e => e.status.state === 'stopped').length)
const attentionCount = computed(() =>
  props.environments.filter(e => e.status.state !== 'running' && e.status.state !== 'stopped').length
)

function presetAvatar(name: string) {
  const lower = name.toLowerCase()
  if (lower.includes('word')) return 'wp'
  if (lower.includes('lara')) return 'la'
  if (lower.includes('sym')) return 'sy'
  return 'default'
}

function stateDot(state: string) {
  if (state === 'running') return 'running'
  if (state === 'partial') return 'partial'
  if (state === 'stopped') return 'stopped'
  return 'unknown'
}
</script>

<template>
  <!-- Stats row -->
  <div class="stat-row">
    <div class="stat-chip stat-chip--accent">
      <span class="stat-chip__label">Environments</span>
      <span class="stat-chip__value">{{ environments.length }}</span>
    </div>
    <div class="stat-chip stat-chip--green">
      <span class="stat-chip__label">Running</span>
      <span class="stat-chip__value">{{ runningCount }}</span>
    </div>
    <div class="stat-chip">
      <span class="stat-chip__label">Stopped</span>
      <span class="stat-chip__value">{{ stoppedCount }}</span>
    </div>
    <div class="stat-chip stat-chip--amber">
      <span class="stat-chip__label">Attention</span>
      <span class="stat-chip__value">{{ attentionCount }}</span>
    </div>
    <div class="stat-chip">
      <span class="stat-chip__label">Presets</span>
      <span class="stat-chip__value">{{ presets.length }}</span>
    </div>
  </div>

  <!-- Two-column: recent + presets -->
  <div class="flex gap-12" style="align-items: start">
    <!-- Recent environments -->
    <div class="card flex-1">
      <div class="card__header">
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="8" cy="8" r="6"/>
          <polyline points="8,5 8,8 10,10"/>
        </svg>
        Recent stacks
        <div class="card__header-spacer"/>
        <button type="button" class="btn btn--ghost btn--sm" @click="$emit('tab-change', 'environments')">
          View all
        </button>
      </div>
      <div v-if="environments.length === 0" class="empty-state">
        No environments yet — use Create to get started.
      </div>
      <div v-else>
        <div
          v-for="env in recentEnvironments"
          :key="env.name"
          class="recent-row"
          style="padding: 7px 14px"
        >
          <span :class="`dot dot--${stateDot(env.status.state)}`"/>
          <span class="recent-row__name">{{ env.name }}</span>
          <span class="recent-row__meta">PHP {{ env.phpVersion }} · {{ env.webServer }}</span>
          <span class="recent-row__time">{{ formatUpdatedAt(env.updatedAt) }}</span>
        </div>
      </div>
    </div>

    <!-- Preset catalog -->
    <div class="card" style="width: 260px; flex-shrink: 0">
      <div class="card__header">
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M2 4h12v9a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V4z"/>
          <path d="M5 4V3a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v1"/>
        </svg>
        Presets
      </div>
      <div class="card__body flex-col gap-0" style="padding: 0">
        <div
          v-for="preset in presets"
          :key="preset.name"
          class="preset-row"
          style="padding: 7px 14px"
        >
          <div :class="`avatar avatar--${presetAvatar(preset.name)}`" :data-tooltip="preset.description">
            {{ preset.name.slice(0,2).toUpperCase() }}
          </div>
          <span class="preset-row__name">{{ preset.name }}</span>
          <span class="preset-row__meta">PHP {{ preset.phpVersion }}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Feature cells -->
  <div class="feature-row">
    <div class="feature-cell feature-cell--accent">
      <div class="feature-cell__title">Preset bootstrap</div>
      <div class="feature-cell__desc">One-pass stack + app install for WordPress, Laravel, Symfony.</div>
    </div>
    <div class="feature-cell">
      <div class="feature-cell__title">Lifecycle control</div>
      <div class="feature-cell__desc">Start, stop, destroy from the Environments tab.</div>
    </div>
    <div class="feature-cell">
      <div class="feature-cell__title">Managed backups</div>
      <div class="feature-cell__desc">Create, export, import, and restore archives per environment.</div>
    </div>
    <div class="feature-cell">
      <div class="feature-cell__title">Status at a glance</div>
      <div class="feature-cell__desc">Container state, ports, tooling — no raw docker output needed.</div>
    </div>
  </div>
</template>
