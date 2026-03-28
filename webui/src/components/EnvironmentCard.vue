<script setup lang="ts">
import { ref } from 'vue'
import { runEnvironmentAction, type EnvironmentView } from '../api'
import { formatUpdatedAt, statusLabel, statusTone, toolingSummary } from '../lib/display'

const props = defineProps<{
  environment: EnvironmentView
}>()

const emit = defineEmits<{
  'environment-updated': [environment: EnvironmentView]
  notify: [payload: { type: 'success' | 'error'; message: string }]
}>()

const pendingAction = ref<'start' | 'stop' | 'destroy' | ''>('')

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
</script>

<template>
  <article class="environment-card surface-panel">
    <div class="environment-card__head">
      <div>
        <div class="environment-card__title-row">
          <h3 class="h4 mb-0">{{ environment.name }}</h3>
          <span :class="`badge rounded-pill text-bg-${statusTone(environment.status.state)}-subtle status-badge`">
            {{ statusLabel(environment.status.state) }}
          </span>
          <span class="badge rounded-pill text-bg-light border">{{ environment.preset }}</span>
        </div>
        <p class="environment-subtitle mb-2">
          {{ environment.projectType }} • PHP {{ environment.phpVersion }} • {{ environment.webServer }} • {{ environment.database.engine }} {{ environment.database.version }}
        </p>
        <p class="micro-copy mb-1">{{ environment.projectRoot }}</p>
        <p class="micro-copy mb-0">Updated {{ formatUpdatedAt(environment.updatedAt) }}</p>
      </div>

      <div class="action-stack">
        <button type="button" class="btn btn-dark" :disabled="Boolean(pendingAction)" @click="handleAction('start')">
          {{ pendingAction === 'start' ? 'Starting…' : 'Start' }}
        </button>
        <button type="button" class="btn btn-outline-dark" :disabled="Boolean(pendingAction)" @click="handleAction('stop')">
          {{ pendingAction === 'stop' ? 'Stopping…' : 'Stop' }}
        </button>
        <button type="button" class="btn btn-outline-danger" :disabled="Boolean(pendingAction)" @click="handleAction('destroy')">
          {{ pendingAction === 'destroy' ? 'Destroying…' : 'Destroy' }}
        </button>
      </div>
    </div>

    <div class="link-row mb-4">
      <a v-if="environment.urls.app" class="quick-link" :href="environment.urls.app" target="_blank" rel="noreferrer">Open app</a>
      <a v-if="environment.urls.adminer" class="quick-link" :href="environment.urls.adminer" target="_blank" rel="noreferrer">Open Adminer</a>
      <a v-if="environment.urls.mailpit" class="quick-link" :href="environment.urls.mailpit" target="_blank" rel="noreferrer">Open Mailpit</a>
      <span class="quick-link quick-link--muted">DB {{ environment.urls.database }}</span>
      <span v-if="environment.urls.smtp" class="quick-link quick-link--muted">SMTP {{ environment.urls.smtp }}</span>
    </div>

    <div class="meta-grid mb-4">
      <article class="detail-block detail-block--accent">
        <span>Stack</span>
        <strong>{{ environment.projectType }}</strong>
        <small>{{ toolingSummary(environment) }}</small>
      </article>

      <article v-if="environment.application?.name" class="detail-block">
        <span>Application</span>
        <strong>{{ environment.application.name }}</strong>
        <small>{{ environment.application.version || 'Installed' }}</small>
      </article>

      <article class="detail-block">
        <span>Database</span>
        <strong>{{ environment.database.name }}</strong>
        <small>{{ environment.database.user }}@{{ environment.database.host }}</small>
      </article>

      <article class="detail-block">
        <span>Ports</span>
        <strong>HTTP {{ environment.network.httpPort }}</strong>
        <small>DB {{ environment.network.databasePort }}</small>
      </article>

      <article class="detail-block">
        <span>Xdebug</span>
        <strong>{{ environment.tooling.xdebug.enabled ? 'Enabled' : 'Disabled' }}</strong>
        <small v-if="environment.tooling.xdebug.enabled">{{ environment.tooling.xdebug.clientHost }}:{{ environment.tooling.xdebug.clientPort }}</small>
        <small v-else>Not attached to this stack</small>
      </article>
    </div>

    <div v-if="environment.status.error" class="alert alert-warning mb-4 py-2">
      {{ environment.status.error }}
    </div>

    <div>
      <div class="section-row mb-3">
        <h4 class="h6 text-uppercase tracking mb-0">Containers</h4>
        <span class="micro-copy">{{ environment.status.containers.length }} reported</span>
      </div>

      <div v-if="environment.status.containers.length === 0" class="empty-inline">
        No active containers reported by docker compose.
      </div>

      <div v-else class="container-table">
        <div v-for="container in environment.status.containers" :key="container.name" class="container-row">
          <div>
            <strong>{{ container.service }}</strong>
            <div class="micro-copy">{{ container.name }}</div>
          </div>
          <div>
            <span class="container-state">{{ container.state }}</span>
            <div v-if="container.health" class="micro-copy">Health: {{ container.health }}</div>
          </div>
          <div class="micro-copy text-start text-lg-end">
            <div v-for="publishedPort in container.publishedPorts ?? []" :key="publishedPort">{{ publishedPort }}</div>
          </div>
        </div>
      </div>
    </div>
  </article>
</template>