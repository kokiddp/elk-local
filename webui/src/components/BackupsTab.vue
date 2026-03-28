<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  createManagedBackup,
  exportBackup,
  fetchBackups,
  importBackup,
  restoreBackup,
  type BackupActionResponse,
  type BackupView,
  type EnvironmentView,
} from '../api'
import { backupStatusLabel, formatBytes, formatUpdatedAt } from '../lib/display'

const props = defineProps<{
  environments: EnvironmentView[]
}>()

const emit = defineEmits<{
  notify: [payload: { type: 'success' | 'error'; message: string }]
  'refresh-environments': []
}>()

const selectedEnvironmentName = ref('')
const backups = ref<BackupView[]>([])
const isInventoryLoading = ref(false)
const inventoryError = ref('')
const busyAction = ref('')

const createForm = reactive({
  includeProjectFiles: false,
  force: false,
})

const exportForm = reactive({
  outputPath: '',
  includeProjectFiles: false,
  force: false,
})

const importForm = reactive({
  archivePath: '',
  force: false,
})

const restoreForm = reactive({
  archivePath: '',
  restoreProjectFiles: false,
  force: false,
})

const selectedEnvironment = computed(() => {
  return props.environments.find((environment) => environment.name === selectedEnvironmentName.value) ?? null
})

watch(
  () => props.environments,
  (environments) => {
    if (environments.length === 0) {
      selectedEnvironmentName.value = ''
      backups.value = []
      return
    }

    const stillExists = environments.some((environment) => environment.name === selectedEnvironmentName.value)
    if (!selectedEnvironmentName.value || !stillExists) {
      selectedEnvironmentName.value = environments[0].name
    }
  },
  { immediate: true, deep: true },
)

watch(selectedEnvironmentName, () => {
  void loadInventory()
}, { immediate: true })

async function loadInventory() {
  if (!selectedEnvironmentName.value) {
    backups.value = []
    inventoryError.value = ''
    return
  }

  isInventoryLoading.value = true
  inventoryError.value = ''

  try {
    const response = await fetchBackups(selectedEnvironmentName.value)
    backups.value = response.backups
  } catch (error) {
    inventoryError.value = error instanceof Error ? error.message : 'Unable to load backups.'
  } finally {
    isInventoryLoading.value = false
  }
}

function useBackupForRestore(backup: BackupView) {
  restoreForm.archivePath = backup.fileName || backup.path
}

function applyBackupResponse(response: BackupActionResponse, successMessage: string) {
  backups.value = response.backups ?? backups.value
  emit('notify', {
    type: 'success',
    message: response.message || successMessage,
  })
  emit('refresh-environments')
}

async function runAction(actionName: string, callback: () => Promise<BackupActionResponse>) {
  busyAction.value = actionName

  try {
    const response = await callback()
    applyBackupResponse(response, 'Backup workflow completed.')
  } catch (error) {
    emit('notify', {
      type: 'error',
      message: error instanceof Error ? error.message : 'Backup workflow failed.',
    })
  } finally {
    busyAction.value = ''
  }
}

function ensureEnvironmentSelected() {
  if (!selectedEnvironmentName.value) {
    emit('notify', { type: 'error', message: 'Choose an environment before running backup actions.' })
    return false
  }

  return true
}

async function handleCreateBackup() {
  if (!ensureEnvironmentSelected()) {
    return
  }

  await runAction('create', () => createManagedBackup(selectedEnvironmentName.value, { ...createForm }))
}

async function handleExportBackup() {
  if (!ensureEnvironmentSelected()) {
    return
  }

  if (!exportForm.outputPath.trim()) {
    emit('notify', { type: 'error', message: 'Provide an output path for export.' })
    return
  }

  await runAction('export', () => exportBackup(selectedEnvironmentName.value, { ...exportForm }))
}

async function handleImportBackup() {
  if (!ensureEnvironmentSelected()) {
    return
  }

  if (!importForm.archivePath.trim()) {
    emit('notify', { type: 'error', message: 'Provide the archive path to import.' })
    return
  }

  await runAction('import', () => importBackup(selectedEnvironmentName.value, { ...importForm }))
}

async function handleRestoreBackup() {
  if (!ensureEnvironmentSelected()) {
    return
  }

  if (!restoreForm.archivePath.trim()) {
    emit('notify', { type: 'error', message: 'Choose a managed archive or enter a path to restore.' })
    return
  }

  if (!restoreForm.force) {
    emit('notify', { type: 'error', message: 'Restore requires explicit confirmation because it replaces the target database.' })
    return
  }

  await runAction('restore', () => restoreBackup(selectedEnvironmentName.value, { ...restoreForm }))
}
</script>

