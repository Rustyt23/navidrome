import { useCallback, useEffect, useMemo, useRef } from 'react'
import keyHandlers from './keyHandlers'

const usePlayerKeyboard = (audioInstance, playerState) => {
  const handlers = useMemo(
    () => keyHandlers(audioInstance, playerState),
    [audioInstance, playerState],
  )

  const hasPrevTrack = useCallback(() => {
    const queue = playerState?.queue || []
    if (!queue.length) {
      return false
    }

    const currentUuid = playerState?.current?.uuid
    if (!currentUuid) {
      return false
    }

    const currentIndex = queue.findIndex((item) => item.uuid === currentUuid)
    return currentIndex > 0
  }, [playerState])

  const runPrevSong = useCallback(
    (event) => {
      const handler = handlers?.PREV_SONG
      if (typeof handler !== 'function') {
        return
      }

      if (
        !event?.metaKey &&
        hasPrevTrack() &&
        audioInstance &&
        typeof audioInstance.currentTime === 'number'
      ) {
        try {
          audioInstance.currentTime = 0
        } catch (err) {
          // Ignore failures when resetting the current time (e.g. if metadata
          // is not yet available)
        }
      }

      handler(event)
    },
    [audioInstance, handlers, hasPrevTrack],
  )

  const runNextSong = useCallback(
    (event) => {
      const handler = handlers?.NEXT_SONG
      if (typeof handler === 'function') {
        handler(event)
      }
    },
    [handlers],
  )

  const prevHandlerRef = useRef(runPrevSong)
  const nextHandlerRef = useRef(runNextSong)

  useEffect(() => {
    prevHandlerRef.current = runPrevSong
  }, [runPrevSong])

  useEffect(() => {
    nextHandlerRef.current = runNextSong
  }, [runNextSong])

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }

    const handleKeyDown = (event) => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
        return
      }

      event.preventDefault()
      event.stopPropagation()

      const ref = event.key === 'ArrowLeft' ? prevHandlerRef : nextHandlerRef
      const callback = ref.current

      if (typeof callback === 'function') {
        callback(event)
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)

    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
    }
  }, [])

  return handlers
}

export default usePlayerKeyboard
