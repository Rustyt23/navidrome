import { describe, it, expect } from 'vitest'
import { explainRefusal } from './reason'

// The strings below are not invented: they are every distinct rejection the
// engine has actually written into this library's audit table, plus the two
// shapes rejection() can emit that have not come up yet. A parser tested only
// against strings its author made up proves that the regexes match themselves.
describe('explainRefusal', () => {
  it('explains the common case - the encoder put the peaks back', () => {
    const { headline, detail } = explainRefusal(
      'true peak 0.16 dBTP exceeds the -0.50 dBTP ceiling',
    )
    expect(headline).toBe('Re-encoding pushed the peaks back up')
    expect(detail).toContain('0.16 dBTP')
    // The overshoot is the number that decides whether this row matters, and
    // it is the one the engine never states.
    expect(detail).toContain('0.66 dB above the -0.50')
    expect(detail).toContain('Your original file is untouched.')
    // Magnitude, not a signed quantity: "+0.66 dB above" reads as a typo.
    expect(detail).not.toMatch(/[+][\d]/)
  })

  it('reads a peak that is over the ceiling but still below zero', () => {
    // -0.08 is over a -0.50 ceiling and cannot clip. Getting the sign wrong
    // here would report a song as 0.42 dB *under* the line it failed.
    const { detail } = explainRefusal(
      'true peak -0.08 dBTP exceeds the -0.50 dBTP ceiling',
    )
    expect(detail).toContain('0.42 dB above the -0.50')
  })

  it('explains a loudness miss', () => {
    const { headline, detail } = explainRefusal(
      'landed at -12.92 LUFS, expected -12.60',
    )
    expect(headline).toBe('The rewrite did not land on the target')
    expect(detail).toContain('-12.92 LUFS instead of -12.60')
    expect(detail).toContain('0.32 dB out')
  })

  it('explains a format mismatch in words', () => {
    const { headline, detail } = explainRefusal(
      'output was not format-identical: [duration changed]',
    )
    expect(headline).toBe('The rewrite came back a different file')
    expect(detail).toContain('its length came out different')
    expect(detail).not.toContain('format-identical')
  })

  it('lists several format problems', () => {
    // Go prints []string as [a b c], so these arrive space-joined and are
    // detected rather than split.
    const { detail } = explainRefusal(
      'output was not format-identical: [codec mp3->aac duration changed cover art lost]',
    )
    expect(detail).toContain('its length came out different')
    expect(detail).toContain('the cover art was lost')
    expect(detail).toContain('the audio format changed')
    expect(detail).toContain(' and ')
  })

  it('keeps an unrecognised format problem rather than dropping it', () => {
    const { detail } = explainRefusal(
      'output was not format-identical: [something new]',
    )
    expect(detail).toContain('something new')
  })

  it('explains both misses together', () => {
    const { headline, detail } = explainRefusal(
      'landed at -12.90 LUFS (expected -12.60) and 0.20 dBTP (ceiling -0.50)',
    )
    expect(headline).toBe('The rewrite missed on loudness and peaks')
    expect(detail).toContain('-12.90 LUFS instead of -12.60')
    expect(detail).toContain('0.20 dBTP')
  })

  it('falls back to the raw text instead of going blank', () => {
    const { headline, detail } = explainRefusal('ffmpeg exited with status 137')
    expect(headline).toBe('The rewrite was rejected')
    expect(detail).toContain('ffmpeg exited with status 137')
    expect(detail).toContain('Your original file is untouched.')
  })

  it('survives a missing error', () => {
    expect(explainRefusal(undefined).detail).toBe(
      'Your original file is untouched.',
    )
    expect(explainRefusal(null).headline).toBe('The rewrite was rejected')
  })
})
