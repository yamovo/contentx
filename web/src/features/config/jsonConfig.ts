export function formatConfig(config: Record<string, unknown> | undefined): string {
  return JSON.stringify(config || {}, null, 2)
}

export function parseConfig(source: string): { config?: Record<string, unknown>; error?: string } {
  try {
    const value: unknown = JSON.parse(source)
    if (!value || Array.isArray(value) || typeof value !== 'object') {
      return { error: '配置根节点必须是 JSON 对象' }
    }
    return { config: value as Record<string, unknown> }
  } catch {
    return { error: '配置不是合法的 JSON' }
  }
}

