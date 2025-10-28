import { useEffect, useMemo, useState } from 'react'
import config from '../config'
import RetailPlayerMockService from './RetailPlayerMockService'

const buildDevicesUrl = () => {
  if (!config.retailPlayerDevicesEnabled) {
    return null
  }

  const basePath = (config.baseURL || '').replace(/\/+$/, '')
  if (!basePath) {
    return '/api/native/retailplayer/devices'
  }

  return `${basePath}/api/native/retailplayer/devices`
}

const mapDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const id = device.id || device.macAddress || device.ordinal?.toString()

  if (!id) {
    return null
  }

  return {
    id,
    name: device.name || id,
    channel: device.channel || '',
    channelList: device.channelList || '',
    organization: device.organization || device.orgUnit || device.location || '',
  }
}

const fetchRetailPlayerDevices = async (signal) => {
  const url = buildDevicesUrl()

  if (!url) {
    return null
  }

  const response = await fetch(url, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    signal,
  })

  if (!response.ok) {
    throw new Error(
      `Retail player device request failed with status ${response.status}`,
    )
  }

  const payload = await response.json()
  const devices = Array.isArray(payload?.data)
    ? payload.data.map(mapDevice).filter(Boolean)
    : []

  return devices
}

const shouldUseApi = () => Boolean(config.retailPlayerDevicesEnabled)

const useRetailPlayerDevices = () => {
  const [devices, setDevices] = useState(() => RetailPlayerMockService.listDevices())
  const [error, setError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)

  const apiEnabled = useMemo(() => shouldUseApi(), [])

  useEffect(() => {
    if (!apiEnabled) {
      return undefined
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    fetchRetailPlayerDevices(abortController.signal)
      .then((apiDevices) => {
        if (apiDevices) {
          setDevices(apiDevices)
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
  }, [apiEnabled])

  return {
    devices,
    error,
    isApiEnabled: apiEnabled,
    isLoading,
  }
}

export default useRetailPlayerDevices
