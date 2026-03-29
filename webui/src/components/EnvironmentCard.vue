<script setup lang="ts">
import { computed, ref } from 'vue'
import { deleteEnvironment, openEnvironmentFolder, openEnvironmentInVSCode, runEnvironmentAction, type EnvironmentView } from '../api'
import { formatUpdatedAt } from '../lib/display'

const props = defineProps<{ environment: EnvironmentView }>()

const emit = defineEmits<{
  'environment-updated': [environment: EnvironmentView]
  'environment-removed': [name: string]
  notify: [payload: { type: 'success' | 'error'; message: string }]
}>()

const pendingAction = ref<'start' | 'stop' | 'remove' | 'open-editor' | 'open-folder' | ''>('')

const hasContainers = computed(() => props.environment.status.containers.length > 0)

const badgeClass = computed(() => {
  const s = props.environment.status.state
  if (s === 'running') return 'badge--running'
  if (s === 'partial') return 'badge--partial'
  if (s === 'stopped') return 'badge--stopped'
  return 'badge--unknown'
})

const badgeLabel = computed(() => {
  const s = props.environment.status.state
  if (s === 'running') return 'Running'
  if (s === 'partial') return 'Degraded'
  if (s === 'stopped') return 'Offline'
  return 'Attention'
})

const dotClass = computed(() => {
  const s = props.environment.status.state
  if (s === 'running') return 'dot--running'
  if (s === 'partial') return 'dot--partial'
  if (s === 'stopped') return 'dot--stopped'
  return 'dot--unknown'
})

function containerDot(c: EnvironmentView['status']['containers'][number]) {
  if (c.health === 'healthy' || (c.state === 'running' && !c.health)) return 'dot--running'
  if (c.state === 'exited' || c.state === 'dead') return 'dot--stopped'
  return 'dot--unknown'
}

function isStartDisabled() {
  return Boolean(pendingAction.value) || props.environment.status.state === 'running'
}

function isStopDisabled() {
  return Boolean(pendingAction.value) || !hasContainers.value
}

async function handleAction(action: 'start' | 'stop') {
  pendingAction.value = action
  try {
    const res = await runEnvironmentAction(props.environment.name, action)
    emit('environment-updated', res.environment)
    emit('notify', { type: 'success', message: action === 'start'
      ? `${res.environment.name} started.`
      : `${res.environment.name} stopped.`
    })
  } catch (e) {
    emit('notify', { type: 'error', message: e instanceof Error ? e.message : `Unable to ${action}.` })
  } finally { pendingAction.value = '' }
}

async function handleRemove() {
  const ok = window.confirm(`Remove ${props.environment.name}? All containers and environment files will be deleted.`)
  if (!ok) return
  pendingAction.value = 'remove'
  try {
    const res = await deleteEnvironment(props.environment.name)
    emit('environment-removed', res.name)
    emit('notify', { type: 'success', message: `${res.name} removed.` })
  } catch (e) {
    emit('notify', { type: 'error', message: e instanceof Error ? e.message : 'Unable to remove.' })
  } finally { pendingAction.value = '' }
}

async function handleOpenVSCode() {
  pendingAction.value = 'open-editor'
  try {
    const res = await openEnvironmentInVSCode(props.environment.name)
    emit('environment-updated', res.environment)
    emit('notify', { type: 'success', message: res.output || `${props.environment.name} opened in VS Code.` })
  } catch (e) {
    emit('notify', { type: 'error', message: e instanceof Error ? e.message : 'Unable to open in VS Code.' })
  } finally { pendingAction.value = '' }
}

async function handleOpenFolder() {
  pendingAction.value = 'open-folder'
  try {
    const res = await openEnvironmentFolder(props.environment.name)
    emit('environment-updated', res.environment)
    emit('notify', { type: 'success', message: res.output || `${props.environment.name} folder opened.` })
  } catch (e) {
    emit('notify', { type: 'error', message: e instanceof Error ? e.message : 'Unable to open folder.' })
  } finally { pendingAction.value = '' }
}
</script>

