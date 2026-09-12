import { describe, expect, it } from 'vitest'
import { isException } from './outcome'

// The same rows the server's exceptions filter is tested against, so the
// verdict column and the Exception LUFS page agree about every song.
describe('isException', () => {
  const offBy = (lufs) => Math.abs(lufs + 12.6)

  it('lists a rewritten file whose peak is still over the ceiling', () => {
    const audit = { phase: 0, action: 'gain', tpBefore: -8, tpAfter: -0.49 }
    expect(isException(audit, offBy(-12.6), -0.5)).toBe(true)
  })

  it('does not list a rewritten file exactly on the ceiling', () => {
    const audit = { phase: 0, action: 'gain', tpBefore: -8, tpAfter: -0.5 }
    expect(isException(audit, offBy(-12.6), -0.5)).toBe(false)
  })

  it('lists a refusal that left the peaks over, even near target', () => {
    const audit = {
      phase: 3,
      action: 'refused',
      lufsBefore: -12.15,
      tpBefore: 0.63,
    }
    expect(isException(audit, offBy(-12.15), -0.5)).toBe(true)
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
