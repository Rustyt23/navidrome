import { renderHook, act } from '@testing-library/react-hooks'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useJobStatus } from './useJobStatus'
import { httpClient } from '../dataProvider'

vi.mock('../dataProvider', () => ({
  httpClient: vi.fn(),
}))

vi.mock('react-admin', async () => {
  const actual = await vi.importActual('react-admin')
  return { ...actual, useRefresh: vi.fn(() => vi.fn()) }
})

// Each test gets its own URL: the store is module-level and deliberately
// outlives any one component, so reusing a URL would leak state between tests.
let nextUrl = 0
const freshUrl = () => `/api/job/${(nextUrl += 1)}`

const respondWith = (json) => httpClient.mockResolvedValue({ json })

describe('useJobStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    respondWith({ running: false })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('reports what the job says', async () => {
    respondWith({ running: true, processed: 3, total: 10 })
    const url = freshUrl()

    const { result, waitFor } = renderHook(() => useJobStatus(url))

    await waitFor(() => result.current.status !== null)
    expect(result.current.status.processed).toBe(3)
  })

  // The bug this sharing exists to fix: a component that starts a job used to
  // update only its own copy, so the list that draws the progress bar never
  // learned a run was going.
  it('shows one component the job another component started', async () => {
    const url = freshUrl()
    respondWith({ running: false })

    const list = renderHook(() => useJobStatus(url))
    const button = renderHook(() => useJobStatus(url))
    await list.waitFor(() => list.result.current.status !== null)
    expect(list.result.current.status.running).toBe(false)

    respondWith({ running: true, processed: 1, total: 4 })
    await act(async () => {
      await button.result.current.poll()
    })

    expect(button.result.current.status.running).toBe(true)
    expect(list.result.current.status.running).toBe(true)
    expect(list.result.current.status.processed).toBe(1)
  })

  it('asks once when several watchers poll together', async () => {
    const url = freshUrl()
    const first = renderHook(() => useJobStatus(url))
    const second = renderHook(() => useJobStatus(url))
    await first.waitFor(() => first.result.current.status !== null)

    httpClient.mockClear()
    await act(async () => {
      await Promise.all([
        first.result.current.poll(),
        second.result.current.poll(),
      ])
    })

    expect(httpClient).toHaveBeenCalledTimes(1)
  })

  it('keeps the shared record when one watcher goes away', async () => {
    const url = freshUrl()
    respondWith({ running: true, processed: 2 })

    const list = renderHook(() => useJobStatus(url))
    const button = renderHook(() => useJobStatus(url))
    await list.waitFor(() => list.result.current.status !== null)

    button.unmount()

    respondWith({ running: true, processed: 5 })
    await act(async () => {
      await list.result.current.poll()
    })
    expect(list.result.current.status.processed).toBe(5)
  })

  it('follow reports the finished job', async () => {
    const url = freshUrl()
    respondWith({ running: false, normalized: 4, skipped: 1, failed: 0 })
    const { result, waitFor } = renderHook(() => useJobStatus(url))
    await waitFor(() => result.current.status !== null)

    let settled = null
    await act(async () => {
      result.current.follow((json) => {
        settled = json
      })
      await new Promise((resolve) => setTimeout(resolve, 600))
    })

    expect(settled).toMatchObject({ normalized: 4, skipped: 1, failed: 0 })
  })

  it('never announces completion because time passed or a status request failed', async () => {
    vi.useFakeTimers()
    respondWith({ running: true, normalized: 1, total: 5 })
    const url = freshUrl()
    const { result, unmount } = renderHook(() => useJobStatus(url))
    const done = vi.fn()
    let stop
    await act(async () => {
      stop = result.current.follow(done)
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000)
    })
    expect(done).not.toHaveBeenCalled()
    httpClient.mockRejectedValue(new Error('offline'))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000)
    })
    expect(done).not.toHaveBeenCalled()
    respondWith({ running: false, normalized: 3, failed: 1, skipped: 1 })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(done).toHaveBeenCalledTimes(1)
    expect(done).toHaveBeenCalledWith({
      running: false,
      normalized: 3,
      failed: 1,
      skipped: 1,
    })
    stop()
    unmount()
  })

  it('ignores an idle response from a request sent before the job started', async () => {
    vi.useFakeTimers()
    let releaseOld
    httpClient.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          releaseOld = resolve
        }),
    )
    const url = freshUrl()
    const { result, unmount } = renderHook(() => useJobStatus(url))
    const done = vi.fn()
    const stop = result.current.follow(done)
    respondWith({ running: true })
    await act(async () => {
      releaseOld({ json: { running: false } })
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000)
    })
    expect(done).not.toHaveBeenCalled()
    stop()
    unmount()
  })

  it('stops following when its caller leaves', async () => {
    vi.useFakeTimers()
    const url = freshUrl()
    const { result, unmount } = renderHook(() => useJobStatus(url))
    const done = vi.fn()
    const stop = result.current.follow(done)
    stop()
    unmount()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(done).not.toHaveBeenCalled()
  })
})
