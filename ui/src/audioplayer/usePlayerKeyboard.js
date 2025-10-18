import { useEffect } from 'react'

const usePlayerKeyboard = (handlers) => {
  const prevHandler = handlers?.PREV_SONG
  const nextHandler = handlers?.NEXT_SONG

  useEffect(() => {
    if (!prevHandler && !nextHandler) {
      return undefined
    }

    const handleKeyDown = (event) => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
        return
      }

      event.preventDefault()
      event.stopPropagation()

      if (event.key === 'ArrowLeft') {
        prevHandler?.(event)
      } else if (event.key === 'ArrowRight') {
        nextHandler?.(event)
      }
    }

    window.addEventListener('keydown', handleKeyDown, true)

    return () => {
      window.removeEventListener('keydown', handleKeyDown, true)
    }
  }, [prevHandler, nextHandler])
}

export default usePlayerKeyboard
