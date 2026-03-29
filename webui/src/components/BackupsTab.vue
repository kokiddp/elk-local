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

const props = defineProps<{ environments: EnvironmentView[] }>()

const emit = defineEmits<{
  notify: [payload: { type: 'success' | 'error'; message: string }]
  'refresh-environments': []
}>()

const selectedEnvName = ref('')
const selectedBackupPath = ref('')
const backups = ref<BackupView[]>([])
const isLoadingBackups = ref(false)
const inventoryError = ref('')
const busyAction = ref('')

const createForm = reactive({ includeProjectFiles: false, force: false })
const exportForm = reactive({ outputPath: '', includeProjectFiles: false, force: false })
const importForm = reactive({ archivePath: '', force: false })
const restoreForm = reactive({ archivePath: '', restoreProjectFiles: false, force: false })

const selectedEnv = computed(() => props.environments.find(e => e.name === selectedEnvName.value) ?? null)
const selectedBackup = computed(() => backups.value.find(b => b.path === selectedBackupPath.value) ?? backups.value[0] ?? null)
const totalSize = computed(() => backups.value.reduce((t, b) => t + b.sizeBytes, 0))
const snapshotCount = computed(() => backups.value.filter(b => b.includesProjectFiles).length)

watch(() => props.environments, (envs) => {
  if (!envs.length) { selectedEnvName.value = ''; backups.value = []; return }
  const still = envs.some(e => e.name === selectedEnvName.value)
  if (!selectedEnvName.value || !still) selectedEnvName.value = envs[0].name
}, { immediate: true, deep: true })

watch(selectedEnvName, () => void loadInventory(), { immediate: true })

watch(backups, (items) => {
  if (items.some(b => b.path === selectedBackupPath.value)) return
  selectedBackupPath.value = items[0]?.path ?? ''
}, { deep: true })

async function loadInventory() {
  if (!selectedEnvName.value) { backups.value = []; inventoryError.value = ''; return }
  isLoadingBackups.value = true
  inventoryError.value = ''
  try {
    const res = await fetchBackups(selectedEnvName.value)
    backups.value = res.backups
  } catch (e) {
    inventoryError.value = e instanceof Error ? e.message : 'Failed to load backups.'
  } finally {
    isLoadingBackups.value = false
  }
}

function applyResponse(res: BackupActionResponse, msg: string) {
  backups.value = res.backups ?? backups.value
  selectedBackupPath.value = res.backup?.path ?? selectedBackupPath.value
  emit('notify', { type: 'success', message: res.message || msg })
  emit('refresh-environments')
}

function isBusy(name: string) { return busyAction.value === name }

async function runAction(name: string, fn: () => Promise<BackupActionResponse>) {
  busyAction.value = name
  try { applyResponse(await fn(), 'Done.') }
  catch (e) { emit('notify', { type: 'error', message: e instanceof Error ? e.message : 'Action failed.' }) }
  finally { busyAction.value = '' }
}

function ensureEnv() {
  if (!selectedEnvName.value) {
    emit('notify', { type: 'error', message: 'Choose an environment first.' })
    return false
  }
  return true
}

async function handleCreate() {
  if (!ensureEnv()) return
  await runAction('create', () => createManagedBackup(selectedEnvName.value, { ...createForm }))
}

async function handleExport() {
  if (!ensureEnv()) return
  if (!exportForm.outputPath.trim()) {
    emit('notify', { type: 'error', message: 'Provide an output path.' })
    return
  }
  await runAction('export', () => exportBackup(selectedEnvName.value, { ...exportForm }))
}

async function handleImport() {
  if (!ensureEnv()) return
  if (!importForm.archivePath.trim()) {
    emit('notify', { type: 'error', message: 'Provide archive path.' })
    return
  }
  await runAction('import', () => importBackup(selectedEnvName.value, { ...importForm }))
}

async function handleRestore() {
  if (!ensureEnv()) return
  if (!restoreForm.archivePath.trim()) {
    emit('notify', { type: 'error', message: 'Provide archive path or file name.' })
    return
  }
  if (!restoreForm.force) {
    emit('notify', { type: 'error', message: 'Check the confirmation box to allow restore.' })
    return
  }
  await runAction('restore', () => restoreBackup(selectedEnvName.value, { ...restoreForm }))
}

async function handleOpenFolder(backup: BackupView) {
  if (!ensureEnv()) return
  await runAction(`open-folder:${backup.fileName}`, () => openManagedBackupFolder(selectedEnvName.value, backup.fileName))
}

async function handleDownload(backup: BackupView) {
  if (!ensureEnv()) return
  busyAction.value = `download:${backup.fileName}`
  try {
    await downloadManagedBackup(selectedEnvName.value, backup.fileName)
    emit('notify', { type: 'success', message: `Downloaded ${backup.fileName}` })
  } catch (e) {
    emit('notify', { type: 'error', message: e instanceof Error ? e.message : 'Download failed.' })
  } finally { busyAction.value = '' }
}

