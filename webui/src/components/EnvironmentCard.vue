<script setup lang="ts">
import { computed, ref } from 'vue'
import { deleteEnvironment, openEnvironmentInVSCode, runEnvironmentAction, type EnvironmentView } from '../api'
import { environmentStateSummary, formatUpdatedAt, statusLabel, toolingSummary } from '../lib/display'

const props = defineProps<{
  environment: EnvironmentView
}>()

const emit = defineEmits<{
  'environment-updated': [environment: EnvironmentView]
  'environment-removed': [name: string]
  notify: [payload: { type: 'success' | 'error'; message: string }]
}>()

const pendingAction = ref<'start' | 'stop' | 'destroy' | 'delete' | 'open-editor' | ''>('')

const hasContainers = computed(() => props.environment.status.containers.length > 0)
const canDelete = computed(() => props.environment.status.state === 'stopped' && !hasContainers.value)
const publishedPortCount = computed(() => {
  return props.environment.status.containers.reduce((total, container) => total + (container.publishedPorts?.length ?? 0), 0)
})

const serviceCards = computed(() => {
  return [
    {
      label: 'Application',
      title: props.environment.urls.app ? 'Open application' : 'No public app URL',
      meta: props.environment.urls.app ?? `${props.environment.projectType} stack`,
      href: props.environment.urls.app,
      accent: true,
    },
    {
      label: 'Adminer',
      title: props.environment.tooling.adminer.enabled ? 'Database UI ready' : 'Adminer disabled',
      meta:
        props.environment.urls.adminer ??
        (props.environment.tooling.adminer.enabled ? `Port ${props.environment.tooling.adminer.port ?? 'pending'}` : 'Enable it when you need DB inspection'),
      href: props.environment.urls.adminer,
      accent: false,
    },
    {
      label: 'Mailpit',
      title: props.environment.tooling.mailpit.enabled ? 'Inbox and SMTP capture' : 'Mailpit disabled',
      meta:
        props.environment.urls.mailpit ??
        (props.environment.tooling.mailpit.enabled
          ? `HTTP ${props.environment.tooling.mailpit.port ?? 'pending'} • SMTP ${props.environment.tooling.mailpit.smtpPort ?? 'pending'}`
          : 'Useful when validating app mail flows'),
      href: props.environment.urls.mailpit,
      accent: false,
    },
    {
      label: 'Database',
      title: props.environment.database.name,
      meta: props.environment.urls.database ?? `${props.environment.database.user}@127.0.0.1:${props.environment.network.databasePort}`,
      href: undefined,
      accent: false,
    },
  ]
})

const pathRows = computed(() => {
  return [
    { label: 'Project root', value: props.environment.projectRoot },
    { label: 'Runtime storage', value: props.environment.storagePath },
    { label: 'Compose file', value: props.environment.composePath },
    { label: 'Manifest', value: props.environment.manifestPath },
  ]
})

const xdebugSummary = computed(() => {
  if (!props.environment.tooling.xdebug.enabled) {
    return 'Disabled for this stack'
  }

  return `Listen with the managed VS Code config on ${props.environment.tooling.xdebug.clientPort}.`
})

const deleteHint = computed(() => {
  if (canDelete.value) {
    return 'This stack is fully stopped and can be removed permanently.'
  }

  return 'Destroy containers first to unlock permanent delete.'
})

function environmentTone(state: string) {
  switch (state) {
    case 'running':
      return 'running'
    case 'partial':
      return 'partial'
    case 'stopped':
      return 'stopped'
    default:
      return 'unknown'
  }
}

function containerTone(container: EnvironmentView['status']['containers'][number]) {
  if (container.health === 'healthy' || (container.state === 'running' && !container.health)) {
    return 'healthy'
  }

  if (container.state === 'exited' || container.state === 'dead') {
    return 'stopped'
  }

  return 'attention'
}

function containerLabel(container: EnvironmentView['status']['containers'][number]) {
  return container.health || container.state
}

function isActionDisabled(action: 'start' | 'stop' | 'destroy') {
  if (pendingAction.value) {
    return true
  }

  if (action === 'start') {
    return props.environment.status.state === 'running'
  }

  return !hasContainers.value
}

function successMessage(action: 'start' | 'stop' | 'destroy', environment: EnvironmentView) {
  switch (action) {
    case 'start':
      return `${environment.name} is running. Open the app link when the container status looks healthy.`
    case 'stop':
      return `${environment.name} is stopped.`
    case 'destroy':
      return `${environment.name} containers were removed.`
  }
}

function deleteMessage(name: string, removedProjectFiles?: boolean, removedBackups?: boolean) {
  const fragments = ['registry entry removed']
  if (removedProjectFiles) {
    fragments.push('managed project files removed')
  }
  if (removedBackups) {
    fragments.push('managed backups removed')
  }

  return `${name} was deleted: ${fragments.join(', ')}.`
}

