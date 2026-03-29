export interface PresetOption {
  name: string
  description: string
  applicationName?: string
  defaultAppVersion?: string
  appVersionHint?: string
  projectType: string
  phpVersion: string
  webServer: string
  databaseEngine: string
  databaseVersion: string
}

export interface EnvironmentView {
  name: string
  preset: string
  application: {
    name?: string
    version?: string
  }
  projectType: string
  projectRoot: string
  composePath: string
  storagePath: string
  phpVersion: string
  webServer: string
  updatedAt: string
  manifestPath: string
  database: {
    engine: string
    version: string
    name: string
    user: string
    host: string
    port: number
    rootUser: string
  }
  tooling: {
    adminer: { enabled: boolean; port?: number }
    mailpit: { enabled: boolean; port?: number; smtpPort?: number }
    xdebug: { enabled: boolean; clientHost?: string; clientPort?: number }
  }
  network: {
    httpPort: number
    databasePort: number
  }
  urls: {
    app?: string
    adminer?: string
    mailpit?: string
    database?: string
    smtp?: string
  }
  status: {
    state: string
    error?: string
    rawOutput?: string
    containers: Array<{
      name: string
      service: string
      state: string
      health?: string
      publishedPorts?: string[]
    }>
  }
}

export interface DashboardResponse {
  projectRoot: string
  defaultProjectRootBase: string
  environments: EnvironmentView[]
  presetOptions: PresetOption[]
}

export interface EnvironmentResponse {
  environment: EnvironmentView
  output?: string
}

export interface DeleteEnvironmentResponse {
  name: string
  removedProjectFiles?: boolean
  removedBackups?: boolean
}

export interface BackupView {
  fileName: string
  path: string
  sizeBytes: number
  createdAt: string
  environmentName?: string
  databaseName?: string
  includesProjectFiles: boolean
  error?: string
}

export interface BackupInventoryResponse {
  environmentName: string
  backups: BackupView[]
}

export interface BackupActionResponse {
  environmentName: string
  backup: BackupView
  backups?: BackupView[]
  restoredProjectFiles?: boolean
  message?: string
}

export interface CreateEnvironmentPayload {
  name: string
  preset: string
  applicationVersion: string
  projectRoot: string
  phpVersion: string
  webServer: string
  databaseEngine: string
  databaseVersion: string
  databaseName: string
  databaseUser: string
  databasePassword: string
  databaseRootPassword: string
  adminerEnabled: boolean
  mailpitEnabled: boolean
  xdebugEnabled: boolean
  force: boolean
}

export interface CreateBackupPayload {
  outputPath?: string
  includeProjectFiles: boolean
  force: boolean
}

export interface ImportBackupPayload {
  archivePath: string
  force: boolean
}

export interface RestoreBackupPayload {
  archivePath: string
  restoreProjectFiles: boolean
  force: boolean
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })

  const payload = (await response.json().catch(() => ({}))) as { error?: string }
  if (!response.ok) {
    throw new Error(payload.error ?? `Request failed with status ${response.status}`)
  }

  return payload as T
}

export function fetchDashboard() {
  return request<DashboardResponse>('/api/environments')
}

export function createEnvironment(payload: CreateEnvironmentPayload) {
  return request<EnvironmentResponse>('/api/environments', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function runEnvironmentAction(name: string, action: 'start' | 'stop' | 'destroy') {
  return request<EnvironmentResponse>(`/api/environments/${name}/actions/${action}`, {
    method: 'POST',
  })
}

export function openEnvironmentInVSCode(name: string) {
  return request<EnvironmentResponse>(`/api/environments/${name}/actions/open-editor`, {
    method: 'POST',
  })
}

export function openEnvironmentFolder(name: string) {
  return request<EnvironmentResponse>(`/api/environments/${name}/actions/open-folder`, {
    method: 'POST',
  })
}

export function deleteEnvironment(name: string) {
  return request<DeleteEnvironmentResponse>(`/api/environments/${name}`, {
    method: 'DELETE',
  })
}

export function fetchBackups(name: string) {
  return request<BackupInventoryResponse>(`/api/environments/${name}/backups`)
}

export function createManagedBackup(name: string, payload: CreateBackupPayload) {
  return request<BackupActionResponse>(`/api/environments/${name}/backups/create`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function exportBackup(name: string, payload: CreateBackupPayload) {
  return request<BackupActionResponse>(`/api/environments/${name}/backups/export`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function importBackup(name: string, payload: ImportBackupPayload) {
  return request<BackupActionResponse>(`/api/environments/${name}/backups/import`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function restoreBackup(name: string, payload: RestoreBackupPayload) {
  return request<BackupActionResponse>(`/api/environments/${name}/backups/restore`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function deleteManagedBackup(name: string, fileName: string) {
  return request<BackupActionResponse>(`/api/environments/${name}/backups/${encodeURIComponent(fileName)}`, {
    method: 'DELETE',
  })
}

export function openManagedBackupFolder(name: string, fileName: string) {
  return request<BackupActionResponse>(`/api/environments/${name}/backups/${encodeURIComponent(fileName)}/actions/open-folder`, {
    method: 'POST',
  })
}

export function downloadManagedBackupUrl(name: string, fileName: string) {
  return `/api/environments/${name}/backups/${encodeURIComponent(fileName)}/download`
}

export async function downloadManagedBackup(name: string, fileName: string) {
  const response = await fetch(downloadManagedBackupUrl(name, fileName), {
    method: 'GET',
  })

  const errorPayload = (await response.clone().json().catch(() => ({}))) as { error?: string }
  if (!response.ok) {
    throw new Error(errorPayload.error ?? `Request failed with status ${response.status}`)
  }

  const blob = await response.blob()
  const objectUrl = window.URL.createObjectURL(blob)

  const downloadLink = document.createElement('a')
  downloadLink.href = objectUrl
  downloadLink.download = extractDownloadFileName(response.headers.get('Content-Disposition'), fileName)
  downloadLink.style.display = 'none'

  document.body.appendChild(downloadLink)
  downloadLink.click()
  downloadLink.remove()

  window.setTimeout(() => {
    window.URL.revokeObjectURL(objectUrl)
  }, 1000)
}

function extractDownloadFileName(contentDisposition: string | null, fallbackFileName: string) {
  if (!contentDisposition) {
    return fallbackFileName
  }

  const encodedMatch = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (encodedMatch?.[1]) {
    return decodeURIComponent(encodedMatch[1])
  }

  const quotedMatch = contentDisposition.match(/filename="([^"]+)"/i)
  if (quotedMatch?.[1]) {
    return quotedMatch[1]
  }

  const plainMatch = contentDisposition.match(/filename=([^;]+)/i)
  if (plainMatch?.[1]) {
    return plainMatch[1].trim()
  }

  return fallbackFileName
}