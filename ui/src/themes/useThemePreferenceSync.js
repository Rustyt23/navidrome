import { useEffect, useRef, useState } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import httpClient from '../dataProvider/httpClient'
import { changeTheme } from '../actions'
import themes from './index'
import { AUTO_THEME_ID } from '../consts'
import config from '../config'

const resolveThemePreference = (value) => {
  if (!value) {
    return null
  }
  if (value === AUTO_THEME_ID) {
    return AUTO_THEME_ID
  }
  if (themes[value]) {
    return value
  }
  const match = Object.keys(themes).find(
    (key) => themes[key].themeName === value,
  )
  return match || null
}

const fallbackTheme =
  resolveThemePreference(config.defaultTheme) || 'DarkTheme'

const saveThemePreference = (theme) =>
  httpClient('/api/preferences', {
    method: 'PUT',
    body: JSON.stringify({ theme }),
  })

const useThemePreferenceSync = () => {
  const dispatch = useDispatch()
  const theme = useSelector((state) => state.theme)
  const themeRef = useRef(theme)
  const lastSyncedRef = useRef()
  const [initialSyncComplete, setInitialSyncComplete] = useState(false)

  useEffect(() => {
    themeRef.current = theme
  }, [theme])

  useEffect(() => {
    let cancelled = false

    const fetchPreference = async () => {
      try {
        const response = await httpClient('/api/preferences')
        const payload = response?.json ?? {}
        const serverTheme = resolveThemePreference(payload.theme)
        const isDefault = payload.isDefault ?? payload.theme === undefined
        const resolvedServerTheme = serverTheme || fallbackTheme

        if (isDefault) {
          const currentTheme = themeRef.current || fallbackTheme
          if (currentTheme !== resolvedServerTheme) {
            try {
              await saveThemePreference(currentTheme)
              lastSyncedRef.current = currentTheme
            } catch (error) {
              lastSyncedRef.current = undefined
            }
          } else {
            lastSyncedRef.current = resolvedServerTheme
          }
        } else {
          const currentTheme = themeRef.current
          if (resolvedServerTheme && resolvedServerTheme !== currentTheme) {
            dispatch(changeTheme(resolvedServerTheme))
            lastSyncedRef.current = resolvedServerTheme
          } else {
            lastSyncedRef.current = currentTheme || resolvedServerTheme
          }
        }
      } catch (error) {
        lastSyncedRef.current = undefined
      } finally {
        if (!cancelled) {
          setInitialSyncComplete(true)
        }
      }
    }

    fetchPreference()

    return () => {
      cancelled = true
    }
  }, [dispatch])

  useEffect(() => {
    if (!initialSyncComplete) {
      return
    }
    const currentTheme = themeRef.current || fallbackTheme
    if (!currentTheme) {
      return
    }
    if (lastSyncedRef.current === currentTheme) {
      return
    }
    lastSyncedRef.current = currentTheme
    saveThemePreference(currentTheme).catch(() => {
      lastSyncedRef.current = undefined
    })
  }, [theme, initialSyncComplete])
}

export default useThemePreferenceSync

