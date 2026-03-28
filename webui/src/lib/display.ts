import type { BackupView, EnvironmentView } from '../api'

export function statusTone(state: string) {
  switch (state) {
    case 'running':
      return 'success'
    case 'partial':
      return 'warning'
    case 'stopped':
      return 'secondary'
    default:
      return 'danger'
  }
}

export function statusLabel(state: string) {
  switch (state) {
    case 'running':
      return 'Running'
    case 'partial':
      return 'Partially running'
    case 'stopped':
      return 'Stopped'
    default:
      return 'Needs attention'
  }
}

export function formatUpdatedAt(value: string) {
  if (!value) {
    return 'Not generated yet'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString()
}

export function toolingSummary(environment: EnvironmentView) {
  return [
    environment.tooling.adminer.enabled ? 'Adminer' : null,
    environment.tooling.mailpit.enabled ? 'Mailpit' : null,
    environment.tooling.xdebug.enabled ? 'Xdebug' : null,
  ].filter(Boolean).join(' • ') || 'No optional tooling'
}

export function formatBytes(sizeBytes: number) {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = sizeBytes
  let unitIndex = 0

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }

  const precision = value >= 10 || unitIndex === 0 ? 0 : 1
  return `${value.toFixed(precision)} ${units[unitIndex]}`
}

export function backupStatusLabel(backup: BackupView) {
  if (backup.error) {
    return 'Needs attention'
  }

  return backup.includesProjectFiles ? 'DB + project snapshot' : 'Database only'
}