async function handleDelete(backup: BackupView) {
  if (!ensureEnv()) return
  if (!window.confirm(`Delete ${backup.fileName}? Cannot be undone.`)) return
  await runAction(`delete:${backup.fileName}`, () => deleteManagedBackup(selectedEnvName.value, backup.fileName))
}

function useForRestore(backup: BackupView) {
  selectedBackupPath.value = backup.path
  restoreForm.archivePath = backup.fileName || backup.path
}

function backupBadge(backup: BackupView) {
  if (backup.error) return 'badge--unknown'
  return backup.includesProjectFiles ? 'badge--full-snap' : 'badge--db-only'
}

function backupBadgeLabel(backup: BackupView) {
  if (backup.error) return 'Error'
  return backup.includesProjectFiles ? 'Full' : 'DB'
}
</script>

<template>
  <div v-if="environments.length === 0" class="empty-state">
    Create an environment first.
  </div>

  <div v-else class="flex gap-12" style="align-items:start">
    <!-- Left: inventory -->
    <div class="flex-col gap-8 flex-1">
      <!-- Env picker + stats row -->
      <div class="card">
        <div class="card__header">
          <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M8 2a6 6 0 1 0 0 12A6 6 0 0 0 8 2z"/>
            <path d="M8 6v4M6 8h4"/>
          </svg>
          Backups
          <div class="card__header-spacer"/>
          <select v-model="selectedEnvName" style="width:160px">
            <option v-for="env in environments" :key="env.name" :value="env.name">{{ env.name }}</option>
          </select>
          <button
            type="button"
            class="icon-btn"
            :disabled="isLoadingBackups"
            data-tooltip="Refresh inventory"
            @click="loadInventory"
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"
                 :style="isLoadingBackups ? 'animation: spin 0.7s linear infinite' : ''">
              <path d="M13 8a5 5 0 1 1-1.2-3.2l1.2-1.2V7h-3.5"/>
            </svg>
          </button>
        </div>
        <div class="card__body">
          <div class="stat-row">
            <div class="stat-chip stat-chip--accent">
              <span class="stat-chip__label">Archives</span>
              <span class="stat-chip__value">{{ backups.length }}</span>
            </div>
            <div class="stat-chip stat-chip--green">
              <span class="stat-chip__label">Full snapshots</span>
              <span class="stat-chip__value">{{ snapshotCount }}</span>
            </div>
            <div class="stat-chip">
              <span class="stat-chip__label">Total size</span>
              <span class="stat-chip__value" style="font-size:14px">{{ formatBytes(totalSize) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Backup list -->
      <div class="list-box list-box--scroll">
        <div v-if="inventoryError" class="toast toast--error" style="position:static; margin:8px; pointer-events:auto">
          {{ inventoryError }}
        </div>
        <div v-if="backups.length === 0 && !isLoadingBackups" class="empty-state">No archives yet.</div>
        <button
          v-for="backup in backups"
          :key="backup.path"
          type="button"
          class="backup-item"
          :class="{ 'backup-item--active': selectedBackup?.path === backup.path }"
          @click="selectedBackupPath = backup.path"
        >
          <span class="badge" :class="backupBadge(backup)">{{ backupBadgeLabel(backup) }}</span>
          <span class="backup-item__name">{{ backup.fileName }}</span>
          <span class="backup-item__size">{{ formatBytes(backup.sizeBytes) }}</span>
          <span class="backup-item__date">{{ formatUpdatedAt(backup.createdAt) }}</span>
        </button>
      </div>

      <!-- Selected backup detail + actions -->
      <div v-if="selectedBackup" class="card">
        <div class="card__header">
          <span style="font-size:12px; font-weight:600; font-family:var(--mono)">{{ selectedBackup.fileName }}</span>
          <span class="badge" :class="backupBadge(selectedBackup)">{{ backupBadgeLabel(selectedBackup) }}</span>
          <div class="card__header-spacer"/>
          <button
            type="button"
            class="icon-btn icon-btn--accent"
            :disabled="isBusy(`download:${selectedBackup.fileName}`)"
            data-tooltip="Download"
            @click="handleDownload(selectedBackup)"
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M8 2v9M5 8l3 3 3-3"/>
              <path d="M2 13h12"/>
            </svg>
          </button>
          <button
            type="button"
            class="icon-btn"
            :disabled="isBusy(`open-folder:${selectedBackup.fileName}`)"
            data-tooltip="Open folder"
            @click="handleOpenFolder(selectedBackup)"
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M2 4h4l1 2h7v8H2V4z"/>
            </svg>
          </button>
          <button
            type="button"
            class="icon-btn icon-btn--amber"
            :disabled="Boolean(selectedBackup.error)"
            data-tooltip="Use for restore"
            @click="useForRestore(selectedBackup)"
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M3 8a5 5 0 1 0 1.2-3.2"/>
              <polyline points="3,4 3,8 7,8"/>
            </svg>
          </button>
          <button
            type="button"
            class="icon-btn icon-btn--red"
            :disabled="isBusy(`delete:${selectedBackup.fileName}`)"
            data-tooltip="Delete archive"
            @click="handleDelete(selectedBackup)"
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
              <polyline points="3,5 4,14 12,14 13,5"/>
              <line x1="2" y1="5" x2="14" y2="5"/>
              <line x1="6" y1="5" x2="6" y2="3"/>
              <line x1="10" y1="5" x2="10" y2="3"/>
              <line x1="6" y1="3" x2="10" y2="3"/>
            </svg>
          </button>
        </div>
        <div class="card__body">
          <div class="flex gap-8" style="flex-wrap:wrap">
            <div class="detail-row flex-1" style="border:none; padding:0">
              <span class="detail-row__label">Captured</span>
              <span class="detail-row__value">{{ formatUpdatedAt(selectedBackup.createdAt) }}</span>
            </div>
            <div class="detail-row flex-1" style="border:none; padding:0">
              <span class="detail-row__label">DB</span>
              <span class="detail-row__value">{{ selectedBackup.databaseName || '—' }}</span>
            </div>
            <div class="detail-row flex-1" style="border:none; padding:0">
              <span class="detail-row__label">Size</span>
              <span class="detail-row__value">{{ formatBytes(selectedBackup.sizeBytes) }}</span>
            </div>
          </div>
          <div class="path-text mt-8">{{ selectedBackup.path }}</div>
          <div v-if="selectedBackup.error" class="toast toast--error" style="position:static; margin-top:8px">
            {{ selectedBackup.error }}
          </div>
        </div>
      </div>
    </div>

    <!-- Right: actions -->
    <div class="flex-col gap-8" style="width:260px; flex-shrink:0">
      <!-- Create -->
      <div class="action-card">
        <div class="action-card__title">Create managed backup</div>
        <label class="check-label">
          <input v-model="createForm.includeProjectFiles" type="checkbox"/>
          Include project files
        </label>
        <label class="check-label">
          <input v-model="createForm.force" type="checkbox"/>
          Overwrite if exists
        </label>
        <button type="button" class="btn btn--primary btn--full" :disabled="isBusy('create')" @click="handleCreate">
          {{ isBusy('create') ? 'Creating…' : 'Create backup' }}
        </button>
      </div>

      <!-- Export -->
      <div class="action-card">
        <div class="action-card__title">Export backup</div>
        <div class="form-row">
          <label>Output path</label>
          <input v-model.trim="exportForm.outputPath" type="text" placeholder="/tmp/my-site.tar.gz"/>
        </div>
        <label class="check-label">
          <input v-model="exportForm.includeProjectFiles" type="checkbox"/>
          Include project files
        </label>
        <label class="check-label">
          <input v-model="exportForm.force" type="checkbox"/>
          Overwrite if exists
        </label>
        <button type="button" class="btn btn--ghost btn--full" :disabled="isBusy('export')" @click="handleExport">
          {{ isBusy('export') ? 'Exporting…' : 'Export archive' }}
        </button>
      </div>

      <!-- Import -->
      <div class="action-card">
        <div class="action-card__title">Import archive</div>
        <div class="form-row">
          <label>Archive path</label>
          <input v-model.trim="importForm.archivePath" type="text" placeholder="/tmp/my-site.tar.gz"/>
        </div>
        <label class="check-label">
          <input v-model="importForm.force" type="checkbox"/>
          Overwrite existing
        </label>
        <button type="button" class="btn btn--ghost btn--full" :disabled="isBusy('import')" @click="handleImport">
          {{ isBusy('import') ? 'Importing…' : 'Import into inventory' }}
        </button>
      </div>

      <!-- Restore -->
      <div class="action-card action-card--danger">
        <div class="action-card__title">Restore archive</div>
        <div class="form-row">
          <label>File name or path</label>
          <input v-model.trim="restoreForm.archivePath" type="text" placeholder="my-site-20260328.tar.gz"/>
        </div>
        <label class="check-label">
          <input v-model="restoreForm.restoreProjectFiles" type="checkbox"/>
          Restore project files
        </label>
        <label class="check-label check-label--danger">
          <input v-model="restoreForm.force" type="checkbox"/>
          I understand this replaces the database
        </label>
        <button type="button" class="btn btn--danger btn--full" :disabled="isBusy('restore')" @click="handleRestore">
          {{ isBusy('restore') ? 'Restoring…' : 'Restore' }}
        </button>
      </div>
    </div>
  </div>
</template>
