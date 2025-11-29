import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { mapRetailPlayerDevice } from './deviceUtils'

const buildDevicesUrl = () => '/api/retailplayer/devices'

const fetchRetailPlayerDevices = async (signal) => {
  const url = buildDevicesUrl()

  if (!url) {
    return { devices: null, folders: null, deviceFolders: null, enabled: false }
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
  const folders = Array.isArray(payload?.folders) ? payload.folders : []
  const deviceFolders = Array.isArray(payload?.deviceFolders)
    ? payload.deviceFolders
    : []

  return { devices, folders, deviceFolders, enabled: true }
}

const useRetailPlayerDevices = (options = {}) => {
  const { fetchDevices = Boolean(config.retailPlayerDevicesEnabled) } = options

  const [devices, setDevices] = useState(() => {
    if (config.retailPlayerDevicesEnabled && fetchDevices) {
      return []
    }
    return RetailPlayerMockService.listDevices()
  })
  const [folders, setFolders] = useState([])
  const [deviceFolders, setDeviceFolders] = useState([])
  const [error, setError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isApiEnabled, setIsApiEnabled] = useState(
    Boolean(config.retailPlayerDevicesEnabled),
  )

  useEffect(() => {
    if (!fetchDevices) {
      setIsApiEnabled(Boolean(config.retailPlayerDevicesEnabled))
      setIsLoading(false)
      return undefined
    }

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
        if (Array.isArray(result?.folders)) {
          setFolders(result.folders)
        } else if (!enabled) {
          setFolders([])
        }
        if (Array.isArray(result?.deviceFolders)) {
          setDeviceFolders(result.deviceFolders)
        } else if (!enabled) {
          setDeviceFolders([])
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
  }, [fetchDevices])

  return {
    devices,
    folders,
    deviceFolders,
    error,
    isApiEnabled,
    isLoading,
  }
}

export default useRetailPlayerDevices
