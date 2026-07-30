import { useCallback, useEffect, useRef, useState } from 'react'
import { useRefresh } from 'react-admin'
import { httpClient } from '../dataProvider'

// How hard to poll just after a job is kicked off, and for how long. A job over
// a handful of tracks can start and finish inside a single ordinary interval,
// so on that schedule it would appear to have done nothing at all.
const WATCH_INTERVAL_MS = 400
const WATCH_ATTEMPTS = 25
const IDLE_INTERVAL_MS = 2000

// useJobStatus follows one of the long-running loudness jobs: it polls while
// the job is going, and refreshes the list as completed tracks persist their
// audit records.
//
// Both jobs behave the same way and differ only in their URL, so they share
// this rather than each growing their own copy of the polling - and their own
// copy of every bug in it.
export const useJobStatus = (url) => {
  const refresh = useRefresh()
  const [status, setStatus] = useState(null)
  const previous = useRef({ processed: null, running: false })

  const poll = useCallback(() => {
    return httpClient(url)
      .then(({ json }) => {
        const processed = json?.processed || 0
        const stopped = previous.current.running && !json?.running
        if (
          (previous.current.processed !== null &&
            processed > previous.current.processed) ||
          stopped
        ) {
          refresh()
        }
        previous.current = { processed, running: !!json?.running }
        setStatus(json)
        return json
      })
      .catch(() => null)
  }, [refresh, url])

  // watch is for the moment a job is started from the UI.
  //
  // The request only returns once the job is marked as going, so the first
  // status that comes back NOT going means it has already finished - which for
  // a job over one or two tracks it may well have. onSettled is always called
  // exactly once, whether the job was caught in flight or was over before the
  // first look; a caller that clears its own "starting" state on a transition
  // it never sees would otherwise wait for ever.
  const watch = useCallback(
    (onSettled) => {
      let attempts = 0
      const tick = () => {
        poll().then((json) => {
          attempts += 1
          if ((json && !json.running) || attempts >= WATCH_ATTEMPTS) {
            onSettled?.(json)
            return
          }
          setTimeout(tick, WATCH_INTERVAL_MS)
        })
      }
      setTimeout(tick, WATCH_INTERVAL_MS)
    },
    [poll],
  )

  useEffect(() => {
    poll()
  }, [poll])

  useEffect(() => {
    if (!status?.running) return undefined
    const timer = setInterval(poll, IDLE_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [status?.running, poll])

  return { status, poll, watch }
}

export default useJobStatus
