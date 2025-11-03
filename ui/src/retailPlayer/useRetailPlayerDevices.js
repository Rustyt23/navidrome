import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { buildDeviceSlug, deviceSlugKey, normalizeValue } from './deviceUtils'
import {
  DEFAULT_RETAIL_PLAYER_API_BASE_PATH,
  buildRetailPlayerApiPath,
  normalizeRetailPlayerBasePath,
} from './apiPaths'

const mapDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const rawId = normalizeValue(device.id)
  const fallbackId = rawId || normalizeValue(device.macAddress) || normalizeValue(device.ordinal)

  if (!fallbackId) {
    return null
  }

  const slug = buildDeviceSlug(device) || fallbackId
  const name = normalizeValue(device.name) || fallbackId

  return {
    id: fallbackId,
    apiId: rawId || null,
    name,
    slug,
    slugKey: deviceSlugKey(slug),
    channel: normalizeValue(device.channel),
    channelList: normalizeValue(device.channelList),
    organization:
      normalizeValue(device.organization) ||
      normalizeValue(device.orgUnit) ||
      normalizeValue(device.location),
    timeZone: normalizeValue(device.timeZone),
  }
}

const fetchRetailPlayerDevices = async (basePath, signal) => {
  const url = buildRetailPlayerApiPath(basePath, 'devices')

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
    ? payload.data.map(mapDevice).filter(Boolean)
    : []

  return { devices, enabled: true }
}

const useRetailPlayerDevices = (options = {}) => {
  const basePath = normalizeRetailPlayerBasePath(
    options.basePath || DEFAULT_RETAIL_PLAYER_API_BASE_PATH,
  )
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
    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    fetchRetailPlayerDevices(basePath, abortController.signal)
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
  }, [basePath])

  return {
    devices,
    error,
    isApiEnabled,
    isLoading,
  }
}

export default useRetailPlayerDevices
