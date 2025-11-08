import { useEffect, useMemo, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { deviceSlugKey } from './deviceUtils'
import { mapDevice } from './useRetailPlayerDevices'

const buildLookupUrl = (slug) =>
  slug ? `/api/retailplayer/devices/lookup/${encodeURIComponent(slug)}` : null

const findMockDeviceBySlugKey = (slugKey) => {
  if (!slugKey) {
    return null
  }

  const devices = RetailPlayerMockService.listDevices()
  return (
    devices.find((device) => device.slugKey === slugKey) ||
    devices.find((device) => deviceSlugKey(device.name) === slugKey) ||
    devices.find((device) => deviceSlugKey(device.id) === slugKey) ||
    null
  )
}

const normalizeSlugParam = (slugParam) => {
  if (!slugParam) {
    return ''
  }
  try {
    return decodeURIComponent(slugParam)
  } catch (err) {
    return slugParam
  }
}

const useRetailPlayerDeviceLookup = (slugParam) => {
  const [device, setDevice] = useState(null)
  const [error, setError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isApiEnabled, setIsApiEnabled] = useState(
    Boolean(config.retailPlayerDevicesEnabled),
  )

  const normalizedSlug = useMemo(
    () => normalizeSlugParam(slugParam),
    [slugParam],
  )
  const normalizedSlugKey = useMemo(
    () => deviceSlugKey(normalizedSlug),
    [normalizedSlug],
  )

  useEffect(() => {
    if (!normalizedSlugKey) {
      setDevice(null)
      setError(null)
      setIsLoading(false)
      setIsApiEnabled(Boolean(config.retailPlayerDevicesEnabled))
      return undefined
    }

    if (!config.retailPlayerDevicesEnabled) {
      const mockDevice = findMockDeviceBySlugKey(normalizedSlugKey)
      setDevice(mockDevice)
      setError(null)
      setIsApiEnabled(false)
      setIsLoading(false)
      return undefined
    }

    const url = buildLookupUrl(slugParam)
    if (!url) {
      setDevice(null)
      setError(null)
      setIsLoading(false)
      setIsApiEnabled(true)
      return undefined
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)
    setIsApiEnabled(true)

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }
        const mappedDevice = mapDevice(json?.data)
        setDevice(mappedDevice)
        setError(null)
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }
        setDevice(null)
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
  }, [normalizedSlugKey, slugParam])

  return { device, error, isApiEnabled, isLoading }
}

export default useRetailPlayerDeviceLookup
