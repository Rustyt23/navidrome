import { describe, expect, it } from 'vitest'
import { isException, isLevelTwo, outcomeFor } from './outcome'

it.each([-12.8, -12.4])(
  'reports %s LUFS as on target at the inclusive boundary',
  (lufs) => {
    const audit = { phase: 0, lufsBefore: lufs, tpBefore: -1 }
    const settings = { targetLUFS: -12.6, tolerance: 0.2, truePeak: -0.5 }
    expect(outcomeFor({ loudnessAudit: audit }, settings).id).toBe('on_target')
    expect(isLevelTwo(audit, settings)).toBe(false)
  },
)

// The same rows the server's exceptions filter is tested against, so the
// verdict column and the Exception LUFS page agree about every song.
describe('isException', () => {
  const offBy = (lufs) => Math.abs(lufs + 12.6)

  it('lists a rewritten file whose peak is beyond the shipping bound', () => {
    const audit = { phase: 0, action: 'gain', tpBefore: -8, tpAfter: -0.3 }
    expect(isException(audit, offBy(-12.6), -0.5)).toBe(true)
  })

  it('does not list a rewritten file exactly on the ceiling', () => {
    const audit = { phase: 0, action: 'gain', tpBefore: -8, tpAfter: -0.5 }
    expect(isException(audit, offBy(-12.6), -0.5)).toBe(false)
  })

  // Accepted on purpose, inside the measurement tolerance. Listing it here
  // put songs the engine had corrected exactly in front of the client as
  // decisions to make - which is what this whole bound exists to prevent.
  it('does not list a peak inside the measurement tolerance', () => {
    const audit = { phase: 0, action: 'gain', tpBefore: -8, tpAfter: -0.49 }
    expect(isException(audit, offBy(-12.6), -0.5)).toBe(false)
  })

  // The last gate before a song reaches the client. A correction was attempted
  // and could not land - re-encoding pushed the peak back up, or the loudness
  // came out somewhere else - so the produced file was thrown away and the
  // song is exactly as the client delivered it. Inside the wider band that is
  // not a decision for anyone: the peak on the row is the client's own master,
  // and nothing allowed here can change it without moving the song out of the
  // band it is being kept in.
  it('does not list a failed correction that is already near target', () => {
    for (const peak of [0.63, 0, -0.2, 1.4]) {
      const audit = {
        phase: 3,
        action: 'refused',
        lufsBefore: -12.15,
        tpBefore: peak,
      }
      expect(isException(audit, offBy(-12.15), -0.5)).toBe(false)
    }
  })

  it('still lists a failed correction that is far from target', () => {
    const audit = {
      phase: 3,
      action: 'refused',
      lufsBefore: -14.2,
      tpBefore: -3,
    }
    expect(isException(audit, offBy(-14.2), -0.5)).toBe(true)
  })

  // wasException is a one-way latch: nothing in the application lowers it, so
  // every song the old peak rule wrongly listed carries it for ever. Honouring
  // it on a song that is now finished would keep those on the page permanently,
  // and no correction to the rules could ever release them.
  it('releases a latched song once it is finished', () => {
    const fixed = {
      phase: 0,
      action: 'gain',
      wasException: true,
      lufsAfter: -12.6,
      tpAfter: -0.43,
    }
    expect(isException(fixed, offBy(-12.6), -0.5, 0.2)).toBe(false)
  })

  // The shape that filled the client's page: on target as it was mastered,
  // never rewritten, with the peak a commercial master ordinarily carries. The
  // planner calls it finished, so this must agree - the peak cannot be brought
  // down without carrying the loudness out of the band with it.
  it('releases a latched in-band song whatever its peak', () => {
    for (const peak of [-0.26, 0.04, 0.28, 1.5]) {
      const mastered = {
        phase: 0,
        action: 'skipped',
        wasException: true,
        lufsBefore: -12.53,
        tpBefore: peak,
      }
      expect(isException(mastered, offBy(-12.53), -0.5, 0.2)).toBe(false)
    }
  })

  it('keeps a latched song listed while it still needs something', () => {
    const stillOut = {
      phase: 1,
      action: 'gain',
      wasException: true,
      lufsAfter: -13.6,
      tpAfter: -2,
    }
    expect(isException(stillOut, offBy(-13.6), -0.5, 0.2)).toBe(true)
  })

  it('does not list a song that has only been measured', () => {
    const nearWithPeaksOver = {
      phase: 4,
      action: '',
      lufsBefore: -12.9,
      tpBefore: 1,
    }
    const farWithPeaksOver = {
      phase: 1,
      action: '',
      lufsBefore: -20,
      tpBefore: 0.26,
    }
    expect(isException(nearWithPeaksOver, offBy(-12.9), -0.5)).toBe(false)
    expect(isException(farWithPeaksOver, offBy(-20), -0.5)).toBe(false)
  })

  it('still lists a refusal far from target, and a pending review', () => {
    const refused = {
      phase: 1,
      action: 'refused',
      lufsBefore: -14,
      tpBefore: -3,
    }
    expect(isException(refused, offBy(-14), -0.5)).toBe(true)
    expect(
      isException({ phase: 2, lufsBefore: -9, tpBefore: -3 }, 3.6, -0.5),
    ).toBe(true)
  })
})
