<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  createManagedBackup,
  deleteManagedBackup,
  downloadManagedBackup,
  exportBackup,
  fetchBackups,
  importBackup,
  openManagedBackupFolder,
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
const selectedBackupPath = ref('')
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

const selectedBackup = computed(() => {
  return backups.value.find((backup) => backup.path === selectedBackupPath.value) ?? backups.value[0] ?? null
})

const totalBackupSizeBytes = computed(() => {
  return backups.value.reduce((totalSize, backup) => totalSize + backup.sizeBytes, 0)
})

const projectSnapshotCount = computed(() => {
  return backups.value.filter((backup) => backup.includesProjectFiles).length
})

const latestBackup = computed(() => backups.value[0] ?? null)

const inventoryStatusLabel = computed(() => {
  if (isInventoryLoading.value) {
    return 'Refreshing inventory'
  }

  if (backups.value.length === 0) {
    return 'No managed archives yet'
  }

  return `${backups.value.length} archive${backups.value.length === 1 ? '' : 's'} available`
})

watch(
  () => props.environments,
  (environments) => {
    if (environments.length === 0) {
      selectedEnvironmentName.value = ''
      selectedBackupPath.value = ''
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

watch(backups, (items) => {
  if (items.some((backup) => backup.path === selectedBackupPath.value)) {
    return
  }

  selectedBackupPath.value = items[0]?.path ?? ''
}, { deep: true })

async function loadInventory() {
  if (!selectedEnvironmentName.value) {
    backups.value = []
    selectedBackupPath.value = ''
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

function selectBackup(backup: BackupView) {
  selectedBackupPath.value = backup.path
}

function useBackupForRestore(backup: BackupView) {
  selectedBackupPath.value = backup.path
  restoreForm.archivePath = backup.fileName || backup.path
}

function applyBackupResponse(response: BackupActionResponse, successMessage: string) {
  backups.value = response.backups ?? backups.value
  selectedBackupPath.value = response.backup?.path ?? selectedBackupPath.value
  emit('notify', {
    type: 'success',
    message: response.message || successMessage,
  })
  emit('refresh-environments')
}

function isBusy(actionName: string) {
  return busyAction.value === actionName
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

async function handleOpenBackupFolder(backup: BackupView) {
  if (!ensureEnvironmentSelected()) {
    return
  }

  await runAction(`open-folder:${backup.fileName}`, () => openManagedBackupFolder(selectedEnvironmentName.value, backup.fileName))
}

async function handleDownloadBackup(backup: BackupView) {
  if (!ensureEnvironmentSelected()) {
    return
  }

  busyAction.value = `download:${backup.fileName}`

  try {
    await downloadManagedBackup(selectedEnvironmentName.value, backup.fileName)
    emit('notify', {
      type: 'success',
      message: `Downloaded ${backup.fileName}`,
    })
  } catch (error) {
    emit('notify', {
      type: 'error',
      message: error instanceof Error ? error.message : 'Backup download failed.',
    })
  } finally {
    busyAction.value = ''
  }
}

async function handleDeleteBackup(backup: BackupView) {
  if (!ensureEnvironmentSelected()) {
    return
  }

  const confirmed = window.confirm(`Delete the managed backup ${backup.fileName}? This cannot be undone.`)
  if (!confirmed) {
    return
  }

  await runAction(`delete:${backup.fileName}`, () => deleteManagedBackup(selectedEnvironmentName.value, backup.fileName))
}
</script>

<template>
  <div v-if="environments.length === 0" class="surface-panel tab-section">
    <div class="empty-inline">Create an environment first. Backup inventory and restore operations are scoped per managed environment.</div>
  </div>

  <div v-else class="tab-grid tab-grid--backups">
    <section class="surface-panel tab-section tab-section--wide">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Backup control room</p>
        <h2 class="h3 mb-0">Browse managed archives and act on them directly</h2>
      </div>

      <div class="inventory-toolbar backup-toolbar mb-4">
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
          <div class="backup-toolbar__actions">
            <button type="button" class="btn btn-outline-dark" :disabled="isInventoryLoading" @click="loadInventory">
              {{ isInventoryLoading ? 'Loading…' : 'Refresh inventory' }}
            </button>
          </div>
        </div>
      </div>

      <div class="backup-browser__summary mb-4">
        <article class="browser-stat browser-stat--accent">
          <span>Managed archives</span>
          <strong>{{ backups.length }}</strong>
          <p class="mb-0">{{ inventoryStatusLabel }}</p>
        </article>
        <article class="browser-stat browser-stat--cool">
          <span>Stored footprint</span>
          <strong>{{ formatBytes(totalBackupSizeBytes) }}</strong>
          <p class="mb-0">Combined size across the managed inventory.</p>
        </article>
        <article class="browser-stat browser-stat--success">
          <span>Full snapshots</span>
          <strong>{{ projectSnapshotCount }}</strong>
          <p class="mb-0">Archives that include both the database and project files.</p>
        </article>
        <article class="browser-stat browser-stat--warning">
          <span>Latest archive</span>
          <strong>{{ latestBackup ? formatUpdatedAt(latestBackup.createdAt) : 'Waiting' }}</strong>
          <p class="mb-0">Newest managed backup currently visible to the dashboard.</p>
        </article>
      </div>

      <div v-if="inventoryError" class="alert alert-danger mb-4">{{ inventoryError }}</div>

      <div class="backup-browser">
        <div class="backup-browser__list">
          <div class="backup-browser__list-head">
            <strong>{{ inventoryStatusLabel }}</strong>
            <p class="mb-0">Select an archive to inspect it, download it, open its folder, or queue it for restore.</p>
          </div>

          <div v-if="backups.length === 0 && !isInventoryLoading" class="empty-inline">
            No managed archives yet for this environment. Create one below or import an existing archive.
          </div>

          <button
            v-for="backup in backups"
            :key="backup.path"
            type="button"
            class="backup-list-item"
            :class="{
              'backup-list-item--active': selectedBackup?.path === backup.path,
              'backup-list-item--warning': Boolean(backup.error),
            }"
            @click="selectBackup(backup)"
          >
            <div class="backup-list-item__head">
              <div>
                <strong>{{ backup.fileName }}</strong>
                <p class="mb-0">{{ backupStatusLabel(backup) }}</p>
              </div>
              <span
                class="environment-status-pill"
                :class="backup.error ? 'environment-status-pill--unknown' : backup.includesProjectFiles ? 'environment-status-pill--running' : 'environment-status-pill--stopped'"
              >
                {{ backup.error ? 'Needs attention' : backup.includesProjectFiles ? 'Full snapshot' : 'Database only' }}
              </span>
            </div>

            <div class="backup-list-item__meta">
              <span>{{ formatBytes(backup.sizeBytes) }}</span>
              <span>{{ backup.databaseName || 'Unknown database' }}</span>
              <span>{{ formatUpdatedAt(backup.createdAt) }}</span>
            </div>

            <p class="backup-list-item__path mb-0">{{ backup.path }}</p>
          </button>
        </div>

        <div class="backup-browser__detail">
          <article v-if="selectedBackup" class="backup-detail">
            <div class="backup-detail__hero">
              <div class="backup-detail__identity">
                <p class="eyebrow mb-2">Selected archive</p>
                <h3 class="h4 mb-2">{{ selectedBackup.fileName }}</h3>
                <p class="mb-0">{{ backupStatusLabel(selectedBackup) }} for {{ selectedEnvironmentName }}</p>
              </div>

              <div class="backup-detail__actions">
                <button
                  type="button"
                  class="btn btn-dark"
                  :disabled="isBusy('download:' + selectedBackup.fileName)"
                  @click="handleDownloadBackup(selectedBackup)"
                >
                  {{ isBusy('download:' + selectedBackup.fileName) ? 'Downloading…' : 'Download archive' }}
                </button>
                <button
                  type="button"
                  class="btn btn-outline-dark"
                  :disabled="isBusy('open-folder:' + selectedBackup.fileName)"
                  @click="handleOpenBackupFolder(selectedBackup)"
                >
                  {{ isBusy('open-folder:' + selectedBackup.fileName) ? 'Opening…' : 'Open folder' }}
                </button>
                <button type="button" class="btn btn-outline-dark" :disabled="Boolean(selectedBackup.error)" @click="useBackupForRestore(selectedBackup)">
                  Use for restore
                </button>
                <button
                  type="button"
                  class="btn btn-outline-danger"
                  :disabled="isBusy('delete:' + selectedBackup.fileName)"
                  @click="handleDeleteBackup(selectedBackup)"
                >
                  {{ isBusy('delete:' + selectedBackup.fileName) ? 'Deleting…' : 'Delete archive' }}
                </button>
              </div>
            </div>

            <div class="meta-grid meta-grid--backup">
              <div class="detail-block">
                <span>Captured</span>
                <strong>{{ formatUpdatedAt(selectedBackup.createdAt) }}</strong>
              </div>
              <div class="detail-block detail-block--accent">
                <span>Payload</span>
                <strong>{{ selectedBackup.includesProjectFiles ? 'Database + files' : 'Database only' }}</strong>
              </div>
              <div class="detail-block">
                <span>Database</span>
                <strong>{{ selectedBackup.databaseName || 'Unknown database' }}</strong>
              </div>
              <div class="detail-block">
                <span>Archive size</span>
                <strong>{{ formatBytes(selectedBackup.sizeBytes) }}</strong>
              </div>
            </div>

            <div class="detail-section backup-detail__path">
              <span>Managed archive path</span>
              <strong class="path-row__value">{{ selectedBackup.path }}</strong>
            </div>

            <div v-if="selectedBackup.error" class="alert alert-warning mb-0">
              {{ selectedBackup.error }}
            </div>

            <p class="micro-copy mb-0">
              Restore accepts this managed file name directly. Selecting “Use for restore” preloads the restore form below without needing an absolute path.
            </p>
          </article>

          <div v-else class="empty-inline">
            No archive is selected yet. Create or import one to unlock direct download, explorer, restore, and delete actions.
          </div>
        </div>
      </div>
    </section>

    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Create or export</p>
        <h2 class="h3 mb-0">Produce new archives</h2>
      </div>

      <p class="micro-copy mb-4">
        Managed backups stay inside the environment storage tree. Export writes the same portable format to any path on the machine running the ELK-Local daemon.
      </p>

      <div class="action-card-stack">
        <article class="action-card">
          <h3 class="h5 mb-3">Create managed backup</h3>
          <p class="mb-3">Capture a fresh archive into this environment’s managed inventory so it appears immediately in the browser above.</p>
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
          <p class="mb-3">Create a portable archive outside the managed inventory when you need to hand it off or store it somewhere else.</p>
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

      <p class="micro-copy mb-4">
        Import copies an existing archive into the managed inventory. Restore can use either one of those managed file names or an absolute archive path on the daemon host.
      </p>

      <div class="action-card-stack">
        <article class="action-card">
          <h3 class="h5 mb-3">Import archive</h3>
          <p class="mb-3">Bring an external archive under management so it can be browsed, downloaded, opened, restored, or deleted from the dashboard.</p>
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
          <p class="mb-3">Recover database state, and optionally project files, into the currently selected environment.</p>
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