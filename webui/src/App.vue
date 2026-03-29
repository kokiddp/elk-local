<script setup lang="ts">
import { computed, ref } from 'vue'
import BackupsTab from './components/BackupsTab.vue'
import CreateEnvironmentTab from './components/CreateEnvironmentTab.vue'
import EnvironmentsTab from './components/EnvironmentsTab.vue'
import OverviewTab from './components/OverviewTab.vue'
import { useDashboard } from './composables/useDashboard'
import type { EnvironmentView } from './api'

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
  removeEnvironment,
  loadDashboard,
} = useDashboard()

const activeTab = ref('overview')

// Auto-dismiss toasts
let feedbackTimer: ReturnType<typeof setTimeout> | null = null
let errorTimer: ReturnType<typeof setTimeout> | null = null

function handleNotify(payload: { type: 'success' | 'error'; message: string }) {
  clearMessages()
  if (payload.type === 'error') {
    setError(payload.message)
    if (errorTimer) clearTimeout(errorTimer)
    errorTimer = setTimeout(() => setError(''), 8000)
    return
  }
  setFeedback(payload.message)
  if (feedbackTimer) clearTimeout(feedbackTimer)
  feedbackTimer = setTimeout(() => setFeedback(''), 5000)
}

function handleEnvironmentUpdated(environment: EnvironmentView) {
  replaceEnvironment(environment)
}

function handleEnvironmentRemoved(name: string) {
  removeEnvironment(name)
}

function handleRefresh() {
  clearMessages()
  void loadDashboard({ silent: true })
}

const attentionCount = computed(() =>
  environments.value.filter(e => e.status.state !== 'running' && e.status.state !== 'stopped').length
)
</script>

<template>
  <div class="app-layout">
    <!-- Sidebar navigation -->
    <nav class="sidebar">
      <!-- Logo -->
      <div class="sidebar__logo">
        <svg viewBox="0 0 16 16" width="18" height="18" fill="currentColor">
          <path d="M2 2h5v5H2V2zm7 0h5v5H9V2zM2 9h5v5H2V9zm7 0h5v5H9V9z"/>
        </svg>
      </div>

      <!-- Overview -->
      <button
        type="button"
        class="nav-btn"
        :class="{ 'nav-btn--active': activeTab === 'overview' }"
        data-tooltip="Overview"
        @click="activeTab = 'overview'"
      >
        <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="3" y="3" width="6" height="6" rx="1"/>
          <rect x="11" y="3" width="6" height="6" rx="1"/>
          <rect x="3" y="11" width="6" height="6" rx="1"/>
          <rect x="11" y="11" width="6" height="6" rx="1"/>
        </svg>
      </button>

      <!-- Create -->
      <button
        type="button"
        class="nav-btn"
        :class="{ 'nav-btn--active': activeTab === 'create' }"
        data-tooltip="Create environment"
        @click="activeTab = 'create'"
      >
        <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="10" cy="10" r="7"/>
          <line x1="10" y1="7" x2="10" y2="13"/>
          <line x1="7" y1="10" x2="13" y2="10"/>
        </svg>
      </button>

      <!-- Environments -->
      <button
        type="button"
        class="nav-btn"
        :class="{ 'nav-btn--active': activeTab === 'environments' }"
        data-tooltip="Environments"
        @click="activeTab = 'environments'"
      >
        <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="2" y="5" width="16" height="3" rx="1"/>
          <rect x="2" y="10" width="16" height="3" rx="1"/>
          <rect x="2" y="15" width="16" height="3" rx="1"/>
          <circle cx="17" cy="6.5" r="1.5" fill="currentColor" stroke="none"/>
        </svg>
        <span v-if="attentionCount > 0" class="nav-btn__badge">{{ attentionCount }}</span>
      </button>

      <!-- Backups -->
      <button
        type="button"
        class="nav-btn"
        :class="{ 'nav-btn--active': activeTab === 'backups' }"
        data-tooltip="Backups"
        @click="activeTab = 'backups'"
      >
        <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M5 10a5 5 0 1 1 10 0"/>
          <polyline points="10,4 10,10 13,13"/>
          <path d="M3 15h14"/>
        </svg>
      </button>

      <div class="sidebar__spacer"/>

      <!-- Refresh -->
      <button
        type="button"
        class="nav-btn"
        :class="{ 'nav-btn--active': isRefreshing }"
        data-tooltip="Refresh"
        :disabled="isRefreshing"
        @click="handleRefresh"
      >
        <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5"
             :style="isRefreshing ? 'animation: spin 0.7s linear infinite' : ''">
          <path d="M16 10a6 6 0 1 1-1.5-4l1.5-1.5V8h-3.5"/>
        </svg>
      </button>
    </nav>

    <!-- Main area -->
    <div class="main-pane">
      <!-- Top bar -->
      <header class="topbar">
        <span class="topbar__title">ELK-Local</span>
        <div class="topbar__spacer"/>
        <span class="topbar__root" :title="projectRoot">{{ projectRoot }}</span>
        <span
          class="badge"
          :class="runningCount > 0 ? 'badge--running' : 'badge--stopped'"
        >
          {{ runningCount }} running
        </span>
        <span class="badge badge--accent">{{ environments.length }} env</span>
      </header>

      <!-- Content -->
      <div class="content-area">
        <div v-if="isLoading" class="loading-row">
          <div class="progress-panel__spinner"/>
          Loading…
        </div>

        <template v-else>
          <OverviewTab
            v-if="activeTab === 'overview'"
            :environments="environments"
            :presets="presets"
            :running-count="runningCount"
            @tab-change="activeTab = $event"
          />

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
            @environment-removed="handleEnvironmentRemoved"
            @notify="handleNotify"
          />

          <BackupsTab
            v-else-if="activeTab === 'backups'"
            :environments="environments"
            @notify="handleNotify"
            @refresh-environments="loadDashboard({ silent: true })"
          />
        </template>
      </div>
    </div>

    <!-- Toasts -->
    <div class="toast-area">
      <div v-if="feedback" class="toast toast--success">{{ feedback }}</div>
      <div v-if="errorMessage" class="toast toast--error">{{ errorMessage }}</div>
    </div>
  </div>
</template>
