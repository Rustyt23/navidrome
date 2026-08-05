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
    expect(o[DECISION_CEILING].sole).toContain('0.91 dB louder than -12.60')
    expect(o[DECISION_CEILING].sole).toContain('does nothing')
    expect(JSON.stringify(o)).not.toContain('0.00 dB')
  })

  it('says plainly when gain-to-ceiling is just leave-alone', () => {
    // joycelyn's dance: peaks at -0.49 leave no room once spring-back is
    // allowed for, so this option cannot move the song at all. Offering it as
    // a third choice invites someone to pick it expecting a third result.
    const o = song(-13.13, -0.49)
    expect(o[DECISION_CEILING].sole).toContain('no room to turn this one up')
    expect(o[DECISION_CEILING].lands).toBe('-13.13 LUFS')
    expect(o[DECISION_CEILING].lands).toBe(o[DECISION_SKIP].lands)
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
