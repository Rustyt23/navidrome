import { useEffect, useState } from 'react'
import config from '../config'
import RetailPlayerMockService from './RetailPlayerMockService'
import {
  getRetailPlayerDevicesSync,
  resolveRetailPlayerDevices,
} from './deviceApi'

const useRetailPlayerDevices = () => {
  const [devices, setDevices] = useState(getRetailPlayerDevicesSync)
  const [error, setError] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isApiEnabled, setIsApiEnabled] = useState(
    Boolean(config.retailPlayerDevicesEnabled),
  )

  useEffect(() => {
    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    resolveRetailPlayerDevices({ signal: abortController.signal })
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
