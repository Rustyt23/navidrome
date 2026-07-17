import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { mapRetailPlayerDevice } from './deviceUtils'

const RETAIL_PLAYER_FOLDER_QUERY_PARAM = 'folder'

// Anonymous (public) sessions have no auth token; they can only access the
// folder-scoped public endpoint, keyed by folder name (or id) from the URL.
const isPublicSession = () => {
  if (typeof window === 'undefined') {
    return false
  }
  return !localStorage.getItem('token')
}

const getPublicFolderFromLocation = () => {
  if (typeof window === 'undefined' || !isPublicSession()) {
    return null
  }
  const { hash } = window.location
  const queryIndex = hash ? hash.indexOf('?') : -1
  if (queryIndex === -1) {
    return null
  }
  const params = new URLSearchParams(hash.slice(queryIndex + 1))
  const folder = params.get(RETAIL_PLAYER_FOLDER_QUERY_PARAM)
  return folder && folder.trim() !== '' ? folder.trim() : null
}

const buildDevicesUrl = (deviceName, publicFolder) => {
  if (deviceName) {
    return '/api/retailplayer/rc'
  }
  if (publicFolder) {
    return `/api/retailplayer/folders/${encodeURIComponent(publicFolder)}/devices`
  }
  if (isPublicSession()) {
    // Anonymous visitors without a folder in the URL have nothing to fetch:
    // the full devices endpoint requires authentication.
    return null
  }
  return '/api/retailplayer/devices'
}

const fetchRetailPlayerDevices = async (signal, deviceName, publicFolder) => {
  const url = buildDevicesUrl(deviceName, publicFolder)

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
      if (publicFolder) {
        throw new Error(`Retail player folder "${publicFolder}" not found`)
      }
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
  const [publicFolder, setPublicFolder] = useState(() =>
    getPublicFolderFromLocation(),
  )

  useEffect(() => {
    if (typeof window === 'undefined') {
      return undefined
    }
    const updatePublicFolder = () => {
      setPublicFolder(getPublicFolderFromLocation())
    }
    window.addEventListener('hashchange', updatePublicFolder)
    return () => {
      window.removeEventListener('hashchange', updatePublicFolder)
    }
  }, [])

  useEffect(() => {
    const url = buildDevicesUrl(deviceName, publicFolder)
    if (!url) {
      setIsApiEnabled(false)
      return undefined
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    fetchRetailPlayerDevices(abortController.signal, deviceName, publicFolder)
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
  }, [deviceName, publicFolder])

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
