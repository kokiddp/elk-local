<script setup lang="ts">
defineProps<{
  projectRoot: string
  environmentCount: number
  runningCount: number
  presetCount: number
  isRefreshing: boolean
}>()

defineEmits<{
  refresh: []
}>()
</script>

<template>
  <header class="dashboard-header surface-panel">
    <div class="dashboard-header__content">
      <div>
        <p class="eyebrow mb-2">ELK-Local Dashboard</p>
        <h1 class="dashboard-title mb-3">A tighter control room for local PHP stacks.</h1>
        <p class="dashboard-copy mb-0">
          Keep the CLI in the lead for scripting and repeatable flows. Use this view when you need fast visibility into
          runtime health, ports, tooling, backups, and the stack you want to touch next.
        </p>

        <div class="dashboard-ribbon mt-3">
          <span class="dashboard-ribbon__pill">CLI-first</span>
          <span class="dashboard-ribbon__pill">Runtime state at a glance</span>
          <span class="dashboard-ribbon__pill">Backups and tooling in one place</span>
        </div>
      </div>

      <div class="dashboard-actions">
        <div class="root-pill">
          <span>Project root</span>
          <strong>{{ projectRoot }}</strong>
        </div>
        <button type="button" class="btn btn-dark px-3" :disabled="isRefreshing" @click="$emit('refresh')">
          {{ isRefreshing ? 'Refreshing…' : 'Refresh dashboard' }}
        </button>
      </div>
    </div>

    <div class="header-stats">
      <article class="header-stat">
        <span>Managed environments</span>
        <strong>{{ environmentCount }}</strong>
      </article>
      <article class="header-stat">
        <span>Running now</span>
        <strong>{{ runningCount }}</strong>
      </article>
      <article class="header-stat">
        <span>Preset catalog</span>
        <strong>{{ presetCount }}</strong>
      </article>
    </div>
  </header>
</template>