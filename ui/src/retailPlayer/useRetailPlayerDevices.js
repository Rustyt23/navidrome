import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { mapRetailPlayerDevice } from './deviceUtils'
import { mapRetailPlayerFolder } from './folderUtils'

const buildDevicesUrl = (deviceName) =>
  deviceName ? '/api/retailplayer/rc' : '/api/retailplayer/devices'

const fetchRetailPlayerDevices = async (signal, deviceName) => {
  const url = buildDevicesUrl(deviceName)

  if (!url) {
    return { devices: null, folders: null, deviceFolders: null, enabled: false }
  }

  let payload
  try {
    const options = { signal }
    if (deviceName) {
      const headers = new Headers({ Accept: 'application/json' })
      headers.set('Content-Type', 'application/json')
      options.headers = headers
      options.method = 'POST'
      options.body = JSON.stringify({ name: deviceName })
    }

    const { json } = await httpClient(url, options)
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
  const folders = Array.isArray(payload?.folders)
    ? payload.folders.map(mapRetailPlayerFolder).filter(Boolean)
    : []
  const deviceFolders = Array.isArray(payload?.deviceFolders)
    ? payload.deviceFolders
    : []

  return { devices, folders, deviceFolders, enabled: true }
}

const useRetailPlayerDevices = ({ deviceName = '' } = {}) => {
  const [devices, setDevices] = useState(() => {
    if (config.retailPlayerDevicesEnabled) {
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
    const url = buildDevicesUrl(deviceName)
    if (!url) {
      setIsApiEnabled(false)
      return undefined
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    fetchRetailPlayerDevices(abortController.signal, deviceName)
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
  }, [deviceName])

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
