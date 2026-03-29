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