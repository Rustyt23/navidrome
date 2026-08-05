import { useCallback, useEffect, useRef, useState } from 'react'
import { useRefresh } from 'react-admin'
import { httpClient } from '../dataProvider'

// Absolute, and including the /api prefix. httpClient only applies baseUrl(),
// which does not add it - so a bare "song/silence/analyze" is resolved relative
// to the current page and 404s under the hash router.
export const ANALYZE_URL = '/api/song/silence/analyze'
export const TRIM_URL = '/api/song/silence/trim'
export const SUMMARY_URL = '/api/song/silence/summary'
export const CLEAR_URL = '/api/song/silence/analyze/results'

// How hard to poll just after a job is kicked off, and for how long. A job over
// a handful of songs can start and finish inside a single idle interval, so on
// that schedule it would appear to have done nothing at all.
const WATCH_INTERVAL_MS = 400
const WATCH_ATTEMPTS = 25
const IDLE_INTERVAL_MS = 2000

// One shared record per job URL, so every component watching a job reads the
// same state. Without this, the button that starts a run and the toolbar that
// draws its progress each poll separately and can disagree about whether
// anything is happening.
const stores = new Map()

const getStore = (url) => {
  let store = stores.get(url)
  if (!store) {
    store = { url, status: null, listeners: new Set(), timer: null, inFlight: null }
    stores.set(url, store)
  }
  return store
}

// Only poll on a timer while a job is actually going, and only while somebody
// is watching.
const syncTimer = (store) => {
  const wanted = !!store.status?.running && store.listeners.size > 0
  if (wanted && !store.timer) {
    store.timer = setInterval(() => pollStore(store), IDLE_INTERVAL_MS)
  } else if (!wanted && store.timer) {
    clearInterval(store.timer)
    store.timer = null
  }
}

// Concurrent callers share one request: the shared timer and any number of
// watch ticks can land together, and they all want the same answer.
const pollStore = (store) => {
  if (store.inFlight) return store.inFlight
  store.inFlight = httpClient(store.url)
    .then(({ json }) => {
      store.inFlight = null
      store.status = json
      store.listeners.forEach((listener) => listener(json))
      syncTimer(store)
      return json
    })
    .catch(() => {
      store.inFlight = null
      return null
    })
  return store.inFlight
}

// useSilenceStatus follows one of the silence jobs: it polls while the job runs
// and refreshes the list as songs finish, so rows update as the work lands.
export const useSilenceStatus = (url) => {
  const refresh = useRefresh()
  const [status, setStatus] = useState(() => getStore(url).status)
  const previous = useRef({ processed: null, running: false })
  // Held in a ref so a new identity from react-admin cannot tear down and
  // rebuild the subscription underneath a running job.
  const refreshRef = useRef(refresh)
  refreshRef.current = refresh

  useEffect(() => {
    const store = getStore(url)
    const listener = (json) => {
      const processed = json?.processed || 0
      const stopped = previous.current.running && !json?.running
      if (
        (previous.current.processed !== null &&
          processed > previous.current.processed) ||
        stopped
      ) {
        refreshRef.current()
      }
      previous.current = { processed, running: !!json?.running }
      setStatus(json)
    }
    store.listeners.add(listener)
    pollStore(store)
    return () => {
      store.listeners.delete(listener)
      syncTimer(store)
    }
  }, [url])

  const poll = useCallback(() => pollStore(getStore(url)), [url])

  // watch is for the moment a job is started from the UI. The start request
  // marks the job running before it replies, so the first status that comes
  // back NOT running means it has already finished - which for a job over one
  // or two songs it may well have. onSettled fires exactly once either way.
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

  return { status, poll, watch }
}

export const useSilenceAnalyzeStatus = () => useSilenceStatus(ANALYZE_URL)
export const useSilenceTrimStatus = () => useSilenceStatus(TRIM_URL)
