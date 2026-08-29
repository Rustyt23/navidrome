import { describe, it, expect } from 'vitest'
import { reportFor } from './report'

const settings = { targetLUFS: -12.6, truePeak: -0.5, tolerance: 0.2 }
const processed = (extra) => ({
  loudnessAudit: {
    status: 'processed',
    action: 'gain',
    lufsBefore: -14.0,
    lufsAfter: -12.6,
    tpBefore: -2.0,
    tpAfter: -0.6,
    codecBefore: 'mp3',
    codecAfter: 'mp3',
    bitrateBefore: 320,
    bitrateAfter: 320,
    sampleRateBefore: 44100,
    sampleRateAfter: 44100,
    channelsBefore: 2,
    channelsAfter: 2,
    durationBefore: 200,
    durationAfter: 200,
    artBefore: true,
    artAfter: true,
    ...extra,
  },
})

// The engine ships a file above the configured ceiling when that ceiling turns
// out to be unreachable, records the fact internally, and then discards it.
// Those tracks looked identical to clean ones on every page.
describe('relaxed ceiling', () => {
  const lines = (r) => [
    ...r.applied,
    ...r.significant,
    ...r.minor,
    ...r.unchanged,
  ]

  it('says so when the finished peak is above the configured ceiling', () => {
    const r = reportFor(processed({ tpAfter: -0.12 }), settings)
    const said = lines(r).join(' | ')
    expect(said).toContain('-0.12 dBTP')
    expect(said).toContain('was not reachable')
    // It cannot clip, so it must not read as a fault.
    expect(said).toContain('cannot clip')
    expect(r.significant.join(' ')).not.toContain('was not reachable')
  })

  it('stays quiet for a file that respected the ceiling', () => {
    const r = reportFor(processed({ tpAfter: -0.62 }), settings)
    expect(lines(r).join(' ')).not.toContain('was not reachable')
  })

  it('does not fire on a peak sitting exactly on the ceiling', () => {
    const r = reportFor(processed({ tpAfter: -0.5 }), settings)
    expect(lines(r).join(' ')).not.toContain('was not reachable')
  })

  it('follows the configured ceiling rather than a hard-coded one', () => {
    const strict = { ...settings, truePeak: -1.5 }
    const r = reportFor(processed({ tpAfter: -0.6 }), strict)
    expect(lines(r).join(' ')).toContain('above the -1.50 target')
  })
})
