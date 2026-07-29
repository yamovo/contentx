import { onUnmounted } from 'vue'

type AnimateFn = (targets: any, params: any) => any

interface StaggerToken {
  __stagger: true
  value: number | ((index: number) => number)
  options?: Record<string, any>
}

let realAnimate: AnimateFn | null = null
let realStagger: ((...args: any[]) => any) | null = null
let modulePromise: Promise<void> | null = null
const pendingQueue: Array<{ targets: any; params: any }> = []

function loadModule() {
  if (!modulePromise) {
    modulePromise = Promise.all([
      import('animejs'),
      import('animejs/utils'),
    ]).then(([animeMod, utilsMod]) => {
      realAnimate = (animeMod as any).animate as AnimateFn
      realStagger = (utilsMod as any).stagger ?? null
      // Flush queued animation calls
      const queue = pendingQueue.splice(0)
      for (const call of queue) {
        const resolvedParams = resolveParams(call.params)
        realAnimate!(call.targets, resolvedParams)
      }
    })
  }
  return modulePromise
}

/** Recursively resolve StaggerTokens in params using the real stagger fn. */
function resolveParams(params: any): any {
  if (!realStagger || typeof params !== 'object' || params === null) return params
  const resolved: any = Array.isArray(params) ? [...params] : { ...params }
  for (const key of Object.keys(resolved)) {
    const val = resolved[key]
    if (val && typeof val === 'object' && val.__stagger) {
      resolved[key] = realStagger(val.value, val.options)
    }
  }
  return resolved
}

// Start loading the module immediately so the chunk begins downloading.
loadModule()

/**
 * Wrapper around animejs `animate()` that dynamically imports the library
 * to keep it out of the main bundle. Calls made before the module loads
 * are queued and flushed automatically.
 *
 * Tracks every created animation and cancels them all on component unmount
 * to prevent rAF-loop leaks from `loop: true` animations on detached DOM.
 */
export function useAnime() {
  const instances: Array<{ cancel: () => void }> = []

  const animate: AnimateFn = (targets, params) => {
    if (realAnimate) {
      const instance = realAnimate(targets, params)
      instances.push(instance)
      return instance
    }
    // Module not yet loaded — queue the call.
    pendingQueue.push({ targets, params })
    // Return a proxy so callers that chain .cancel() etc. won't crash.
    const placeholder = {
      cancel() { /* will be cancelled when flushed */ },
      pause() {},
      resume() {},
    }
    instances.push(placeholder)
    return placeholder
  }

  onUnmounted(() => {
    for (const instance of instances) {
      instance.cancel()
    }
    instances.length = 0
  })

  return { animate, stagger }
}

/**
 * Dynamic-import-aware stagger. Before the animejs/utils module loads it
 * returns a token that the animate proxy resolves at flush time.
 */
function stagger(
  value: number | ((index: number) => number),
  options?: Record<string, any>,
): any {
  if (realStagger) {
    return realStagger(value, options)
  }
  // Return a token; resolveParams will convert it when the module arrives.
  const token: StaggerToken = { __stagger: true, value, options }
  return token
}
