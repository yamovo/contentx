import { onUnmounted } from 'vue'
import { animate as animeAnimate } from 'animejs'

type AnimateFn = typeof animeAnimate

/**
 * Wrapper around animejs `animate()` that tracks every created animation and
 * cancels them all when the component unmounts.
 *
 * Without this, `loop: true` animations keep the anime engine's rAF loop
 * running forever against detached DOM nodes — every visit to a page that
 * starts such an animation leaks CPU and memory (audit P0).
 */
export function useAnime() {
  const instances: ReturnType<AnimateFn>[] = []

  const animate: AnimateFn = (targets, params) => {
    const instance = animeAnimate(targets, params)
    instances.push(instance)
    return instance
  }

  onUnmounted(() => {
    for (const instance of instances) {
      instance.cancel()
    }
    instances.length = 0
  })

  return { animate }
}
