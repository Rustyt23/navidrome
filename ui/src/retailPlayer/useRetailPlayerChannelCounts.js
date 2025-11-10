import { useEffect, useMemo, useRef, useState } from 'react'
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
        safeDevices
          .map((device) => normalizeValue(device.channelList))
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
          results.forEach(({ channelListId, count, aborted }) => {
            if (aborted) {
              return
            }
            next[channelListId] =
              typeof count === 'number' && Number.isFinite(count) ? count : null
          })
          return next
        })
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
  }, [safeDevices, isApiEnabled, channelListCounts])

  const countsByDeviceId = useMemo(() => {
    const mapping = {}
    safeDevices.forEach((device) => {
      const deviceId = device.id
      if (!deviceId) {
        return
      }
      const channelListId = normalizeValue(device.channelList)
      if (channelListId && hasOwn(channelListCounts, channelListId)) {
        mapping[deviceId] = channelListCounts[channelListId]
      } else {
        mapping[deviceId] = null
      }
    })
    return mapping
  }, [safeDevices, channelListCounts])

  return { countsByDeviceId, isLoading, error }
}

export default useRetailPlayerChannelCounts
