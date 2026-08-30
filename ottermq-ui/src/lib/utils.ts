export function formatBytes(bytes: number) {
  if (!bytes) return '-'
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${sizes[i]}`
}

export function stateColor(state: string): string {
  switch (state) {
    case 'running': return 'green';
    case 'idle': return 'yellow';
    case 'error': return 'red';
    default: return 'gray';
  }
}