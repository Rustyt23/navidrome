const LIBRARY_MUTATED_EVENT = 'nd:library-mutated'

export const emitLibraryMutated = (detail = {}) => {
  if (typeof window === 'undefined' || typeof window.dispatchEvent !== 'function') {
    return
  }
  try {
    window.dispatchEvent(
      new CustomEvent(LIBRARY_MUTATED_EVENT, {
        detail: { timestamp: Date.now(), ...detail },
      }),
    )
  } catch (error) {
    // Silently ignore errors when dispatching custom events
  }
}

export const subscribeLibraryMutated = (handler) => {
  if (typeof window === 'undefined' || typeof window.addEventListener !== 'function') {
    return () => {}
  }
  window.addEventListener(LIBRARY_MUTATED_EVENT, handler)
  return () => {
    window.removeEventListener(LIBRARY_MUTATED_EVENT, handler)
  }
}

export const getLibraryMutatedEventName = () => LIBRARY_MUTATED_EVENT
