<script setup lang="ts">
interface DashboardTab {
  id: string
  label: string
  summary: string
  count?: number
}

defineProps<{
  modelValue: string
  tabs: DashboardTab[]
}>()

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <nav class="tab-bar surface-panel" aria-label="Dashboard sections">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      class="tab-chip"
      :class="{ 'tab-chip--active': tab.id === modelValue }"
      @click="$emit('update:modelValue', tab.id)"
    >
      <span class="tab-chip__label">{{ tab.label }}</span>
      <span class="tab-chip__summary">{{ tab.summary }}</span>
      <span v-if="typeof tab.count === 'number'" class="tab-chip__count">{{ tab.count }}</span>
    </button>
  </nav>
</template>