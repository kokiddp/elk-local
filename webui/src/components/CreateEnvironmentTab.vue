<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { createEnvironment, type CreateEnvironmentPayload, type EnvironmentView, type PresetOption } from '../api'

const props = defineProps<{
  presets: PresetOption[]
  defaultProjectRootBase: string
}>()

const emit = defineEmits<{
  'environment-updated': [environment: EnvironmentView]
  notify: [payload: { type: 'success' | 'error'; message: string }]
  'tab-change': [tab: string]
}>()

const isCreating = ref(false)

const form = reactive<CreateEnvironmentPayload>({
  name: '',
  preset: 'wordpress',
  applicationVersion: '',
  projectRoot: '',
  phpVersion: '',
  webServer: '',
  databaseEngine: '',
  databaseVersion: '',
  databaseName: '',
  databaseUser: 'elk',
  databasePassword: 'elk',
  databaseRootPassword: 'elkroot',
  adminerEnabled: true,
  mailpitEnabled: false,
  xdebugEnabled: false,
  force: false,
})

const selectedPreset = computed(() => props.presets.find(p => p.name === form.preset) ?? null)

const projectRootPreview = computed(() => {
  if (form.projectRoot.trim()) return form.projectRoot.trim()
  const base = props.defaultProjectRootBase.trim()
  if (!base) return 'daemon default'
  const name = form.name.trim() || '<name>'
  return `${base.replace(/\/+$/, '')}/${name}`
})

watch(selectedPreset, (preset, prev) => {
  if (!preset?.applicationName) { form.applicationVersion = ''; return }
  if (preset.name !== prev?.name || !form.applicationVersion) {
    form.applicationVersion = preset.defaultAppVersion ?? 'latest'
  }
}, { immediate: true })

async function handleCreate() {
  isCreating.value = true
  try {
    const res = await createEnvironment(form)
    emit('environment-updated', res.environment)
    const app = res.environment.application?.name
      ? ` ${res.environment.application.name}${res.environment.application.version ? ` ${res.environment.application.version}` : ''} installed.`
      : ''
    emit('notify', { type: 'success', message: `Created ${res.environment.name}.${app}` })
    form.name = ''
    form.projectRoot = ''
    form.databaseName = ''
    form.force = false
    emit('tab-change', 'environments')
  } catch (e) {
    emit('notify', { type: 'error', message: e instanceof Error ? e.message : 'Unable to create environment.' })
  } finally {
    isCreating.value = false
  }
}
</script>

