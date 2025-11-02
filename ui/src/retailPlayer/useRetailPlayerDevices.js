import { useEffect, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import { buildDeviceSlug, deviceSlugKey, normalizeValue } from './deviceUtils'

const buildDevicesUrl = () => '/api/retailplayer/devices'

const mapDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const id = normalizeValue(device.id)
  const name = normalizeValue(device.name) || id
  if (!id && !name) {
    return null
  }

  const slug = buildDeviceSlug(device) || name || id

  return {
    id: id || name,
    apiId: normalizeValue(device.apiId) || id || null,
    name: name || id,
    slug,
    slugKey: deviceSlugKey(slug),
    channel: normalizeValue(device.channel),
    channelList: normalizeValue(device.channelList),
    organization: normalizeValue(device.organization),
    timeZone: normalizeValue(device.timeZone),
  }
}

const fetchRetailPlayerDevices = async (signal) => {
  const url = buildDevicesUrl()
  if (!url) {
    return { devices: [], enabled: false }
  }

  const { json } = await httpClient(url, { signal })
  const devices = Array.isArray(json?.data)
    ? json.data.map(mapDevice).filter(Boolean)
    : []

  return { devices, enabled: true }
}

const useRetailPlayerDevices = () => {
  const [devices, setDevices] = useState([])
  const [error, setError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isApiEnabled, setIsApiEnabled] = useState(
    Boolean(config.retailPlayerDevicesEnabled),
  )

  useEffect(() => {
    const url = buildDevicesUrl()
    if (!url) {
      setIsApiEnabled(false)
      setDevices([])
      return () => {}
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    fetchRetailPlayerDevices(abortController.signal)
      .then((result) => {
        setIsApiEnabled(Boolean(result?.enabled))
        setDevices(Array.isArray(result?.devices) ? result.devices : [])
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }
        if (err?.status === 404) {
          setIsApiEnabled(false)
          setDevices([])
          setError(null)
          return
        }
        setError(err)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setIsLoading(false)
        }
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
