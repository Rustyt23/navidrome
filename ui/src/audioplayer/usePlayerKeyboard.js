import { useEffect, useMemo } from 'react'
import keyHandlers from './keyHandlers'

const usePlayerKeyboard = (audioInstance, playerState) => {
  const handlers = useMemo(
    () => keyHandlers(audioInstance, playerState),
    [audioInstance, playerState],
  )

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

      const handlerKey = event.key === 'ArrowLeft' ? 'PREV_SONG' : 'NEXT_SONG'
      const handler = handlers?.[handlerKey]

      if (typeof handler === 'function') {
        handler(event)
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)

    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
    }
  }, [handlers])

  return handlers
}

export default usePlayerKeyboard
