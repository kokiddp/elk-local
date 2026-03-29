<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { EnvironmentView } from '../api'
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

const selectedEnvironmentName = ref('')

const selectedEnvironment = computed(() => {
  return props.environments.find((environment) => environment.name === selectedEnvironmentName.value) ?? null
})

watch(
  () => props.environments,
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

function selectEnvironment(name: string) {
  selectedEnvironmentName.value = name
}
</script>

<template>
  <section class="tab-section surface-panel">
    <div class="section-heading mb-4">
      <p class="eyebrow mb-2">Managed environments</p>
      <h2 class="h3 mb-0">All environments</h2>
    </div>

    <div v-if="environments.length === 0 && !isLoading" class="empty-inline">
      No environments yet. Use the Create tab to generate one, then come back here for runtime checks.
    </div>

    <div v-else class="environment-browser">
      <aside class="environment-browser__list">
        <div class="environment-browser__list-head">
          <strong>{{ environments.length }} stack{{ environments.length === 1 ? '' : 's' }}</strong>
          <span class="micro-copy">Select one to inspect and operate it.</span>
        </div>

        <button
          v-for="environment in environments"
          :key="environment.name"
          type="button"
          class="environment-list-item"
          :class="{ 'environment-list-item--active': environment.name === selectedEnvironmentName }"
          @click="selectEnvironment(environment.name)"
        >
          <div class="environment-list-item__head">
            <strong>{{ environment.name }}</strong>
            <span :class="`badge rounded-pill text-bg-${environment.status.state === 'running' ? 'success' : environment.status.state === 'partial' ? 'warning' : environment.status.state === 'stopped' ? 'secondary' : 'danger'}-subtle status-badge`">
              {{ environment.status.state === 'running' ? 'Running' : environment.status.state === 'partial' ? 'Degraded' : environment.status.state === 'stopped' ? 'Offline' : 'Needs attention' }}
            </span>
          </div>
          <span class="micro-copy">{{ environment.projectType }} • PHP {{ environment.phpVersion }} • {{ environment.webServer }}</span>
          <span class="micro-copy">HTTP {{ environment.network.httpPort }} • {{ environment.status.containers.length }} container{{ environment.status.containers.length === 1 ? '' : 's' }}</span>
        </button>
      </aside>

      <div class="environment-browser__detail">
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
  </section>
</template>