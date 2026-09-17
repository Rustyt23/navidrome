import { describe, it, expect } from 'vitest'
import {
  ANALYZE_URL,
  CLEAR_URL,
  SUMMARY_URL,
  TRIM_URL,
} from './useSilenceStatus'

// httpClient only applies baseUrl(), which does not add the /api prefix. A bare
// "song/silence/summary" is therefore resolved relative to the current page -
// under the hash router that means localhost/song/silence/summary, which 404s
// and shows "Something went wrong" the moment the page opens.
//
// Every silence endpoint goes through these constants for that reason, so this
// one check covers all of them.
describe('silence endpoint URLs', () => {
  const urls = { ANALYZE_URL, TRIM_URL, SUMMARY_URL, CLEAR_URL }

  it.each(Object.entries(urls))('%s is absolute and under /api', (_name, url) => {
    expect(url.startsWith('/api/song/silence')).toBe(true)
  })

  it('derives the stop endpoints from the same base', () => {
    expect(`${ANALYZE_URL}/stop`).toBe('/api/song/silence/analyze/stop')
    expect(`${TRIM_URL}/stop`).toBe('/api/song/silence/trim/stop')
  })
})