<template>
  <div class="flex gap-12" style="align-items:start">
    <!-- Form -->
    <div class="card flex-1">
      <div class="card__header">
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="8" cy="8" r="6"/>
          <line x1="8" y1="5" x2="8" y2="11"/>
          <line x1="5" y1="8" x2="11" y2="8"/>
        </svg>
        New environment
      </div>
      <div class="card__body">
        <form class="flex-col gap-10" @submit.prevent="handleCreate">
          <!-- Name + Preset -->
          <div class="form-grid form-grid--2">
            <div class="form-row">
              <label for="f-name">Name</label>
              <input id="f-name" v-model.trim="form.name" type="text" placeholder="wordpress-demo" required/>
            </div>
            <div class="form-row">
              <label for="f-preset">Preset</label>
              <select id="f-preset" v-model="form.preset">
                <option v-for="p in presets" :key="p.name" :value="p.name">{{ p.name }}</option>
              </select>
            </div>
          </div>

          <!-- Project root -->
          <div class="form-row">
            <label for="f-root">Project root override</label>
            <input id="f-root" v-model.trim="form.projectRoot" type="text" :placeholder="projectRootPreview"/>
          </div>

          <!-- PHP + Webserver -->
          <div class="form-grid form-grid--2">
            <div class="form-row">
              <label for="f-php">PHP</label>
              <select id="f-php" v-model="form.phpVersion">
                <option value="">Preset default</option>
                <option value="7.4">7.4</option>
                <option value="8.0">8.0</option>
                <option value="8.1">8.1</option>
                <option value="8.2">8.2</option>
                <option value="8.3">8.3</option>
                <option value="8.4">8.4</option>
              </select>
            </div>
            <div class="form-row">
              <label for="f-web">Web server</label>
              <select id="f-web" v-model="form.webServer">
                <option value="">Preset default</option>
                <option value="apache">Apache</option>
                <option value="nginx">Nginx</option>
              </select>
            </div>
          </div>

          <!-- DB engine + App version -->
          <div class="form-grid form-grid--2">
            <div class="form-row">
              <label for="f-db">Database</label>
              <select id="f-db" v-model="form.databaseEngine">
                <option value="">Preset default</option>
                <option value="mariadb">MariaDB</option>
                <option value="mysql">MySQL</option>
              </select>
            </div>
            <div v-if="selectedPreset?.applicationName" class="form-row">
              <label for="f-appver">{{ selectedPreset.applicationName }} version</label>
              <input id="f-appver" v-model.trim="form.applicationVersion" type="text" :placeholder="selectedPreset.defaultAppVersion || 'latest'"/>
            </div>
          </div>

          <!-- DB credentials -->
          <div class="form-grid form-grid--2">
            <div class="form-row">
              <label for="f-dbname">DB name</label>
              <input id="f-dbname" v-model.trim="form.databaseName" type="text" placeholder="Optional"/>
            </div>
            <div class="form-row">
              <label for="f-dbuser">DB user</label>
              <input id="f-dbuser" v-model.trim="form.databaseUser" type="text"/>
            </div>
          </div>
          <div class="form-grid form-grid--2">
            <div class="form-row">
              <label for="f-dbpass">DB password</label>
              <input id="f-dbpass" v-model="form.databasePassword" type="password"/>
            </div>
            <div class="form-row">
              <label for="f-dbrootpass">Root password</label>
              <input id="f-dbrootpass" v-model="form.databaseRootPassword" type="password"/>
            </div>
          </div>

          <!-- Tooling toggles -->
          <div class="tool-toggles">
            <label class="tool-toggle">
              <input v-model="form.adminerEnabled" type="checkbox"/>
              Adminer
            </label>
            <label class="tool-toggle">
              <input v-model="form.mailpitEnabled" type="checkbox"/>
              Mailpit
            </label>
            <label class="tool-toggle">
              <input v-model="form.xdebugEnabled" type="checkbox"/>
              Xdebug
            </label>
          </div>

          <!-- Force -->
          <label class="check-label">
            <input v-model="form.force" type="checkbox"/>
            Overwrite existing ELK-Local files if present
          </label>

          <!-- Progress -->
          <div v-if="isCreating" class="progress-panel">
            <div class="progress-panel__spinner"/>
            <div>
              <div class="progress-panel__title">Creating {{ form.name || 'environment' }}…</div>
              <div class="progress-panel__body">Containers are starting, app install in progress.</div>
            </div>
          </div>

          <button type="submit" class="btn btn--primary btn--full" :disabled="isCreating">
            <svg viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.5">
              <circle cx="8" cy="8" r="6"/>
              <line x1="8" y1="5" x2="8" y2="11"/>
              <line x1="5" y1="8" x2="11" y2="8"/>
            </svg>
            {{ isCreating ? 'Creating…' : 'Create environment' }}
          </button>
        </form>
      </div>
    </div>

    <!-- Preset info sidebar -->
    <div class="card" style="width:240px; flex-shrink:0">
      <div class="card__header">
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="8" cy="8" r="6"/>
          <line x1="8" y1="7" x2="8" y2="12"/>
          <circle cx="8" cy="5" r="0.5" fill="currentColor"/>
        </svg>
        Preset defaults
      </div>
      <div v-if="selectedPreset" class="card__body flex-col gap-6">
        <div style="font-weight:600; font-size:12px">{{ selectedPreset.name }}</div>
        <div class="text-xs text-muted">{{ selectedPreset.description }}</div>
        <div class="preset-pill-row">
          <span class="preset-pill">{{ selectedPreset.projectType }}</span>
          <span class="preset-pill">PHP {{ selectedPreset.phpVersion }}</span>
          <span class="preset-pill">{{ selectedPreset.webServer }}</span>
          <span class="preset-pill">{{ selectedPreset.databaseEngine }} {{ selectedPreset.databaseVersion }}</span>
          <span v-if="selectedPreset.applicationName" class="preset-pill">
            {{ selectedPreset.applicationName }} {{ selectedPreset.defaultAppVersion }}
          </span>
        </div>
        <div class="detail-row" style="border:none; padding:0">
          <span class="detail-row__label">Root</span>
          <span class="detail-row__value">{{ projectRootPreview }}</span>
        </div>
      </div>
      <div v-else class="empty-state" style="padding:16px">No preset selected.</div>
    </div>
  </div>
</template>
