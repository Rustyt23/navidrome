import { useCallback, useEffect, useState } from 'react'

let raUseStore

try {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  raUseStore = require('ra-core').useStore
} catch (error) {
  raUseStore = undefined
}

const readInitialValue = (key, defaultValue) => {
  if (typeof window === 'undefined' || !window.localStorage) {
    return defaultValue
  }
  const stored = window.localStorage.getItem(key)
  if (stored === null) {
    return defaultValue
  }
  const parsed = parseInt(stored, 10)
  return Number.isNaN(parsed) ? defaultValue : parsed
}

const useFallbackStore = (key, defaultValue) => {
  const [value, setValueState] = useState(() =>
    readInitialValue(key, defaultValue),
  )

  const setValue = useCallback(
    (newValue) => {
      setValueState(newValue)
      if (typeof window !== 'undefined' && window.localStorage) {
        window.localStorage.setItem(key, newValue.toString())
      }
    },
    [key],
  )

  useEffect(() => {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem(key, value.toString())
    }
  }, [key, value])

  return [value, setValue]
}

const useStoreHook =
  typeof raUseStore === 'function' ? raUseStore : useFallbackStore

export const usePerPagePreference = (key, defaultValue = 50) => {
  const [perPage, setPerPage] = useStoreHook(key, defaultValue)
  return { perPage, setPerPage }
}

export default usePerPagePreference
