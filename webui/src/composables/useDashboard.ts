import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { fetchDashboard, type DashboardResponse, type EnvironmentView } from '../api'

export function useDashboard(refreshIntervalMs = 5000) {
  const dashboard = ref<DashboardResponse | null>(null)
  const isLoading = ref(true)
  const isRefreshing = ref(false)
  const feedback = ref('')
  const errorMessage = ref('')

  const environments = computed(() => dashboard.value?.environments ?? [])
  const presets = computed(() => dashboard.value?.presetOptions ?? [])
  const projectRoot = computed(() => dashboard.value?.projectRoot ?? 'Loading project root...')
  const defaultProjectRootBase = computed(() => dashboard.value?.defaultProjectRootBase ?? '')
  const runningCount = computed(() => environments.value.filter((environment) => environment.status.state === 'running').length)

  let refreshTimer: number | undefined

  function clearMessages() {
    feedback.value = ''
    errorMessage.value = ''
  }

  function setFeedback(message: string) {
    feedback.value = message
    errorMessage.value = ''
  }

  function setError(message: string) {
    errorMessage.value = message
    feedback.value = ''
  }

  function replaceEnvironment(nextEnvironment: EnvironmentView) {
    if (!dashboard.value) {
      return
    }

    const nextEnvironments = [...dashboard.value.environments]
    const index = nextEnvironments.findIndex((environment) => environment.name === nextEnvironment.name)

    if (index >= 0) {
      nextEnvironments.splice(index, 1, nextEnvironment)
    } else {
      nextEnvironments.push(nextEnvironment)
    }

    nextEnvironments.sort((left, right) => left.name.localeCompare(right.name))
    dashboard.value = {
      ...dashboard.value,
      environments: nextEnvironments,
    }
  }

  function removeEnvironment(name: string) {
    if (!dashboard.value) {
      return
    }

    dashboard.value = {
      ...dashboard.value,
      environments: dashboard.value.environments.filter((environment) => environment.name !== name),
    }
  }

  async function loadDashboard(options?: { silent?: boolean }) {
    if (options?.silent) {
      isRefreshing.value = true
    } else {
      isLoading.value = true
      errorMessage.value = ''
    }

    try {
      dashboard.value = await fetchDashboard()
    } catch (error) {
      setError(error instanceof Error ? error.message : 'Unable to load environments.')
    } finally {
      isLoading.value = false
      isRefreshing.value = false
    }
  }

  onMounted(() => {
    void loadDashboard()
    refreshTimer = window.setInterval(() => {
      void loadDashboard({ silent: true })
    }, refreshIntervalMs)
  })

  onBeforeUnmount(() => {
    if (refreshTimer !== undefined) {
      window.clearInterval(refreshTimer)
    }
  })

  return {
    dashboard,
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
  }
}