<template>
  <div class="flex-col gap-8">
    <!-- Header card -->
    <div class="card">
      <div class="card__header">
        <span class="dot" :class="dotClass"/>
        <span style="font-size:13px; font-weight:600">{{ environment.name }}</span>
        <span class="badge" :class="badgeClass">{{ badgeLabel }}</span>
        <span class="badge badge--blue" style="text-transform:none; letter-spacing:0">{{ environment.preset }}</span>
        <span v-if="environment.application?.name" class="badge badge--accent" style="text-transform:none; letter-spacing:0">
          {{ environment.application.name }}{{ environment.application.version ? ` ${environment.application.version}` : '' }}
        </span>
        <div class="card__header-spacer"/>
        <span class="text-xs text-muted">{{ formatUpdatedAt(environment.updatedAt) }}</span>

        <!-- Action buttons -->
        <button
          type="button"
          class="icon-btn icon-btn--green"
          :disabled="isStartDisabled()"
          data-tooltip="Start"
          @click="handleAction('start')"
        >
          <svg viewBox="0 0 16 16" fill="currentColor"><polygon points="4,2 14,8 4,14"/></svg>
        </button>
        <button
          type="button"
          class="icon-btn"
          :disabled="isStopDisabled()"
          data-tooltip="Stop"
          @click="handleAction('stop')"
        >
          <svg viewBox="0 0 16 16" fill="currentColor"><rect x="3" y="3" width="10" height="10" rx="1"/></svg>
        </button>
        <!-- VS Code icon -->
        <button
          type="button"
          class="icon-btn icon-btn--accent"
          :disabled="Boolean(pendingAction)"
          data-tooltip="Open in VS Code"
          @click="handleOpenVSCode"
        >
          <svg viewBox="0 0 100 100" fill="currentColor">
            <path d="M74.1 4.3 51.6 27.5 32.5 11.2 24 16v68l8.5 4.8 19.1-16.3 22.5 23.2L100 88.6V11.4Zm0 72.2L56 60l18.1-20Z"/>
            <path d="m24 16-8.5 4.8v58.4L24 84z" opacity=".4"/>
          </svg>
        </button>
        <!-- Open project folder -->
        <button
          type="button"
          class="icon-btn"
          :disabled="Boolean(pendingAction)"
          data-tooltip="Open project folder"
          @click="handleOpenFolder"
        >
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M2 4h4l1 2h7v8H2V4z"/>
          </svg>
        </button>
        <button
          type="button"
          class="icon-btn icon-btn--red"
          :disabled="Boolean(pendingAction)"
          data-tooltip="Remove environment"
          @click="handleRemove"
        >
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
            <polyline points="3,5 4,14 12,14 13,5"/>
            <line x1="2" y1="5" x2="14" y2="5"/>
            <line x1="6" y1="5" x2="6" y2="3"/>
            <line x1="10" y1="5" x2="10" y2="3"/>
            <line x1="6" y1="3" x2="10" y2="3"/>
          </svg>
        </button>

        <!-- Pending spinner -->
        <div v-if="pendingAction" class="progress-panel__spinner" style="margin-left:4px"/>
      </div>

      <!-- Stack info row -->
      <div class="card__body flex items-center gap-6" style="padding-top:8px; padding-bottom:8px; flex-wrap:wrap">
        <span class="preset-pill">{{ environment.projectType }}</span>
        <span class="preset-pill">PHP {{ environment.phpVersion }}</span>
        <span class="preset-pill">{{ environment.webServer }}</span>
        <span class="preset-pill">{{ environment.database.engine }} {{ environment.database.version }}</span>
        <span v-if="environment.tooling.adminer.enabled" class="preset-pill">Adminer</span>
        <span v-if="environment.tooling.mailpit.enabled" class="preset-pill">Mailpit</span>
        <span v-if="environment.tooling.xdebug.enabled" class="preset-pill">Xdebug</span>
      </div>
    </div>

    <!-- Service tiles -->
    <div class="service-tiles">
      <a
        v-if="environment.urls.app"
        :href="environment.urls.app"
        target="_blank"
        rel="noreferrer"
        class="service-tile service-tile--active"
      >
        <span class="service-tile__label">App</span>
        <span class="service-tile__url">{{ environment.urls.app }}</span>
        <span class="service-tile__meta">HTTP :{{ environment.network.httpPort }}</span>
      </a>
      <div v-else class="service-tile service-tile--disabled">
        <span class="service-tile__label">App</span>
        <span class="service-tile__meta">:{{ environment.network.httpPort }} · not running</span>
      </div>

      <a
        v-if="environment.urls.adminer"
        :href="environment.urls.adminer"
        target="_blank"
        rel="noreferrer"
        class="service-tile"
      >
        <span class="service-tile__label">Adminer</span>
        <span class="service-tile__url">{{ environment.urls.adminer }}</span>
      </a>
      <div v-else-if="environment.tooling.adminer.enabled" class="service-tile service-tile--disabled">
        <span class="service-tile__label">Adminer</span>
        <span class="service-tile__meta">:{{ environment.tooling.adminer.port ?? '—' }} · not running</span>
      </div>

      <a
        v-if="environment.urls.mailpit"
        :href="environment.urls.mailpit"
        target="_blank"
        rel="noreferrer"
        class="service-tile"
      >
        <span class="service-tile__label">Mailpit</span>
        <span class="service-tile__url">{{ environment.urls.mailpit }}</span>
      </a>
      <div v-else-if="environment.tooling.mailpit.enabled" class="service-tile service-tile--disabled">
        <span class="service-tile__label">Mailpit</span>
        <span class="service-tile__meta">not running</span>
      </div>

      <div class="service-tile">
        <span class="service-tile__label">Database</span>
        <span class="service-tile__meta">{{ environment.database.engine }} · {{ environment.database.name }}</span>
        <span class="service-tile__meta">:{{ environment.network.databasePort }}</span>
      </div>
    </div>

    <!-- Error alert -->
    <div v-if="environment.status.error" class="toast toast--error" style="position:static; pointer-events:auto">
      {{ environment.status.error }}
    </div>

    <!-- Paths + Containers in two columns -->
    <div class="flex gap-8" style="align-items:start">
      <!-- Paths -->
      <div class="card flex-1">
        <div class="card__header">
          <svg viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M2 4h4l1 2h7v8H2V4z"/>
          </svg>
          Paths
        </div>
        <div class="card__body flex-col gap-0">
          <div class="detail-row">
            <span class="detail-row__label">Project root</span>
            <span class="detail-row__value">{{ environment.projectRoot }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-row__label">Storage</span>
            <span class="detail-row__value">{{ environment.storagePath }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-row__label">Compose</span>
            <span class="detail-row__value">{{ environment.composePath }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-row__label">DB host port</span>
            <span class="detail-row__value">{{ environment.network.databasePort }}</span>
          </div>
        </div>
      </div>

      <!-- Containers -->
      <div class="card" style="width:260px; flex-shrink:0">
        <div class="card__header">
          <svg viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.5">
            <rect x="1" y="4" width="14" height="10" rx="1"/>
            <path d="M4 4V2h8v2"/>
          </svg>
          Containers
          <span class="badge" :class="hasContainers ? 'badge--running' : 'badge--stopped'">
            {{ environment.status.containers.length }}
          </span>
        </div>
        <div class="card__body flex-col gap-0">
          <div v-if="!hasContainers" class="empty-state" style="padding:12px">Idle</div>
          <div
            v-for="c in environment.status.containers"
            :key="c.name"
            class="container-row"
          >
            <span class="dot" :class="containerDot(c)"/>
            <span class="container-row__name">{{ c.service }}</span>
            <span class="container-row__service">{{ c.state }}</span>
            <span v-for="p in (c.publishedPorts ?? [])" :key="p" class="port-chip">{{ p }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
