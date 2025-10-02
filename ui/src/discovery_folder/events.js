export const DISCOVERY_CHANGED_EVENT = 'discovery:changed'

export const emitDiscoveryChanged = (detail = {}) => {
  try {
    window.dispatchEvent(new CustomEvent(DISCOVERY_CHANGED_EVENT, { detail }))
  } catch (error) {
    // ignore dispatch issues
  }
}

export const addDiscoveryChangedListener = (handler) => {
  window.addEventListener(DISCOVERY_CHANGED_EVENT, handler)
  return () => window.removeEventListener(DISCOVERY_CHANGED_EVENT, handler)
}
