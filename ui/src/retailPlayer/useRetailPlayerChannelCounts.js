import { useEffect, useMemo, useState } from 'react'

const useRetailPlayerChannelCounts = (devices, isApiEnabled) => {
  const safeDevices = useMemo(
    () =>
      Array.isArray(devices)
        ? devices.filter((device) => device && typeof device === 'object')
        : [],
    [devices],
  )
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState(null)

  useEffect(() => {
    // Intentionally disable channel count polling to avoid per-channel-list API calls.
    setIsLoading(false)
    setError(null)

    return undefined
  }, [safeDevices, isApiEnabled])

  const countsByDeviceId = useMemo(() => {
    const mapping = {}
    safeDevices.forEach((device) => {
      const deviceId = device.id
      if (!deviceId) {
        return
      }
      const rawCount = device.channelCatalogCount
      mapping[deviceId] = Number.isFinite(rawCount) ? rawCount : null
    })
    return mapping
  }, [safeDevices])

  return { countsByDeviceId, isLoading, error }
}

export default useRetailPlayerChannelCounts
