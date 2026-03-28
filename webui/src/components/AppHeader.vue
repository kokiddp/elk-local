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
        <h1 class="dashboard-title mb-3">Operate local PHP environments without the sprawl.</h1>
        <p class="dashboard-copy mb-0">
          The CLI remains primary. This UI is a tighter control room for create, runtime operations, and managed backups.
        </p>
      </div>

      <div class="dashboard-actions">
        <div class="root-pill">
          <span>Project root</span>
          <strong>{{ projectRoot }}</strong>
        </div>
        <button type="button" class="btn btn-dark px-3" :disabled="isRefreshing" @click="$emit('refresh')">
          {{ isRefreshing ? 'Refreshing…' : 'Refresh data' }}
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