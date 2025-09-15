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
      return
    }

    try {
      const playPromise = audioInstance.play()
      if (playPromise && typeof playPromise.catch === 'function') {
        playPromise.catch((error) => {
          if (
            error &&
            (error.name === 'NotAllowedError' || error.name === 'AbortError')
          ) {
            return
          }
          if (error) {
            // eslint-disable-next-line no-console
            console.warn('Unable to resume playback after transport change', error)
          }
        })
      }
    } catch (error) {
      if (!error || (error.name !== 'NotAllowedError' && error.name !== 'AbortError')) {
        // eslint-disable-next-line no-console
        console.warn('Unable to resume playback after transport change', error)
      }
    }
  }, [audioInstance])

  const advanceQueue = useCallback(() => {
    if (!audioInstance) {
      return
    }

    const controlMethod =
      direction === 'next' ? audioInstance.playNext : audioInstance.playPrev

    if (typeof controlMethod !== 'function') {
      return
    }

    const wasPlaying = !audioInstance.paused
    const readyStateThreshold =
      typeof audioInstance.HAVE_ENOUGH_DATA === 'number'
        ? audioInstance.HAVE_ENOUGH_DATA
        : 4

    clearListeners()

    let resumed = false
    const handleReady = () => {
      if (resumed) {
        return
      }
      resumed = true
      clearListeners()
      if (wasPlaying) {
        ensurePlayback()
      }
    }

    if (wasPlaying) {
      READY_EVENTS.forEach((eventName) => {
        audioInstance.addEventListener(eventName, handleReady, { once: true })
      })
      cleanupRef.current = () => {
        READY_EVENTS.forEach((eventName) => {
          audioInstance.removeEventListener(eventName, handleReady)
        })
      }
    }

    controlMethod.call(audioInstance)

    if (wasPlaying && audioInstance.readyState >= readyStateThreshold) {
      handleReady()
    }
  }, [audioInstance, clearListeners, direction, ensurePlayback])

  const runActivation = useCallback(
    (source) => {
      const now = Date.now()
      if (now - lastActivationRef.current < debounceMs) {
        return
      }
      lastActivationRef.current = now

      advanceQueue()

      if (source === 'pointer' && typeof onPointerInteraction === 'function') {
        onPointerInteraction()
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
      runActivation('pointer')
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
      runActivation('keyboard')
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
  debounceMs: PropTypes.number,
}

export default TransportControlButton
