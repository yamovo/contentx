let markedModule: typeof import('marked') | null = null
let DOMPurifyModule: typeof import('dompurify') | null = null

const FALLBACK = '<p>预览暂不可用，请继续使用 Markdown 编辑区。</p>'

/** Eagerly start loading marked + DOMPurify so the chunks begin downloading
 *  as soon as this module is imported by any consumer. */
const ready = Promise.all([
  import('marked'),
  import('dompurify'),
]).then(([m, d]) => {
  markedModule = m
  DOMPurifyModule = d
})

/** Render markdown to sanitised HTML. Synchronous once deps have loaded;
 *  returns a safe fallback string while the dynamic imports are in-flight. */
export function renderSafeMarkdown(source: string): string {
  if (!markedModule || !DOMPurifyModule) return FALLBACK
  try {
    const { marked } = markedModule
    const DOMPurify = DOMPurifyModule.default
    return DOMPurify.sanitize(marked(source || '') as string)
  } catch {
    return FALLBACK
  }
}

/** Async variant — always waits for deps, never returns the fallback. */
export async function renderSafeMarkdownAsync(source: string): Promise<string> {
  await ready
  return renderSafeMarkdown(source)
}

/** Resolve once the dynamic imports have finished. Useful for components
 *  that need to guarantee the deps are available before first render. */
export function waitForMarkdownDeps(): Promise<void> {
  return ready
}
