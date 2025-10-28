import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'

const buildDevicesUrl = () => '/api/retailplayer/devices'

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

const useRetailPlayerDevices = () => {
  const [devices, setDevices] = useState(() => RetailPlayerMockService.listDevices())
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
        setIsApiEnabled(Boolean(result?.enabled))
        if (Array.isArray(result?.devices)) {
          setDevices(result.devices)
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
