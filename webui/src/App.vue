<script setup lang="ts">
import { computed, ref } from 'vue'
import AppHeader from './components/AppHeader.vue'
import BackupsTab from './components/BackupsTab.vue'
import CreateEnvironmentTab from './components/CreateEnvironmentTab.vue'
import EnvironmentsTab from './components/EnvironmentsTab.vue'
import OverviewTab from './components/OverviewTab.vue'
import TabBar from './components/TabBar.vue'
import { useDashboard } from './composables/useDashboard'
import type { EnvironmentView } from './api'

interface DashboardTab {
  id: string
  label: string
  summary: string
  count?: number
}

const {
  environments,
  presets,
  projectRoot,
  defaultProjectRootBase,
  runningCount,
  isLoading,
  isRefreshing,
  feedback,
  errorMessage,
  clearMessages,
  setFeedback,
  setError,
  replaceEnvironment,
  loadDashboard,
} = useDashboard()

const activeTab = ref('overview')

const tabs = computed<DashboardTab[]>(() => [
  {
    id: 'overview',
    label: 'Overview',
    summary: 'State, presets, and workflow coverage',
  },
  {
    id: 'create',
    label: 'Create',
    summary: 'Bootstrap a stack and install the app',
    count: presets.value.length,
  },
  {
    id: 'environments',
    label: 'Environments',
    summary: 'Operate running or stopped stacks',
    count: environments.value.length,
  },
  {
    id: 'backups',
    label: 'Backups',
    summary: 'Inventory, export, import, restore',
    count: environments.value.length,
  },
])

function handleNotify(payload: { type: 'success' | 'error'; message: string }) {
  if (payload.type === 'error') {
    setError(payload.message)
    return
  }

  setFeedback(payload.message)
}

function handleEnvironmentUpdated(environment: EnvironmentView) {
  replaceEnvironment(environment)
}

function handleRefresh() {
  clearMessages()
  void loadDashboard({ silent: true })
}
</script>

<template>
  <main class="app-shell container-xxl py-4 py-xl-5">
    <AppHeader
      :project-root="projectRoot"
      :environment-count="environments.length"
      :running-count="runningCount"
      :preset-count="presets.length"
      :is-refreshing="isRefreshing"
      @refresh="handleRefresh"
    />

    <div v-if="errorMessage" class="alert alert-danger notice-panel" role="alert">
      {{ errorMessage }}
    </div>

    <div v-if="feedback" class="alert alert-success notice-panel" role="alert">
      {{ feedback }}
    </div>

    <TabBar v-model="activeTab" :tabs="tabs" />

    <section class="workspace-panel">
      <div v-if="isLoading" class="surface-panel tab-section loading-state">
        Loading dashboard data…
      </div>

      <OverviewTab v-else-if="activeTab === 'overview'" :environments="environments" :presets="presets" :running-count="runningCount" />

      <CreateEnvironmentTab
        v-else-if="activeTab === 'create'"
        :presets="presets"
        :default-project-root-base="defaultProjectRootBase"
        @environment-updated="handleEnvironmentUpdated"
        @notify="handleNotify"
        @tab-change="activeTab = $event"
      />

      <EnvironmentsTab
        v-else-if="activeTab === 'environments'"
        :environments="environments"
        :is-loading="isLoading"
        @environment-updated="handleEnvironmentUpdated"
        @notify="handleNotify"
      />

      <BackupsTab
        v-else
        :environments="environments"
        @notify="handleNotify"
        @refresh-environments="loadDashboard({ silent: true })"
      />
    </section>
  </main>
</template>