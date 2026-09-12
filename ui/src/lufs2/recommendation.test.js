import { describe, it, expect } from 'vitest'
import {
  DECISION_CEILING,
  DECISION_LIMIT,
  DECISION_SKIP,
  optionsFor,
  recommendationFor,
} from './recommendation'

const settings = { targetLUFS: -12.6, truePeak: -0.5 }
const song = (lufsBefore, tpBefore, bitrateBefore = 320) =>
  optionsFor(
    recommendationFor(
      { loudnessAudit: { lufsBefore, tpBefore, bitrateBefore } },
      settings,
    ),
  )

// These describe what each decision does to a song, and they are read by
// somebody deciding whether to approve a page of them. Every case below is a
// real row from the library that the previous wording described incorrectly -
// not a hypothetical - which is why they are pinned.
describe('optionsFor', () => {
  it('states the trade for a song that can reach target by trimming peaks', () => {
    // Window to the Soul: 1.25 dB down, peaks with a little room.
    const o = song(-13.85, -1.28)
    expect(o[DECISION_LIMIT].cost).toContain('too little to hear')
    expect(o[DECISION_CEILING].cost).toContain('below -12.60')
    expect(o[DECISION_SKIP].cost).toContain('1.25 dB below -12.60')
  })

  it('does not claim a song louder than target is short of it', () => {
    // Fasnating Groove: -11.69 LUFS, peaks at +0.98 - already over the target
    // and clipping. shortfall floors at zero, so this used to read "stops 0.00
    // dB short of -12.60" on a song 0.91 dB the wrong side of it.
    const o = song(-11.69, 0.98)
    expect(o[DECISION_SKIP].cost).toBe('it stays 0.91 dB louder than -12.60')
    expect(o[DECISION_CEILING].gain).toContain('1.63 dB quieter')
    expect(o[DECISION_CEILING].lands).toBe('about -13.32 LUFS')
    expect(JSON.stringify(o)).not.toContain('0.00 dB')
  })

  it('shows the reduction when peaks leave no room to turn up', () => {
    const o = song(-13.13, -0.49)
    expect(o[DECISION_CEILING].short).toBe('0.16 dB quieter, no limiting')
    expect(o[DECISION_CEILING].lands).toBe('about -13.29 LUFS')
    expect(o[DECISION_CEILING].lands).not.toBe(o[DECISION_SKIP].lands)
  })

  it('describes the reported mismatch using the server gain', () => {
    const o = song(-14, 0.2)
    expect(o[DECISION_CEILING].short).toBe('0.85 dB quieter, no limiting')
    expect(o[DECISION_CEILING].lands).toBe('about -14.85 LUFS')
    expect(o[DECISION_CEILING].cost).toContain('unsafe output is rejected')
  })

  it('leaves safe songs within the configured tolerance unchanged', () => {
    const rec = recommendationFor(
      {
        loudnessAudit: { lufsBefore: -12.9, tpBefore: -2, bitrateBefore: 320 },
      },
      { ...settings, tolerance: 0.4 },
    )
    expect(optionsFor(rec)[DECISION_CEILING].lands).toBe('-12.90 LUFS')
    expect(optionsFor(rec)[DECISION_CEILING].short).toContain('no change')
    expect(song(-12.6, 0.2)[DECISION_CEILING].short).toContain('quieter')
  })

  it('shows no change for a volume adjustment smaller than the server minimum', () => {
    const o = song(-14, -0.6)
    expect(o[DECISION_CEILING].lands).toBe('-14.00 LUFS')
    expect(o[DECISION_CEILING].short).toContain('no change')
  })

  it('does not describe a pure volume change as a peak cut', () => {
    // Little Love 2020: needs turning DOWN, so no limiting happens. "0.00 dB
    // off the peaks is inaudible" was true and told nobody anything.
    const o = song(-11.01, -0.01)
    expect(o[DECISION_LIMIT].cost).toContain('only a volume change')
    expect(o[DECISION_LIMIT].cost).not.toContain('0.00 dB')
  })

  it('calls a deep cut audible', () => {
    // Coast to Coast: 3.96 dB down with no headroom - a real decision.
    const o = song(-16.56, -1.25)
    expect(o[DECISION_LIMIT].cost).toContain('soften every drum hit')
    expect(o[DECISION_CEILING].cost).toContain('3.36 dB below -12.60')
  })

  it('never prefixes a magnitude with a sign in prose', () => {
    // "+3.96 dB below the target" is the kind of line that stops a reader.
    for (const o of [
      song(-16.56, -1.25),
      song(-11.69, 0.98),
      song(-13.85, -1.28),
      song(-13.21, 2.43, 128),
    ]) {
      expect(JSON.stringify(o)).not.toMatch(/[+][\d]/)
    }
  })
})

// The one-line form used by both the Suggested column and the Final result
// column. It carries the same claims as the long form and has to break the
// same way on the same songs - two wordings of one fact drifting apart is how
// the page came to say "0.00 dB short" about a song 0.91 dB too loud.
describe('optionsFor short form', () => {
  it('names the peak cut when there is one', () => {
    expect(song(-13.85, -1.28)[DECISION_LIMIT].short).toBe(
      '0.62 dB off the peaks',
    )
  })

  it('says volume-only when no limiting happens', () => {
    expect(song(-11.01, -0.01)[DECISION_LIMIT].short).toBe(
      'volume only, peaks untouched',
    )
  })

  it('does not claim a shortfall a song does not have', () => {
    expect(song(-11.69, 0.98)[DECISION_CEILING].short).toBe(
      '1.63 dB quieter, no limiting',
    )
  })

  it('reports the real shortfall when the option does move the song', () => {
    expect(song(-16.56, -1.25)[DECISION_CEILING].short).toBe(
      '0.60 dB louder, no limiting',
    )
  })

  it('gives every option a short form on every song', () => {
    for (const o of [
      song(-16.56, -1.25),
      song(-11.69, 0.98),
      song(-13.85, -1.28),
      song(-11.01, -0.01),
      song(-13.21, 2.43, 128),
    ]) {
      for (const d of [DECISION_LIMIT, DECISION_CEILING, DECISION_SKIP]) {
        expect(o[d].short).toBeTruthy()
      }
    }
  })
})