async function handleAction(action: 'start' | 'stop' | 'destroy') {
  pendingAction.value = action

  try {
    const response = await runEnvironmentAction(props.environment.name, action)
    emit('environment-updated', response.environment)
    emit('notify', {
      type: 'success',
      message: successMessage(action, response.environment),
    })
  } catch (error) {
    emit('notify', {
      type: 'error',
      message: error instanceof Error ? error.message : `Unable to ${action} ${props.environment.name}.`,
    })
  } finally {
    pendingAction.value = ''
  }
}

async function handleDelete() {
  if (!canDelete.value) {
    emit('notify', {
      type: 'error',
      message: `Destroy ${props.environment.name} before deleting it from the dashboard.`,
    })
    return
  }

  const confirmed = window.confirm(
    `Delete ${props.environment.name}? Managed runtime files will be removed immediately. Managed project files are only removed when this environment lives in the default managed directory.`,
  )
  if (!confirmed) {
    return
  }

  pendingAction.value = 'delete'

  try {
    const response = await deleteEnvironment(props.environment.name)
    emit('environment-removed', response.name)
    emit('notify', {
      type: 'success',
      message: deleteMessage(response.name, response.removedProjectFiles, response.removedBackups),
    })
  } catch (error) {
    emit('notify', {
      type: 'error',
      message: error instanceof Error ? error.message : `Unable to delete ${props.environment.name}.`,
    })
  } finally {
    pendingAction.value = ''
  }
}

async function handleOpenInVSCode() {
  pendingAction.value = 'open-editor'

  try {
    const response = await openEnvironmentInVSCode(props.environment.name)
    emit('environment-updated', response.environment)
    emit('notify', {
      type: 'success',
      message: response.output || `${props.environment.name} opened in VS Code.`,
    })
  } catch (error) {
    emit('notify', {
      type: 'error',
      message: error instanceof Error ? error.message : `Unable to open ${props.environment.name} in VS Code.`,
    })
  } finally {
    pendingAction.value = ''
  }
}
</script>

