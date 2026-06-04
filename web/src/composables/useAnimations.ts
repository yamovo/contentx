import { onMounted, onUnmounted, type Ref } from 'vue'
import { animate, createTimeline } from 'animejs'
import { stagger } from 'animejs/utils'

/**
 * Entrance animation: fade-in + slide-up for a set of elements.
 */
export function useStaggerEntrance(
  selector: string,
  opts: { delay?: number; duration?: number; y?: number } = {},
) {
  const { delay = 80, duration = 600, y = 30 } = opts

  onMounted(() => {
    animate(selector, {
      opacity: { from: 0 },
      translateY: { from: y },
      duration,
      delay: stagger(delay),
      ease: 'outQuint',
    })
  })
}

/**
 * Page transition – animate route changes.
 */
export function usePageTransition(container: Ref<HTMLElement | null>) {
  onMounted(() => {
    if (!container.value) return
    animate(container.value, {
      opacity: { from: 0 },
      translateX: { from: 16 },
      duration: 400,
      ease: 'outQuint',
    })
  })
}

export { stagger, createTimeline }
