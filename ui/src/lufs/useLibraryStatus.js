import { useCallback, useEffect, useRef, useState } from 'react'
import { useRefresh } from 'react-admin'
import { httpClient } from '../dataProvider'

export const LIBRARY_URL = '/api/song/loudness/library'

// useLibraryStatus polls the whole-library optimisation run while it is going,
// and refreshes the list whenever a completed track has persisted its audit.
export const useLibraryStatus = () => {
  const refresh = useRefresh()
  const [status, setStatus] = useState(null)
  const previous = useRef({ processed: null, running: false })

  const poll = useCallback(() => {
    return httpClient(LIBRARY_URL)
      .then(({ json }) => {
        const processed = json?.processed || 0
        if (
          (previous.current.processed !== null &&
            processed > previous.current.processed) ||
          (previous.current.running && !json?.running)
        ) {
          refresh()
        }
        previous.current = { processed, running: !!json?.running }
        setStatus(json)
        return json
      })
      .catch(() => null)
  }, [refresh])

  useEffect(() => {
    poll()
  }, [poll])

  useEffect(() => {
    if (!status?.running) return undefined
    const timer = setInterval(poll, 2000)
    return () => clearInterval(timer)
  }, [status?.running, poll])

  return { status, poll }
}
