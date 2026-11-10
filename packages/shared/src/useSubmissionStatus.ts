import { ref, watch } from 'vue'

type StatusContext = {
  lineId: number
  businessDate?: string
  shiftPlanItemId?: number
}

export function useSubmissionStatus(
  context: () => StatusContext | undefined,
  load: (context: StatusContext, signal: AbortSignal) => Promise<number[] | null>,
) {
  const submitted = ref<Set<number> | null>(null)
  let refresh = () => {}

  watch(context, (current, _, onCleanup) => {
    submitted.value = null
    refresh = () => {}
    if (!current) return

    let request: AbortController | undefined
    refresh = async () => {
      request?.abort()
      const controller = new AbortController()
      request = controller
      try {
        const ids = await load(current, controller.signal)
        if (!controller.signal.aborted) submitted.value = Array.isArray(ids) ? new Set(ids) : null
      } catch {
        if (!controller.signal.aborted) submitted.value = null
      }
    }

    refresh()
    const timer = window.setInterval(refresh, 30_000)
    onCleanup(() => {
      request?.abort()
      window.clearInterval(timer)
    })
  }, { immediate: true })

  return {
    isPending: (templateId: number) => submitted.value !== null && !submitted.value.has(templateId),
    refresh: () => refresh(),
  }
}