<template>
  <article class="environment-card surface-panel">
    <div class="environment-card__hero">
      <div class="environment-card__mast">
        <div class="environment-card__title-row">
          <h3 class="h4 mb-0">{{ environment.name }}</h3>
          <span class="environment-status-pill" :class="`environment-status-pill--${environmentTone(environment.status.state)}`">
            {{ statusLabel(environment.status.state) }}
          </span>
          <span class="stack-chip">{{ environment.preset }}</span>
          <span v-if="environment.application?.name" class="stack-chip stack-chip--soft">
            {{ environment.application.name }}{{ environment.application.version ? ` ${environment.application.version}` : '' }}
          </span>
        </div>

        <p class="environment-subtitle mb-0">
          {{ environment.projectType }} • PHP {{ environment.phpVersion }} • {{ environment.webServer }} • {{ environment.database.engine }} {{ environment.database.version }}
        </p>

        <div class="environment-state-panel">
          <strong>{{ environmentStateSummary(environment) }}</strong>
          <span class="micro-copy">
            {{ hasContainers ? `${environment.status.containers.length} container${environment.status.containers.length === 1 ? '' : 's'} currently reported.` : 'Compose status is currently idle.' }}
          </span>
        </div>

        <div class="link-row">
          <span class="quick-link quick-link--muted">Updated {{ formatUpdatedAt(environment.updatedAt) }}</span>
          <span class="quick-link quick-link--muted">Tooling {{ toolingSummary(environment) }}</span>
        </div>
      </div>

      <div class="environment-card__actions">
        <div class="action-cluster">
          <button type="button" class="btn btn-outline-secondary" :disabled="Boolean(pendingAction)" @click="handleOpenInVSCode">
            {{ pendingAction === 'open-editor' ? 'Opening…' : 'Open in VS Code' }}
          </button>
          <button type="button" class="btn btn-dark" :disabled="isActionDisabled('start')" @click="handleAction('start')">
            {{ pendingAction === 'start' ? 'Starting…' : 'Start' }}
          </button>
          <button type="button" class="btn btn-outline-dark" :disabled="isActionDisabled('stop')" @click="handleAction('stop')">
            {{ pendingAction === 'stop' ? 'Stopping…' : 'Stop' }}
          </button>
        </div>

        <div class="action-cluster action-cluster--danger">
          <button type="button" class="btn btn-outline-danger" :disabled="isActionDisabled('destroy')" @click="handleAction('destroy')">
            {{ pendingAction === 'destroy' ? 'Destroying…' : 'Destroy' }}
          </button>
          <button type="button" class="btn btn-danger" :disabled="Boolean(pendingAction) || !canDelete" @click="handleDelete">
            {{ pendingAction === 'delete' ? 'Deleting…' : 'Delete' }}
          </button>
        </div>

        <p class="micro-copy environment-card__hint mb-0">
          {{ deleteHint }} Open in VS Code targets the project root shown below.
        </p>
      </div>
    </div>

    <div v-if="!canDelete" class="alert alert-secondary mb-0 py-2">
      Destroy the stack before permanent delete becomes available.
    </div>

    <div class="service-grid">
      <component
        :is="service.href ? 'a' : 'article'"
        v-for="service in serviceCards"
        :key="service.label"
        class="service-card"
        :class="{
          'service-card--interactive': service.href,
          'service-card--accent': service.accent,
          'service-card--muted': !service.href && service.label !== 'Database',
        }"
        v-bind="service.href ? { href: service.href, target: '_blank', rel: 'noreferrer' } : {}"
      >
        <span class="service-card__eyebrow">{{ service.label }}</span>
        <strong>{{ service.title }}</strong>
        <small>{{ service.meta }}</small>
      </component>
    </div>

    <section class="detail-section">
      <div class="section-row mb-3">
        <h4 class="h6 text-uppercase tracking mb-0">Runtime posture</h4>
        <span class="micro-copy">{{ publishedPortCount }} published port{{ publishedPortCount === 1 ? '' : 's' }}</span>
      </div>

      <div class="meta-grid meta-grid--environment">
        <article class="detail-block detail-block--accent">
          <span>Stack</span>
          <strong>{{ environment.projectType }}</strong>
          <small>{{ environment.webServer }} • PHP {{ environment.phpVersion }}</small>
        </article>

        <article class="detail-block">
          <span>Database</span>
          <strong>{{ environment.database.name }}</strong>
          <small>{{ environment.database.user }}@{{ environment.database.host }} • host port {{ environment.network.databasePort }}</small>
        </article>

        <article class="detail-block">
          <span>HTTP</span>
          <strong>{{ environment.network.httpPort }}</strong>
          <small>{{ environment.urls.app ?? 'Open through the application card' }}</small>
        </article>

        <article class="detail-block">
          <span>Optional tooling</span>
          <strong>{{ toolingSummary(environment) }}</strong>
          <small>{{ xdebugSummary }}</small>
        </article>

        <article class="detail-block">
          <span>Containers</span>
          <strong>{{ environment.status.containers.length }}</strong>
          <small>{{ hasContainers ? 'Reported by docker compose status' : 'No active containers currently reported' }}</small>
        </article>
      </div>
    </section>

    <div v-if="environment.status.error" class="alert alert-warning mb-0 py-2">
      {{ environment.status.error }}
    </div>

    <div class="detail-columns">
      <section class="detail-section">
        <div class="section-row mb-3">
          <h4 class="h6 text-uppercase tracking mb-0">Paths</h4>
          <span class="micro-copy">Project and runtime files</span>
        </div>

        <div class="path-list">
          <div v-for="row in pathRows" :key="row.label" class="path-row">
            <span>{{ row.label }}</span>
            <strong class="path-row__value">{{ row.value }}</strong>
          </div>
        </div>
      </section>

      <section class="detail-section">
        <div class="section-row mb-3">
          <h4 class="h6 text-uppercase tracking mb-0">Containers</h4>
          <span class="micro-copy">{{ environment.status.containers.length }} reported</span>
        </div>

        <div v-if="!hasContainers" class="empty-inline">
          No active containers reported by docker compose.
        </div>

        <div v-else class="container-card-grid">
          <article v-for="container in environment.status.containers" :key="container.name" class="container-card">
            <div class="container-card__header">
              <div>
                <strong>{{ container.service }}</strong>
                <div class="micro-copy">{{ container.name }}</div>
              </div>

              <span class="container-pill" :class="`container-pill--${containerTone(container)}`">
                {{ containerLabel(container) }}
              </span>
            </div>

            <div class="micro-copy">State {{ container.state }}</div>

            <div v-if="container.publishedPorts?.length" class="container-port-list">
              <span v-for="publishedPort in container.publishedPorts" :key="publishedPort" class="port-chip">
                {{ publishedPort }}
              </span>
            </div>

            <div v-else class="micro-copy">No published ports</div>
          </article>
        </div>
      </section>
    </div>

    <div class="link-row">
      <span class="quick-link quick-link--muted">DB {{ environment.urls.database ?? `${environment.database.user}@127.0.0.1:${environment.network.databasePort}` }}</span>
      <span v-if="environment.urls.smtp" class="quick-link quick-link--muted">SMTP {{ environment.urls.smtp }}</span>
      <a v-if="environment.urls.app" class="quick-link" :href="environment.urls.app" target="_blank" rel="noreferrer">Open app</a>
      <a v-if="environment.urls.adminer" class="quick-link" :href="environment.urls.adminer" target="_blank" rel="noreferrer">Adminer</a>
      <a v-if="environment.urls.mailpit" class="quick-link" :href="environment.urls.mailpit" target="_blank" rel="noreferrer">Mailpit</a>
    </div>
  </article>
</template>
