import { DEFAULT_PAGE_SIZE, PAGE_SIZES } from '../consts'

const STORAGE_KEY_PREFIX = 'nd.pagination.perPage.'

const sanitizeStoredValue = (value, fallback) => {
  const numeric = Number.parseInt(value, 10)
  if (Number.isFinite(numeric) && PAGE_SIZES.includes(numeric)) {
    return numeric
  }
  return fallback
}

const sanitizeFallback = (fallback) =>
  PAGE_SIZES.includes(fallback) ? fallback : DEFAULT_PAGE_SIZE

export const getStoredPageSize = (scope, fallback = DEFAULT_PAGE_SIZE) => {
  const sanitizedFallback = sanitizeFallback(fallback)
  if (!scope || typeof window === 'undefined') {
    return sanitizedFallback
  }

  try {
    const raw = window.localStorage.getItem(`${STORAGE_KEY_PREFIX}${scope}`)
    if (!raw) {
      return sanitizedFallback
    }
    return sanitizeStoredValue(raw, sanitizedFallback)
  } catch (err) {
    return sanitizedFallback
  }
}

export const setStoredPageSize = (scope, value) => {
  if (!scope || typeof window === 'undefined') {
    return
  }

  const sanitized = sanitizeStoredValue(value, DEFAULT_PAGE_SIZE)
  try {
    window.localStorage.setItem(
      `${STORAGE_KEY_PREFIX}${scope}`,
      String(sanitized),
    )
  } catch (err) {
    // Ignore persistence failures (e.g. private mode)
  }
}
