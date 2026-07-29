// The whole of phase 2 rests on one fact: a constant gain moves the loudness
// and the true peak by exactly the same amount. So for any track we can work
// out, before touching it, whether the target is reachable without altering
// the audio - and if not, precisely what each option would cost.

export const DECISION_PENDING = ''
export const DECISION_LIMIT = 'limit'
export const DECISION_CEILING = 'gain_ceiling'
export const DECISION_SKIP = 'skip'

// How deep a peak reduction has to be before anyone can hear it.
//
// A true peak is not a passage of music: it is the single highest instant in
// the waveform, a handful of samples at the tip of one transient. Trimming a
// decibel off that is inaudible. Past roughly 3 dB the reduction stops being
// confined to the tip and begins softening the attack of every drum hit, and at
// that point the choice is a real one rather than a formality.
export const AUDIBLE_SHAVE_DB = 3.0

export const recommendationFor = (record, settings) => {
  const audit = record?.loudnessAudit
  const target = settings?.targetLUFS ?? -12.6
  const ceiling = settings?.truePeak ?? -0.5
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
  //
  // The shortfall this leaves and the cut the other option needs are the same
  // number, necessarily: both are the headroom the track does not have.
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
    // The target is the requirement, and gaining only to the ceiling misses it
    // by definition - that is what put this track here. So shaving is the
    // answer unless the cut is deep enough to be heard, and only then is there
    // anything for the client to weigh.
    //
    // Note this is the opposite of keying on how far the track would fall short
    // instead: a small shortfall means a small cut, which is precisely when
    // shaving costs least and is most worth doing.
    suggested:
      peakOverBy > AUDIBLE_SHAVE_DB ? DECISION_CEILING : DECISION_LIMIT,
    suggestedBecause:
      peakOverBy > AUDIBLE_SHAVE_DB
        ? `cutting ${peakOverBy.toFixed(2)} dB off the peaks would be audible`
        : `${peakOverBy.toFixed(2)} dB off the peaks is inaudible, and it reaches ${target.toFixed(2)}`,
  }
}

export const fmtDb = (v, digits = 2) =>
  v == null || Number.isNaN(v)
    ? '-'
    : `${v > 0 ? '+' : ''}${Number(v).toFixed(digits)}`

export const fmtLufs = (v, digits = 2) =>
  v == null || Number.isNaN(v) ? '-' : Number(v).toFixed(digits)