<template>
  <div v-if="environments.length === 0" class="surface-panel tab-section">
    <div class="empty-inline">Create an environment first. Backup inventory and restore operations are scoped per managed environment.</div>
  </div>

  <div v-else class="tab-grid tab-grid--backups">
    <section class="surface-panel tab-section tab-section--wide">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Backup inventory</p>
        <h2 class="h3 mb-0">Managed archives for one environment at a time</h2>
      </div>

      <div class="inventory-toolbar mb-4">
        <div class="inventory-toolbar__picker">
          <label class="form-label" for="backup-environment">Environment</label>
          <select id="backup-environment" v-model="selectedEnvironmentName" class="form-select">
            <option v-for="environment in environments" :key="environment.name" :value="environment.name">
              {{ environment.name }}
            </option>
          </select>
        </div>

        <div class="inventory-toolbar__meta">
          <div v-if="selectedEnvironment" class="root-pill">
            <span>Managed backup path</span>
            <strong>{{ selectedEnvironment.storagePath }}/backups</strong>
          </div>
          <button type="button" class="btn btn-outline-dark" :disabled="isInventoryLoading" @click="loadInventory">
            {{ isInventoryLoading ? 'Loading…' : 'Refresh inventory' }}
          </button>
        </div>
      </div>

      <div v-if="inventoryError" class="alert alert-danger mb-4">{{ inventoryError }}</div>

      <div v-if="backups.length === 0 && !isInventoryLoading" class="empty-inline">
        No managed archives yet for this environment. Create one below or import an existing archive.
      </div>

      <div v-else class="backup-list">
        <article v-for="backup in backups" :key="backup.path" class="backup-item">
          <div class="backup-item__main">
            <div>
              <strong>{{ backup.fileName }}</strong>
              <p class="mb-0">{{ backupStatusLabel(backup) }}</p>
            </div>
            <div class="backup-badges">
              <span class="pill-badge">{{ formatBytes(backup.sizeBytes) }}</span>
              <span class="pill-badge">{{ formatUpdatedAt(backup.createdAt) }}</span>
              <span class="pill-badge">{{ backup.databaseName || 'Unknown database' }}</span>
            </div>
          </div>

          <div class="backup-item__sub">
            <small>{{ backup.path }}</small>
            <button type="button" class="btn btn-sm btn-outline-dark" :disabled="Boolean(backup.error)" @click="useBackupForRestore(backup)">
              Use for restore
            </button>
          </div>

          <div v-if="backup.error" class="alert alert-warning mb-0 mt-3 py-2">
            {{ backup.error }}
          </div>
        </article>
      </div>
    </section>

    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Create or export</p>
        <h2 class="h3 mb-0">Produce new archives</h2>
      </div>

      <div class="action-card-stack">
        <article class="action-card">
          <h3 class="h5 mb-3">Create managed backup</h3>
          <label class="check-row">
            <input v-model="createForm.includeProjectFiles" type="checkbox" />
            <span>Include project files in the managed archive.</span>
          </label>
          <label class="check-row">
            <input v-model="createForm.force" type="checkbox" />
            <span>Allow overwriting if the generated path already exists.</span>
          </label>
          <button type="button" class="btn btn-dark w-100" :disabled="busyAction === 'create'" @click="handleCreateBackup">
            {{ busyAction === 'create' ? 'Creating…' : 'Create managed backup' }}
          </button>
        </article>

        <article class="action-card">
          <h3 class="h5 mb-3">Export backup</h3>
          <label class="form-label" for="backup-export-path">Output path</label>
          <input id="backup-export-path" v-model.trim="exportForm.outputPath" class="form-control mb-3" placeholder="/tmp/my-site.tar.gz" />
          <label class="check-row">
            <input v-model="exportForm.includeProjectFiles" type="checkbox" />
            <span>Include project files in the exported archive.</span>
          </label>
          <label class="check-row">
            <input v-model="exportForm.force" type="checkbox" />
            <span>Overwrite the target file if it already exists.</span>
          </label>
          <button type="button" class="btn btn-outline-dark w-100" :disabled="busyAction === 'export'" @click="handleExportBackup">
            {{ busyAction === 'export' ? 'Exporting…' : 'Export archive' }}
          </button>
        </article>
      </div>
    </section>

    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Import or restore</p>
        <h2 class="h3 mb-0">Bring archives back into circulation</h2>
      </div>

      <div class="action-card-stack">
        <article class="action-card">
          <h3 class="h5 mb-3">Import archive</h3>
          <label class="form-label" for="backup-import-path">Archive path</label>
          <input id="backup-import-path" v-model.trim="importForm.archivePath" class="form-control mb-3" placeholder="/tmp/my-site.tar.gz" />
          <label class="check-row">
            <input v-model="importForm.force" type="checkbox" />
            <span>Overwrite a managed archive with the same file name.</span>
          </label>
          <button type="button" class="btn btn-outline-dark w-100" :disabled="busyAction === 'import'" @click="handleImportBackup">
            {{ busyAction === 'import' ? 'Importing…' : 'Import into inventory' }}
          </button>
        </article>

        <article class="action-card action-card--danger">
          <h3 class="h5 mb-3">Restore archive</h3>
          <label class="form-label" for="backup-restore-path">Managed file name or archive path</label>
          <input id="backup-restore-path" v-model.trim="restoreForm.archivePath" class="form-control mb-3" placeholder="my-site-20260328-123456Z.tar.gz" />
          <label class="check-row">
            <input v-model="restoreForm.restoreProjectFiles" type="checkbox" />
            <span>Also restore project files from the archive when available.</span>
          </label>
          <label class="check-row check-row--danger">
            <input v-model="restoreForm.force" type="checkbox" />
            <span>I understand this replaces the target database contents.</span>
          </label>
          <button type="button" class="btn btn-danger w-100" :disabled="busyAction === 'restore'" @click="handleRestoreBackup">
            {{ busyAction === 'restore' ? 'Restoring…' : 'Restore archive' }}
          </button>
        </article>
      </div>

      <p class="micro-copy mt-4 mb-0">
        Paths are resolved on the same machine running the ELK-Local API. Restore accepts either a managed file name from the inventory above or an absolute archive path.
      </p>
    </section>
  </div>
</template>