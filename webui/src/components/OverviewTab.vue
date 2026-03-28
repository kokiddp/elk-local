<script setup lang="ts">
import { computed } from 'vue'
import type { EnvironmentView, PresetOption } from '../api'
import { formatUpdatedAt } from '../lib/display'

const props = defineProps<{
  environments: EnvironmentView[]
  presets: PresetOption[]
  runningCount: number
}>()

const recentEnvironments = computed(() => {
  return [...props.environments]
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
    .slice(0, 4)
})

const installablePresets = computed(() => props.presets.filter((preset) => Boolean(preset.applicationName)))
</script>

<template>
  <div class="tab-grid tab-grid--overview">
    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Control Surface</p>
        <h2 class="h3 mb-0">What this UI handles well now</h2>
      </div>

      <div class="feature-grid">
        <article class="feature-card feature-card--accent">
          <strong>Preset bootstrap</strong>
          <span>Create a WordPress, Laravel, or Symfony stack and install the app code in the same flow.</span>
        </article>
        <article class="feature-card">
          <strong>Runtime operations</strong>
          <span>Start, stop, and destroy environments while keeping the card state synchronized.</span>
        </article>
        <article class="feature-card">
          <strong>Managed archives</strong>
          <span>Inspect backup inventory, export to a target path, import archives, and restore from the UI.</span>
        </article>
        <article class="feature-card">
          <strong>Status visibility</strong>
          <span>See ports, tooling, containers, and recent changes without dropping into raw docker output.</span>
        </article>
      </div>
    </section>

    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Snapshot</p>
        <h2 class="h3 mb-0">Current state</h2>
      </div>

      <div class="snapshot-grid">
        <article class="snapshot-card">
          <span>Running environments</span>
          <strong>{{ runningCount }}</strong>
          <small>{{ environments.length - runningCount }} stopped or partial</small>
        </article>
        <article class="snapshot-card">
          <span>Installable presets</span>
          <strong>{{ installablePresets.length }}</strong>
          <small>WordPress, Laravel, Symfony</small>
        </article>
      </div>

      <div class="preset-directory mt-4">
        <h3 class="h6 text-uppercase tracking mb-3">Preset directory</h3>
        <div class="preset-directory__list">
          <article v-for="preset in presets" :key="preset.name" class="preset-directory__item">
            <div>
              <strong>{{ preset.name }}</strong>
              <p class="mb-0">{{ preset.description }}</p>
            </div>
            <small>
              {{ preset.webServer }} • PHP {{ preset.phpVersion }} • {{ preset.databaseEngine }} {{ preset.databaseVersion }}
            </small>
          </article>
        </div>
      </div>
    </section>

    <section class="surface-panel tab-section tab-section--wide">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Recent environments</p>
        <h2 class="h3 mb-0">Latest touched stacks</h2>
      </div>

      <div v-if="recentEnvironments.length === 0" class="empty-inline">
        No environments yet. Start in the Create tab and the latest stacks will appear here.
      </div>

      <div v-else class="recent-list">
        <article v-for="environment in recentEnvironments" :key="environment.name" class="recent-item">
          <div>
            <strong>{{ environment.name }}</strong>
            <p class="mb-0">{{ environment.projectType }} • PHP {{ environment.phpVersion }} • {{ environment.webServer }}</p>
          </div>
          <small>{{ formatUpdatedAt(environment.updatedAt) }}</small>
        </article>
      </div>
    </section>
  </div>
</template>