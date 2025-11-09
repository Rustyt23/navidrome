import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { mapRetailPlayerDevice } from './deviceUtils'

const buildDevicesUrl = () => '/api/retailplayer/devices'

const fetchRetailPlayerDevices = async (signal) => {
  const url = buildDevicesUrl()

  if (!url) {
    return { devices: null, enabled: false }
  }

  let payload
  try {
    const { json } = await httpClient(url, { signal })
    payload = json
  } catch (err) {
    if (err?.status === 404) {
      return { devices: null, enabled: false }
    }

    const status = typeof err?.status === 'number' ? err.status : null
    if (status) {
      throw new Error(
        `Retail player device request failed with status ${status}`,
      )
    }

    throw err
  }
  const devices = Array.isArray(payload?.data)
    ? payload.data.map(mapRetailPlayerDevice).filter(Boolean)
    : []

  return { devices, enabled: true }
}

const useRetailPlayerDevices = () => {
  const [devices, setDevices] = useState(() => {
    if (config.retailPlayerDevicesEnabled) {
      return []
    }
    return RetailPlayerMockService.listDevices()
  })
  const [error, setError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isApiEnabled, setIsApiEnabled] = useState(
    Boolean(config.retailPlayerDevicesEnabled),
  )

  useEffect(() => {
    const url = buildDevicesUrl()
    if (!url) {
      setIsApiEnabled(false)
      return undefined
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    fetchRetailPlayerDevices(abortController.signal)
      .then((result) => {
        const enabled = Boolean(result?.enabled)
        setIsApiEnabled(enabled)
        if (Array.isArray(result?.devices)) {
          setDevices(result.devices)
        } else if (!enabled) {
          setDevices(RetailPlayerMockService.listDevices())
        }
      })
      .catch((err) => {
        if (err?.name !== 'AbortError') {
          setError(err)
        }
      })
      .finally(() => {
        setIsLoading(false)
      })

    return () => {
      abortController.abort()
    }
  }, [])

  return {
    devices,
    error,
    isApiEnabled,
    isLoading,
  }
}

export default useRetailPlayerDevices
