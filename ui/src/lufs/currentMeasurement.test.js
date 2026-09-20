import { describe, expect, it } from 'vitest'
import { currentMeasurement } from './currentMeasurement'
import { offByFor, outcomeFor } from './outcome'
import { recommendationFor } from '../lufs2/recommendation'
import { reportFor } from './report'

describe('current loudness', () => {
  it('flags outputs beyond the bound the engine ships at', () => {
    const report = reportFor(
      { loudnessAudit: { status: 'processed', tpBefore: -3, tpAfter: -0.3 } },
      { truePeak: -0.5 },
    )
    expect(
      report.significant.some((message) =>
        message.includes('exceeds the -0.50 ceiling'),
      ),
    ).toBe(true)
    expect(report.minor.some((message) => message.includes('ceiling'))).toBe(
      false,
    )
  })

  // The engine accepts a finished file up to TRUE_PEAK_TOLERANCE_DB above the
  // ceiling, because a true peak is reconstructed rather than read. Asking
  // someone to recheck a file that was accepted on purpose is the page
  // contradicting the run that produced it.
  it('does not ask for a recheck of a peak inside the measurement tolerance', () => {
    const report = reportFor(
      { loudnessAudit: { status: 'processed', tpBefore: -3, tpAfter: -0.49 } },
      { truePeak: -0.5 },
    )
    expect(
      report.significant.some((message) => message.includes('ceiling')),
    ).toBe(false)
  })
  it.each([
    undefined,
    {},
    { lufsBefore: null, tpBefore: null },
    { lufsBefore: '', tpBefore: Infinity },
  ])('does not invent measurements for %j', (audit) => {
    expect(currentMeasurement(audit).lufs).toBeNull()
    expect(offByFor(audit)).toBeNull()
    expect(outcomeFor({ loudnessAudit: audit })).toBeNull()
    expect(recommendationFor({ loudnessAudit: audit })).toBeNull()
  })

  it('uses the processed file for the current display and recommendations', () => {
    const audit = {
      lufsBefore: -22,
      tpBefore: -8,
      bitrateBefore: 128,
      lufsAfter: -13,
      tpAfter: -1,
      bitrateAfter: 320,
    }
    expect(currentMeasurement(audit)).toEqual({
      lufs: -13,
      peak: -1,
      bitRate: 320,
    })
    expect(recommendationFor({ loudnessAudit: audit })).toMatchObject({
      lufs: -13,
      peak: -1,
      bitRate: 320,
    })
    expect(offByFor(audit)).toBeCloseTo(0.4)
  })

  it('uses original measurements after restoration', () => {
    expect(
      currentMeasurement({
        lufsBefore: -22,
        tpBefore: -8,
        lufsAfter: null,
        tpAfter: null,
        restoredAt: '2026-09-09',
      }),
    ).toMatchObject({ lufs: -22, peak: -8 })
  })

  it('does not substitute original peaks when the current peak is missing', () => {
    const audit = { lufsBefore: -22, tpBefore: -8, lufsAfter: -12.6 }
    expect(currentMeasurement(audit).peak).toBeNull()
    expect(recommendationFor({ loudnessAudit: audit })).toBeNull()
    expect(outcomeFor({ loudnessAudit: audit }).id).not.toBe('on_target')
  })

  it('does not label an unsafe measured peak on target', () => {
    expect(
      outcomeFor({ loudnessAudit: { lufsBefore: -12.6, tpBefore: -0.3 } }).id,
    ).not.toBe('on_target')
  })

  // A song at the target whose peak sits inside the measurement tolerance is
  // on target. Judged against the bare ceiling it was reported as never
  // attempted, which dropped finished songs out of the headline count.
  it('counts a peak inside the measurement tolerance as on target', () => {
    expect(
      outcomeFor({ loudnessAudit: { lufsBefore: -12.6, tpBefore: -0.49 } }).id,
    ).toBe('on_target')
  })

  it('keeps a real zero measurement', () => {
    expect(currentMeasurement({ lufsBefore: 0, tpBefore: 0 })).toMatchObject({
      lufs: 0,
      peak: 0,
    })
  })

  it('bases predictions on the configured ceiling', () => {
    const record = {
      loudnessAudit: { lufsBefore: -20, tpBefore: -8, bitrateBefore: 320 },
    }
    const strict = recommendationFor(record, { truePeak: -3 })
    const relaxed = recommendationFor(record, { truePeak: -0.5 })
    expect(strict.best.lufs).toBeLessThan(relaxed.best.lufs)
  })
})
