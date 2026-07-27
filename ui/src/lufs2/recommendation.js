// The whole of phase 2 rests on one fact: a constant gain moves the loudness
// and the true peak by exactly the same amount. So for any track we can work
// out, before touching it, whether the target is reachable without altering
// the audio - and if not, precisely what each option would cost.

export const DECISION_PENDING = ''
export const DECISION_LIMIT = 'limit'
export const DECISION_CEILING = 'gain_ceiling'
export const DECISION_SKIP = 'skip'

export const recommendationFor = (record, settings) => {
  const audit = record?.loudnessAudit
  const target = settings?.targetLUFS ?? -12.6
  const ceiling = settings?.truePeak ?? -1.5
  if (!audit || audit.lufsBefore == null || audit.tpBefore == null) {
    return null
  }

  const lufs = Number(audit.lufsBefore)
  const peak = Number(audit.tpBefore)

  // What it would take to hit the target, and where the peaks would land.
  const gainToTarget = target - lufs
  const predictedPeak = peak + gainToTarget
  const peakOverBy = Math.max(0, predictedPeak - ceiling)

  // The most we can lift it without pushing peaks past the ceiling.
  const transparentGain = ceiling - peak
  const loudnessAtCeiling = lufs + transparentGain
  const shortfall = Math.max(0, target - loudnessAtCeiling)

  return {
    lufs,
    peak,
    // Peak-to-loudness ratio: how much headroom the track's dynamics demand.
    plr: peak - lufs,
    gainToTarget,
    predictedPeak,
    peakOverBy,
    transparentGain,
    loudnessAtCeiling,
    shortfall,
    target,
    ceiling,
    // A small shortfall is barely audible, so gaining to the ceiling is the
    // sensible default; a large one means the choice genuinely matters.
    suggested: shortfall <= 1.0 ? DECISION_CEILING : DECISION_LIMIT,
  }
}

export const fmtDb = (v, digits = 2) =>
  v == null || Number.isNaN(v)
    ? '-'
    : `${v > 0 ? '+' : ''}${Number(v).toFixed(digits)}`

export const fmtLufs = (v, digits = 2) =>
  v == null || Number.isNaN(v) ? '-' : Number(v).toFixed(digits)
