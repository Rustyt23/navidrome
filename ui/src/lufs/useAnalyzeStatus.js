import { useCallback, useEffect, useRef, useState } from 'react'
import { useRefresh } from 'react-admin'
import { httpClient } from '../dataProvider'

export const ANALYZE_URL = '/api/song/loudness/analyze'

// useAnalyzeStatus polls the analysis job while it is running and refreshes the
// list whenever completed-track progress advances.
export const useAnalyzeStatus = () => {
  const refresh = useRefresh()
  const [status, setStatus] = useState(null)
  const previous = useRef({ processed: null, running: false })

  const poll = useCallback(() => {
    return httpClient(ANALYZE_URL)
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
