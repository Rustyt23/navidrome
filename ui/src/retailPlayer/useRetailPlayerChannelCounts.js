import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import httpClient from '../dataProvider/httpClient'
import { normalizeValue } from './deviceUtils'

const hasOwn = (object, key) =>
  Object.prototype.hasOwnProperty.call(object, key)

const mapChannelListResponseToCount = (payload) => {
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const channels = Array.isArray(payload.channels)
    ? payload.channels.filter((channel) => {
        if (!channel || typeof channel !== 'object') {
          return false
        }
        const id = normalizeValue(channel.id)
        const name = normalizeValue(channel.name)
        return Boolean(id || name)
      })
    : []

  return channels.length
}

const useRetailPlayerChannelCounts = (devices, isApiEnabled) => {
  const safeDevices = useMemo(
    () =>
      Array.isArray(devices)
        ? devices.filter((device) => device && typeof device === 'object')
        : [],
    [devices],
  )
  const [channelListCounts, setChannelListCounts] = useState({})
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState(null)
  const pendingRef = useRef(new Set())
  const statusQueueRef = useRef(new Map())
  const statusPendingRef = useRef(new Set())
  const statusFetchedRef = useRef(new Set())
  const [statusRequestVersion, setStatusRequestVersion] = useState(0)
  const [channelListOverrides, setChannelListOverrides] = useState({})

  useEffect(() => {
    if (!isApiEnabled) {
      setChannelListCounts({})
      setIsLoading(false)
      setError(null)
      pendingRef.current.clear()
      setChannelListOverrides({})
      statusQueueRef.current.clear()
      statusPendingRef.current.clear()
      statusFetchedRef.current.clear()
      setStatusRequestVersion(0)
    }
  }, [isApiEnabled])

  const enqueueStatusFetch = useCallback(
    (deviceId, identifier) => {
      if (!deviceId || !identifier || !isApiEnabled) {
        return
      }
      if (
        statusFetchedRef.current.has(deviceId) ||
        statusPendingRef.current.has(deviceId)
      ) {
        return
      }
      const existing = statusQueueRef.current.get(deviceId)
      if (existing === identifier) {
        return
      }
      statusQueueRef.current.set(deviceId, identifier)
      setStatusRequestVersion((previous) => previous + 1)
    },
    [isApiEnabled],
  )

  const deviceDescriptors = useMemo(() => {
    if (!safeDevices.length) {
      return []
    }

    return safeDevices
      .map((device) => {
        if (!device || typeof device !== 'object') {
          return null
        }

        const deviceId = normalizeValue(device.id)
        if (!deviceId) {
          return null
        }

        const identifier = normalizeValue(device.apiId || device.id)
        const override = normalizeValue(channelListOverrides[deviceId])
        const channelListId = override || normalizeValue(device.channelList)

        return {
          deviceId,
          identifier,
          channelListId,
        }
      })
      .filter(Boolean)
  }, [safeDevices, channelListOverrides])

  const channelListToDeviceIds = useMemo(() => {
    const mapping = new Map()
    deviceDescriptors.forEach(({ deviceId, channelListId }) => {
      if (!channelListId) {
        return
      }
      if (!mapping.has(channelListId)) {
        mapping.set(channelListId, new Set())
      }
      mapping.get(channelListId).add(deviceId)
    })
    return mapping
  }, [deviceDescriptors])

  const deviceIdentifierMap = useMemo(() => {
    const mapping = new Map()
    deviceDescriptors.forEach(({ deviceId, identifier }) => {
      if (!deviceId || !identifier) {
        return
      }
      mapping.set(deviceId, identifier)
    })
    return mapping
  }, [deviceDescriptors])

  useEffect(() => {
    if (!isApiEnabled) {
      return
    }

    deviceDescriptors.forEach(({ deviceId, identifier, channelListId }) => {
      if (!channelListId && identifier) {
        enqueueStatusFetch(deviceId, identifier)
      }
    })
  }, [deviceDescriptors, enqueueStatusFetch, isApiEnabled])

  useEffect(() => {
    if (!isApiEnabled) {
      return
    }

    if (!statusQueueRef.current.size) {
      return
    }

    const entries = Array.from(statusQueueRef.current.entries())
    statusQueueRef.current.clear()
    entries.forEach(([deviceId]) => statusPendingRef.current.add(deviceId))

    const abortController = new AbortController()

    Promise.all(
      entries.map(([deviceId, identifier]) =>
        httpClient(
          `/api/retailplayer/devices/${encodeURIComponent(identifier)}/status`,
          { signal: abortController.signal },
        )
          .then(({ json }) => {
            if (abortController.signal.aborted) {
              return { deviceId, aborted: true }
            }
            const channelListId = normalizeValue(json?.device?.channelList)
            return { deviceId, channelListId }
          })
          .catch((err) => {
            if (abortController.signal.aborted) {
              return { deviceId, aborted: true }
            }
            return { deviceId, error: err }
          }),
      ),
    )
      .then((results) => {
        if (abortController.signal.aborted) {
          return
        }

        let didUpdate = false
        const nextOverrides = { ...channelListOverrides }

        results.forEach(({ deviceId, channelListId, aborted }) => {
          if (aborted) {
            return
          }
          statusFetchedRef.current.add(deviceId)
          if (channelListId && nextOverrides[deviceId] !== channelListId) {
            nextOverrides[deviceId] = channelListId
            didUpdate = true
          }
        })

        if (didUpdate) {
          setChannelListOverrides(nextOverrides)
        }
      })
      .finally(() => {
        entries.forEach(([deviceId]) => {
          statusPendingRef.current.delete(deviceId)
        })
      })

    return () => {
      abortController.abort()
      entries.forEach(([deviceId, identifier]) => {
        statusPendingRef.current.delete(deviceId)
        if (!statusFetchedRef.current.has(deviceId)) {
          statusQueueRef.current.set(deviceId, identifier)
        }
      })
    }
  }, [statusRequestVersion, isApiEnabled, channelListOverrides])

  useEffect(() => {
    if (!isApiEnabled) {
      setChannelListCounts({})
      setIsLoading(false)
      setError(null)
      pendingRef.current.clear()
      return
    }

    const pending = pendingRef.current

    const uniqueChannelListIds = Array.from(
      new Set(
        deviceDescriptors
          .map((descriptor) => descriptor.channelListId)
          .filter(Boolean),
      ),
    )

    const missingChannelLists = uniqueChannelListIds.filter(
      (channelListId) =>
        !hasOwn(channelListCounts, channelListId) && !pending.has(channelListId),
    )

    if (!missingChannelLists.length) {
      if (!uniqueChannelListIds.length) {
        setIsLoading(false)
        setError(null)
      }
      return
    }

    missingChannelLists.forEach((id) => pending.add(id))

    const abortController = new AbortController()
    setIsLoading(true)
    setError(null)

    Promise.all(
      missingChannelLists.map((channelListId) =>
        httpClient(
          `/api/retailplayer/channel-lists/${encodeURIComponent(
            channelListId,
          )}/channels`,
          { signal: abortController.signal },
        )
          .then(({ json }) => {
            if (abortController.signal.aborted) {
              return { channelListId, aborted: true }
            }
            const count = mapChannelListResponseToCount(json)
            return { channelListId, count }
          })
          .catch((err) => {
            if (abortController.signal.aborted) {
              return { channelListId, aborted: true }
            }
            return { channelListId, count: null, error: err }
          }),
      ),
    )
      .then((results) => {
        if (abortController.signal.aborted) {
          return
        }
        setChannelListCounts((previous) => {
          const next = { ...previous }
          let didChange = false
          results.forEach(({ channelListId, count, aborted }) => {
            if (aborted) {
              return
            }
            const normalizedCount =
              typeof count === 'number' && Number.isFinite(count) ? count : null
            if (next[channelListId] !== normalizedCount) {
              next[channelListId] = normalizedCount
              didChange = true
            }
          })
          return didChange ? next : previous
        })

        const failedResults = results.filter(
          (result) => result.error && !result.aborted,
        )

        if (failedResults.length) {
          failedResults.forEach(({ channelListId }) => {
            const deviceIds = channelListToDeviceIds.get(channelListId)
            if (!deviceIds) {
              return
            }
            deviceIds.forEach((deviceId) => {
              const identifier = deviceIdentifierMap.get(deviceId)
              enqueueStatusFetch(deviceId, identifier)
            })
          })
        }

        const failure = results.find(
          (result) => result.error && !result.aborted,
        )
        setError(failure ? failure.error : null)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setIsLoading(false)
        }
        missingChannelLists.forEach((id) => pending.delete(id))
      })

    return () => {
      abortController.abort()
      missingChannelLists.forEach((id) => pending.delete(id))
    }
  }, [
    deviceDescriptors,
    isApiEnabled,
    channelListCounts,
    channelListToDeviceIds,
    deviceIdentifierMap,
    enqueueStatusFetch,
  ])

  const countsByDeviceId = useMemo(() => {
    const mapping = {}
    deviceDescriptors.forEach(({ deviceId, channelListId }) => {
      if (!deviceId) {
        return
      }
      if (channelListId && hasOwn(channelListCounts, channelListId)) {
        mapping[deviceId] = channelListCounts[channelListId]
      } else {
        mapping[deviceId] = null
      }
    })
    return mapping
  }, [deviceDescriptors, channelListCounts])

  return { countsByDeviceId, isLoading, error }
}

export default useRetailPlayerChannelCounts
