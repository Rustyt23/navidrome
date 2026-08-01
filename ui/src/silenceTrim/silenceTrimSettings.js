export const DEFAULT_SILENCE_MARGIN_SECONDS = 0.1
export const SILENCE_MARGIN_STORAGE_KEY = 'silenceTrim.marginSeconds'
export const SILENCE_MARGIN_OPTIONS = [0, 0.05, 0.1, 0.2, 0.5, 1]

export const getSilenceTrimMargin = () => {
  if (typeof window === 'undefined') return DEFAULT_SILENCE_MARGIN_SECONDS
  const stored = Number(window.localStorage.getItem(SILENCE_MARGIN_STORAGE_KEY))
  return Number.isFinite(stored) && stored >= 0 && stored <= 2
    ? stored
    : DEFAULT_SILENCE_MARGIN_SECONDS
}
