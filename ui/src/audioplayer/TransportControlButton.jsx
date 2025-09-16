import React, { useCallback, useEffect, useRef } from 'react'
import PropTypes from 'prop-types'

const READY_EVENTS = ['canplay', 'canplaythrough', 'loadeddata']
const DEFAULT_DEBOUNCE_MS = 250

const TransportControlButton = ({
  audioInstance,
  direction,
  icon,
  label,
  onPointerInteraction,
  ensureQueueReady,
  getTelemetrySnapshot,
  debounceMs = DEFAULT_DEBOUNCE_MS,
}) => {
  const lastPointerTimeRef = useRef(0)
  const lastActivationRef = useRef(0)
  const cleanupRef = useRef(null)

  const clearListeners = useCallback(() => {
    if (cleanupRef.current) {
      cleanupRef.current()
      cleanupRef.current = null
    }
  }, [])

  useEffect(() => clearListeners, [audioInstance, clearListeners])

  const ensurePlayback = useCallback(() => {
    if (!audioInstance) {
      return Promise.resolve()
    }

    try {
      const playPromise = audioInstance.play()
      if (playPromise && typeof playPromise.then === 'function') {
        return playPromise.catch((error) => {
          if (
            error &&
            (error.name === 'NotAllowedError' || error.name === 'AbortError')
          ) {
            return
          }
          if (error) {
            // eslint-disable-next-line no-console
            console.warn(
              'Unable to resume playback after transport change',
              error,
            )
          }
        })
      }
    } catch (error) {
      if (!error || (error.name !== 'NotAllowedError' && error.name !== 'AbortError')) {
        // eslint-disable-next-line no-console
        console.warn('Unable to resume playback after transport change', error)
      }
    }

    return Promise.resolve()
  }, [audioInstance])

  const predictNextIndex = (snapshot, dir, queueLength) => {
    if (!snapshot || typeof snapshot.currentIndex !== 'number') {
      return null
    }
    const currentIndex = snapshot.currentIndex
    if (currentIndex < 0 || queueLength <= 0) {
      return null
    }
    const mode = snapshot.mode || 'orderLoop'
    if (dir === 'next') {
      switch (mode) {
        case 'singleLoop':
          return currentIndex
        case 'shufflePlay':
          return 'shuffle'
        case 'order':
          return currentIndex + 1 < queueLength ? currentIndex + 1 : null
        default:
          return currentIndex + 1 < queueLength ? currentIndex + 1 : 0
      }
    }
    if (dir === 'prev') {
      switch (mode) {
        case 'singleLoop':
          return currentIndex
        case 'shufflePlay':
          return 'shuffle'
        case 'order':
          return currentIndex > 0 ? currentIndex - 1 : null
        default:
          return currentIndex > 0 ? currentIndex - 1 : queueLength - 1
      }
    }
    return null
  }

  const advanceQueue = useCallback(async () => {
    if (!audioInstance) {
      return
    }

    if (typeof ensureQueueReady === 'function') {
      try {
        await ensureQueueReady(direction)
      } catch (error) {
        if (process.env.NODE_ENV !== 'production') {
          // eslint-disable-next-line no-console
          console.warn('Queue preparation failed before transport change', error)
        }
      }
    }

    const controlMethod =
      direction === 'next' ? audioInstance.playNext : audioInstance.playPrev

    if (typeof controlMethod !== 'function') {
      return
    }

    const telemetryBefore =
      typeof getTelemetrySnapshot === 'function'
        ? getTelemetrySnapshot()
        : null
    const wasPlaying = !audioInstance.paused
    const readyStateThreshold =
      typeof audioInstance.HAVE_ENOUGH_DATA === 'number'
        ? audioInstance.HAVE_ENOUGH_DATA
        : 4

    clearListeners()

    let fallbackTimeout = null
    let finalized = false
    let readyResolve = () => {}
    const readyPromise = wasPlaying
      ? new Promise((resolve) => {
          readyResolve = resolve
        })
      : Promise.resolve()

    const finalizePlayback = () => {
      if (finalized) {
        return
      }
      finalized = true
      if (fallbackTimeout) {
        clearTimeout(fallbackTimeout)
        fallbackTimeout = null
      }
      clearListeners()
      if (wasPlaying) {
        Promise.resolve(ensurePlayback()).finally(() => readyResolve())
      } else {
        readyResolve()
      }
    }

    if (wasPlaying) {
      READY_EVENTS.forEach((eventName) => {
        audioInstance.addEventListener(eventName, finalizePlayback, {
          once: true,
        })
      })
      fallbackTimeout = setTimeout(finalizePlayback, 500)
      cleanupRef.current = () => {
        if (fallbackTimeout) {
          clearTimeout(fallbackTimeout)
          fallbackTimeout = null
        }
        READY_EVENTS.forEach((eventName) => {
          audioInstance.removeEventListener(eventName, finalizePlayback)
        })
      }
    }

    try {
      controlMethod.call(audioInstance)
    } catch (error) {
      finalizePlayback()
      if (process.env.NODE_ENV !== 'production') {
        // eslint-disable-next-line no-console
        console.warn('Transport control invocation failed', error)
      }
      return
    }

    if (wasPlaying && audioInstance.readyState >= readyStateThreshold) {
      finalizePlayback()
    }

    if (!wasPlaying) {
      finalizePlayback()
    }

    await readyPromise

    const telemetryAfter =
      typeof getTelemetrySnapshot === 'function'
        ? getTelemetrySnapshot()
        : null
    const contextValue =
      telemetryAfter?.context || telemetryBefore?.context || 'unknown'
    const queueLength =
      telemetryAfter?.queueLength ??
      telemetryBefore?.queueLength ??
      0
    const beforeIndex = telemetryBefore?.currentIndex ?? -1
    const predictedIndex = predictNextIndex(
      telemetryBefore,
      direction,
      queueLength,
    )
    const afterIndex = telemetryAfter?.currentIndex ?? -1
    const beforePlaying = telemetryBefore?.isPlaying ?? wasPlaying
    const afterPlaying =
      telemetryAfter?.isPlaying ?? (audioInstance ? !audioInstance.paused : false)

    const transitionLabel =
      typeof predictedIndex === 'string'
        ? `${beforeIndex}→${predictedIndex}`
        : `${beforeIndex}→${
            predictedIndex === null ? 'none' : predictedIndex
          }`

    const logPayload = {
      context: contextValue,
      queueLength,
      transition: transitionLabel,
      resolvedIndex: afterIndex,
      isPlaying: { before: !!beforePlaying, after: !!afterPlaying },
    }
    // eslint-disable-next-line no-console
    console.info(`[transport] ${direction}`, logPayload)
  }, [
    audioInstance,
    clearListeners,
    direction,
    ensurePlayback,
    ensureQueueReady,
    getTelemetrySnapshot,
  ])

  const runActivation = useCallback(
    async (source) => {
      const now = Date.now()
      if (now - lastActivationRef.current < debounceMs) {
        return
      }
      lastActivationRef.current = now

      try {
        await advanceQueue()
      } catch (error) {
        if (process.env.NODE_ENV !== 'production') {
          // eslint-disable-next-line no-console
          console.warn('Transport activation failed', error)
        }
      } finally {
        if (source === 'pointer' && typeof onPointerInteraction === 'function') {
          onPointerInteraction()
        }
      }
    },
    [advanceQueue, debounceMs, onPointerInteraction],
  )

  const handlePointerUp = useCallback(
    (event) => {
      lastPointerTimeRef.current = Date.now()
      if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId)
      }
      event.preventDefault()
      event.stopPropagation()
      void runActivation('pointer')
    },
    [runActivation],
  )

  const handlePointerCancel = useCallback(() => {
    lastPointerTimeRef.current = 0
  }, [])

  const handleClick = useCallback(
    (event) => {
      const now = Date.now()
      if (now - lastPointerTimeRef.current < debounceMs) {
        event.preventDefault()
        event.stopPropagation()
        return
      }
      lastPointerTimeRef.current = 0
      event.preventDefault()
      event.stopPropagation()
      void runActivation('keyboard')
    },
    [debounceMs, runActivation],
  )

  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      data-transport-control="true"
      data-transport-direction={direction}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerCancel}
      onClick={handleClick}
      className="transport-control-button"
    >
      {icon}
    </button>
  )
}

TransportControlButton.propTypes = {
  audioInstance: PropTypes.object,
  direction: PropTypes.oneOf(['next', 'prev']).isRequired,
  icon: PropTypes.node.isRequired,
  label: PropTypes.string.isRequired,
  onPointerInteraction: PropTypes.func,
  ensureQueueReady: PropTypes.func,
  getTelemetrySnapshot: PropTypes.func,
  debounceMs: PropTypes.number,
}

export default TransportControlButton
