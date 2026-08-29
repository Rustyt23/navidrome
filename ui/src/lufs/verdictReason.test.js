import { describe, it, expect } from 'vitest'
import { verdictReason } from './verdictReason'

const settings = { targetLUFS: -12.6, tolerance: 0.2 }

describe('verdictReason', () => {
  it('quotes the song’s own numbers rather than defining the term', () => {
    const why = verdictReason(
      {
        lufsBefore: -13.15,
        lufsAfter: -12.71,
        gainApplied: 0.44,
        tpBefore: -1.0,
        tpAfter: -1.08,
      },
      settings,
      'dynamics_changed',
    )
    expect(why).toContain('-13.15 to -12.71 LUFS')
    expect(why).toContain('0.52 dB') // (-1.00 + 0.44) - (-1.08)
    expect(why).toContain('not audible')
  })

  it('explains what is outstanding, not what happened', () => {
    const why = verdictReason({ phase: 2 }, settings, 'needs_decision')
    expect(why).toContain('Exception LUFS page')
    expect(why).toContain('Nothing has been changed')
  })

  it('tells you an empty file cannot be retried into working', () => {
    const why = verdictReason({}, settings, 'no_audio')
    expect(why).toContain('fetching again')
    expect(why).toContain('Re-running will not help')
  })

  it('passes the engine’s own words through on a failure', () => {
    expect(
      verdictReason({ error: 'signal: killed' }, settings, 'failed'),
    ).toContain('signal: killed')
    expect(verdictReason({}, settings, 'failed')).toContain(
      'recorded no reason',
    )
  })

  it('says the original is safe when a build was thrown away', () => {
    const why = verdictReason(
      { error: 'true peak 0.16 dBTP exceeds' },
      settings,
      'left_as_is',
    )
    expect(why).toContain('your original is untouched')
    expect(why).toContain('true peak 0.16')
  })

  it('names the measurement for an untouched song', () => {
    expect(
      verdictReason({ lufsBefore: -12.55 }, settings, 'untouched'),
    ).toContain('-12.55 LUFS, already within 0.2 dB')
  })

  it('explains why level two was left alone', () => {
    const why = verdictReason({ lufsBefore: -13.05 }, settings, 'level_two')
    expect(why).toContain('half a decibel')
    expect(why).toContain('nobody can hear')
  })

  it('gives every verdict the page can show a reason', () => {
    const a = {
      lufsBefore: -14,
      lufsAfter: -12.6,
      gainApplied: 1.4,
      tpBefore: -3,
      tpAfter: -1.6,
    }
    for (const v of [
      'untouched',
      'safe',
      'dynamics_changed',
      'rewrite_costly',
      'reencoded',
      'failed',
      'no_audio',
      'left_as_is',
      'level_two',
      'needs_decision',
    ]) {
      expect(
        verdictReason(a, settings, v),
        `verdict ${v} has no reason`,
      ).toBeTruthy()
    }
  })

  it('survives a missing audit', () => {
    expect(verdictReason(null, settings, 'safe')).toBe('')
  })
})
