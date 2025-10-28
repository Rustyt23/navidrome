import { useEffect, useMemo, useState } from 'react'
import config from '../config'
import RetailPlayerMockService from './RetailPlayerMockService'

const DEFAULT_KEY_HEADER = 'X-API-Key'

const buildFieldsParams = (fields, searchParams) => {
  if (!fields) {
    return
  }

  if (Array.isArray(fields)) {
    fields.filter(Boolean).forEach((field) => searchParams.append('fields', field))
    return
  }

  if (typeof fields === 'string') {
    fields
      .split(',')
      .map((field) => field.trim())
      .filter(Boolean)
      .forEach((field) => searchParams.append('fields', field))
  }
}

const buildDevicesUrl = () => {
  if (!config.retailPlayerApiBaseUrl || !config.retailPlayerApiOrgId) {
    return null
  }

  const url = new URL(
    `/orgs/${config.retailPlayerApiOrgId}/devices`,
    config.retailPlayerApiBaseUrl,
  )

  const { searchParams } = url

  if (config.retailPlayerApiPageSize) {
    searchParams.set('pageSize', String(config.retailPlayerApiPageSize))
  }

  if (config.retailPlayerApiPage) {
    searchParams.set('page', String(config.retailPlayerApiPage))
  }

  if (config.retailPlayerApiFilters) {
    searchParams.set('filters', String(config.retailPlayerApiFilters))
  }

  if (config.retailPlayerApiOrderBy) {
    searchParams.set('orderBy', String(config.retailPlayerApiOrderBy))
  }

  if (config.retailPlayerApiOrderDirection) {
    searchParams.set('orderDirection', String(config.retailPlayerApiOrderDirection))
  }

  if (config.retailPlayerApiSearch) {
    searchParams.set('search', String(config.retailPlayerApiSearch))
  }

  buildFieldsParams(config.retailPlayerApiFields, searchParams)

  return url
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

  const headers = new Headers({ Accept: 'application/json' })

  if (config.retailPlayerApiKey) {
    headers.set(
      config.retailPlayerApiKeyHeader || DEFAULT_KEY_HEADER,
      config.retailPlayerApiKey,
    )
  }

  if (
    config.retailPlayerApiAdditionalHeaders &&
    typeof config.retailPlayerApiAdditionalHeaders === 'object'
  ) {
    Object.entries(config.retailPlayerApiAdditionalHeaders).forEach(([key, value]) => {
      if (key && value !== undefined && value !== null) {
        headers.set(key, String(value))
      }
    })
  }

  const response = await fetch(url.toString(), {
    method: 'GET',
    headers,
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

const shouldUseApi = () =>
  Boolean(config.retailPlayerApiBaseUrl && config.retailPlayerApiOrgId)

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
