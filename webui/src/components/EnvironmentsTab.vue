<script setup lang="ts">
import type { EnvironmentView } from '../api'
import EnvironmentCard from './EnvironmentCard.vue'

defineProps<{
  environments: EnvironmentView[]
  isLoading: boolean
}>()

defineEmits<{
  'environment-updated': [environment: EnvironmentView]
  notify: [payload: { type: 'success' | 'error'; message: string }]
}>()
</script>

<template>
  <section class="tab-section surface-panel">
    <div class="section-heading mb-4">
      <p class="eyebrow mb-2">Managed environments</p>
      <h2 class="h3 mb-0">Current stacks</h2>
    </div>

    <div v-if="environments.length === 0 && !isLoading" class="empty-inline">
      No environments yet. Use the Create tab to generate one, then come back here for runtime checks.
    </div>

    <div v-else class="environment-grid">
      <EnvironmentCard
        v-for="environment in environments"
        :key="environment.name"
        :environment="environment"
        @environment-updated="$emit('environment-updated', $event)"
        @notify="$emit('notify', $event)"
      />
    </div>
  </section>
</template>