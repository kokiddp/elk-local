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

const createProgressTitle = computed(() => {
  const environmentName = createForm.name.trim() || 'your environment'
  return `Creating ${environmentName}`
})

const createProgressSteps = computed(() => {
  const projectRoot = defaultProjectRootPreview.value
  const applicationName = selectedPreset.value?.applicationName || 'the application'

  return [
    'Writing the manifest and Compose files for this stack.',
    'Preparing or starting the required containers. First-run image pulls can take a while.',
    `Installing ${applicationName} into ${projectRoot} and syncing config files.`,
  ]
})

const createForm = reactive<CreateEnvironmentPayload>({
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

const selectedPreset = computed(() => props.presets.find((preset) => preset.name === createForm.preset) ?? null)

const defaultProjectRootPreview = computed(() => {
  const explicitProjectRoot = createForm.projectRoot.trim()
  if (explicitProjectRoot) {
    return explicitProjectRoot
  }

  const basePath = props.defaultProjectRootBase.trim()
  if (!basePath) {
    return 'the daemon default project root'
  }

  const environmentName = createForm.name.trim() || '<name>'
  return joinProjectRoot(basePath, environmentName)
})

watch(
  selectedPreset,
  (preset, previousPreset) => {
    if (!preset?.applicationName) {
      createForm.applicationVersion = ''
      return
    }

    if (preset.name !== previousPreset?.name || !createForm.applicationVersion) {
      createForm.applicationVersion = preset.defaultAppVersion ?? 'latest'
    }
  },
  { immediate: true },
)

async function handleCreate() {
  isCreating.value = true

  try {
    const response = await createEnvironment(createForm)
    emit('environment-updated', response.environment)

    const installedApplication = response.environment.application?.name
      ? ` Installed ${response.environment.application.name}${response.environment.application.version ? ` ${response.environment.application.version}` : ''}.`
      : ''

    emit('notify', {
      type: 'success',
      message: `Created ${response.environment.name}.${installedApplication} The Environments tab is ready for lifecycle checks.`,
    })

    createForm.name = ''
    createForm.projectRoot = ''
    createForm.databaseName = ''
    createForm.force = false
    emit('tab-change', 'environments')
  } catch (error) {
    emit('notify', {
      type: 'error',
      message: error instanceof Error ? error.message : 'Unable to create environment.',
    })
  } finally {
    isCreating.value = false
  }
}

function joinProjectRoot(basePath: string, environmentName: string) {
  const normalizedBasePath = basePath.replace(/\/+$/, '')
  const normalizedEnvironmentName = environmentName.replace(/^\/+/, '')

  if (!normalizedBasePath) {
    return normalizedEnvironmentName
  }

  if (!normalizedEnvironmentName) {
    return normalizedBasePath
  }

  return `${normalizedBasePath}/${normalizedEnvironmentName}`
}
</script>

<template>
  <div class="tab-grid tab-grid--create">
    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Create environment</p>
        <h2 class="h3 mb-0">Generate the stack and install the app in one pass</h2>
      </div>

      <div v-if="selectedPreset" class="preset-summary-card mb-4">
        <div>
          <strong>{{ selectedPreset.name }}</strong>
          <p class="mb-2">{{ selectedPreset.description }}</p>
        </div>
        <div class="preset-summary-card__meta">
          <span>{{ selectedPreset.projectType }}</span>
          <span>PHP {{ selectedPreset.phpVersion }}</span>
          <span>{{ selectedPreset.webServer }}</span>
          <span>{{ selectedPreset.databaseEngine }} {{ selectedPreset.databaseVersion }}</span>
          <span v-if="selectedPreset.applicationName">{{ selectedPreset.applicationName }} {{ selectedPreset.defaultAppVersion }}</span>
        </div>
      </div>

      <form class="form-stack" @submit.prevent="handleCreate">
        <div class="form-grid form-grid--two">
          <div>
            <label class="form-label" for="env-name">Environment name</label>
            <input id="env-name" v-model.trim="createForm.name" class="form-control form-control-lg" placeholder="wordpress-demo" required />
          </div>

          <div>
            <label class="form-label" for="env-preset">Preset</label>
            <select id="env-preset" v-model="createForm.preset" class="form-select">
              <option v-for="preset in presets" :key="preset.name" :value="preset.name">
                {{ preset.name }}
              </option>
            </select>
          </div>
        </div>

        <div>
          <label class="form-label" for="env-project-root">Project root override</label>
          <input
            id="env-project-root"
            v-model.trim="createForm.projectRoot"
            class="form-control"
            :placeholder="defaultProjectRootPreview"
          />
          <div class="form-text">Leave blank to create the application in {{ defaultProjectRootPreview }}.</div>
        </div>

        <div class="form-grid form-grid--two">
          <div>
            <label class="form-label" for="env-php">PHP version</label>
            <select id="env-php" v-model="createForm.phpVersion" class="form-select">
              <option value="">Preset default</option>
              <option value="7.4">7.4</option>
              <option value="8.0">8.0</option>
              <option value="8.1">8.1</option>
              <option value="8.2">8.2</option>
              <option value="8.3">8.3</option>
              <option value="8.4">8.4</option>
            </select>
          </div>

          <div>
            <label class="form-label" for="env-webserver">Web server</label>
            <select id="env-webserver" v-model="createForm.webServer" class="form-select">
              <option value="">Preset default</option>
              <option value="apache">Apache</option>
              <option value="nginx">Nginx</option>
            </select>
          </div>
        </div>

        <div class="form-grid form-grid--two">
          <div>
            <label class="form-label" for="env-database">Database engine</label>
            <select id="env-database" v-model="createForm.databaseEngine" class="form-select">
              <option value="">Preset default</option>
              <option value="mariadb">MariaDB</option>
              <option value="mysql">MySQL</option>
            </select>
          </div>

          <div v-if="selectedPreset?.applicationName">
            <label class="form-label" for="env-app-version">{{ selectedPreset.applicationName }} version</label>
            <input
              id="env-app-version"
              v-model.trim="createForm.applicationVersion"
              class="form-control"
              :placeholder="selectedPreset.defaultAppVersion || 'latest'"
            />
            <div class="form-text">
              {{ selectedPreset.appVersionHint || `Leave blank to install ${selectedPreset.applicationName} ${selectedPreset.defaultAppVersion || 'latest'}.` }}
            </div>
          </div>
        </div>

        <div class="form-grid form-grid--two">
          <div>
            <label class="form-label" for="env-db-name">DB name</label>
            <input id="env-db-name" v-model.trim="createForm.databaseName" class="form-control" placeholder="Optional override" />
          </div>

          <div>
            <label class="form-label" for="env-db-user">DB user</label>
            <input id="env-db-user" v-model.trim="createForm.databaseUser" class="form-control" />
          </div>
        </div>

        <div class="form-grid form-grid--two">
          <div>
            <label class="form-label" for="env-db-password">DB password</label>
            <input id="env-db-password" v-model="createForm.databasePassword" class="form-control" type="password" />
          </div>

          <div>
            <label class="form-label" for="env-db-root-password">DB root password</label>
            <input id="env-db-root-password" v-model="createForm.databaseRootPassword" class="form-control" type="password" />
          </div>
        </div>

        <div class="tool-grid">
          <label class="tool-toggle">
            <input v-model="createForm.adminerEnabled" type="checkbox" />
            <span>Adminer</span>
          </label>
          <label class="tool-toggle">
            <input v-model="createForm.mailpitEnabled" type="checkbox" />
            <span>Mailpit</span>
          </label>
          <label class="tool-toggle">
            <input v-model="createForm.xdebugEnabled" type="checkbox" />
            <span>Xdebug</span>
          </label>
        </div>

        <label class="check-row">
          <input v-model="createForm.force" type="checkbox" />
          <span>Overwrite generated ELK-Local files in an existing environment directory if needed.</span>
        </label>

        <div v-if="isCreating" class="create-progress-panel" aria-live="polite">
          <div class="create-progress-panel__header">
            <span class="create-progress-panel__indicator" aria-hidden="true"></span>
            <div>
              <strong>{{ createProgressTitle }}</strong>
              <p class="micro-copy mb-0">
                ELK-Local keeps this request open until the environment is generated, containers are ready, and the preset install finishes.
              </p>
            </div>
          </div>

          <ol class="create-progress-list mb-0">
            <li v-for="step in createProgressSteps" :key="step">
              {{ step }}
            </li>
          </ol>
        </div>

        <button type="submit" class="btn btn-dark btn-lg w-100" :disabled="isCreating">
          {{ isCreating ? 'Creating environment…' : 'Create environment' }}
        </button>
      </form>
    </section>

    <section class="surface-panel tab-section">
      <div class="section-heading mb-4">
        <p class="eyebrow mb-2">Workflow notes</p>
        <h2 class="h3 mb-0">What the create flow does now</h2>
      </div>

      <div class="feature-grid feature-grid--single">
        <article class="feature-card feature-card--accent">
          <strong>Application install</strong>
          <span>Installable presets populate the project root rather than leaving you with an empty mounted directory.</span>
        </article>
        <article class="feature-card">
          <strong>Config synchronization</strong>
          <span>Database credentials are written into the generated manifest, Compose config, and supported app config files.</span>
        </article>
        <article class="feature-card">
          <strong>Version-aware WordPress</strong>
          <span>Stable, nightly, beta, and RC WordPress builds can be selected directly from this form.</span>
        </article>
      </div>
    </section>
  </div>
</template>