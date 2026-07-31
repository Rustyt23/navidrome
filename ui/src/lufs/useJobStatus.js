import { useCallback, useEffect, useRef, useState } from 'react'
import { useRefresh } from 'react-admin'
import { httpClient } from '../dataProvider'

// How hard to poll just after a job is kicked off, and for how long. A job over
// a handful of tracks can start and finish inside a single ordinary interval,
// so on that schedule it would appear to have done nothing at all.
const WATCH_INTERVAL_MS = 400
const WATCH_ATTEMPTS = 25
const IDLE_INTERVAL_MS = 2000
// follow's cadence. Slower than watch because it may run for hours, and once a
// job is known to be going there is nothing to catch quickly.
const FOLLOW_INTERVAL_MS = 1500

// One shared record per job, and every component watching that job reads it.
//
// They used to have one each. Each call to this hook built its own state and its
// own poller, which meant a component that started a job only ever updated
// itself: pressing "Optimize LUFS" in the selection toolbar told the button's
// copy that a run was going, while the list's copy - the one that draws the
// progress bar and the stop button - carried on believing nothing was happening.
// The run was real and correctly tracked, just not by the thing that displays
// it. Sharing the record removes the whole class of that bug rather than wiring
// each pair of components together by hand, and halves the polling as well.
const stores = new Map()

const getStore = (url) => {
  let store = stores.get(url)
  if (!store) {
    store = {
      url,
      status: null,
      listeners: new Set(),
      timer: null,
      inFlight: null,
    }
    stores.set(url, store)
  }
  return store
}

// Only poll on a timer while a job is actually going, and only while somebody is
// watching. watch and follow drive their own faster ticks on top of this.
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
// watch/follow ticks can land together, and they all want the same answer.
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

// useJobStatus follows one of the long-running loudness jobs: it polls while
// the job is going, and refreshes the list as completed tracks persist their
// audit records.
//
// Both jobs behave the same way and differ only in their URL, so they share
// this rather than each growing their own copy of the polling - and their own
// copy of every bug in it.
export const useJobStatus = (url) => {
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

  // follow stays with a job it has just started until the job ends, however
  // long that takes, and reports the finished counts.
  //
  // It differs from watch in what it is for: watch answers "did that start?"
  // and gives up quickly, which is right for a caller that only needs to clear
  // a spinner. follow answers "how did it turn out?", which a button that used
  // to make a blocking request needs, since its caller no longer learns the
  // outcome from the response.
  //
  // Safe to poll immediately: the request that starts a job marks it running
  // before it replies, so the first look never mistakes "not begun yet" for
  // "already over". Returns a function that stops the following - call it when
  // the caller goes away; the job itself is unaffected.
  const follow = useCallback(
    (onSettled) => {
      let stopped = false
      const tick = () => {
        poll().then((json) => {
          if (stopped) return
          if (json && !json.running) {
            onSettled?.(json)
            return
          }
          setTimeout(tick, FOLLOW_INTERVAL_MS)
        })
      }
      setTimeout(tick, WATCH_INTERVAL_MS)
      return () => {
        stopped = true
      }
    },
    [poll],
  )

  return { status, poll, watch, follow }
}

export default useJobStatus
