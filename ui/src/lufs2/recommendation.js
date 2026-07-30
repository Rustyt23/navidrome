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

// The highest a finished file may peak. Mirrors fallbackCeilingDB in
// core/loudness: still below zero, so nothing can clip.
const SAFE_PEAK = -0.1

// What a rewrite costs a source of this bitrate, before any gain is applied.
// Both figures are measured, not estimated: sources were rewritten at their own
// bitrate with no gain at all, three tracks each.
//
//   loudness lost:  128k -0.46   192k -0.26   256k 0.00   320k 0.00
//   peak gained:    128k +0.25   320k  0.00
//
// The peak was only measured at the two ends, so everything below 256k is
// given the degraded allowance rather than assumed clean - an estimate that is
// too optimistic here would promise a result the file cannot deliver.
const rewriteCost = (bitRate) =>
  !bitRate ? 0 : bitRate <= 160 ? 0.46 : bitRate <= 224 ? 0.26 : 0
const peakSpringBack = (bitRate) => (!bitRate || bitRate > 224 ? 0 : 0.25)

// bestWithoutDistortion works out how close a volume-only change can bring a
// song, and whether that is worth doing at all.
//
// Two things stop a pure gain reaching the target. The peaks run out of room -
// the obvious one, visible in the numbers on this page. And rewriting a
// degraded file costs loudness on its own, so part of any gain is spent just
// standing still: on a 128k source nearly half a decibel goes before the song
// gets any louder. A track can therefore end up FURTHER from target after a
// rewrite than it was before, which is worth saying plainly rather than
// leaving someone to discover it.
//
// The result is an estimate. It is exact on a clean source, where the peak
// follows the gain to within a couple of hundredths. On a degraded one the peak
// can jump unpredictably as the gain rises, so the real ceiling may be lower.
const bestWithoutDistortion = (lufs, peak, target, bitRate) => {
  const cost = rewriteCost(bitRate)
  const headroom = SAFE_PEAK - peakSpringBack(bitRate) - peak
  const gain = Math.min(target - lufs + cost, headroom)
  const lands = lufs + gain - cost

  const offNow = Math.abs(lufs - target)
  const offAfter = Math.abs(lands - target)
  const gains = offNow - offAfter

  // A song whose peaks are already at or over the limit has negative headroom,
  // so the arithmetic above offers to turn it DOWN - which would make the peak
  // safe and the loudness worse. That is not an answer to "how close can we
  // get", and proposing it would send someone off to make a song quieter in
  // the name of reaching a target it is already short of.
  //
  // Rewriting a degraded file also costs a generation of quality outright, so
  // there it has to buy something worth having. A fifth of a decibel is not:
  // nobody can hear it, and the quality does not come back.
  const worthwhile = cost > 0 ? 0.5 : 0.1
  if (gains <= 0 || gains < worthwhile) {
    return {
      lufs,
      offBy: offNow,
      gains: gains > 0 ? gains : 0,
      worthDoing: false,
      onTarget: offNow <= 0.2,
      estimated: false,
    }
  }
  return {
    lufs: lands,
    offBy: offAfter,
    gains,
    worthDoing: true,
    onTarget: offAfter <= 0.2,
    estimated: cost > 0,
  }
}

export const recommendationFor = (record, settings) => {
  const audit = record?.loudnessAudit
  const target = settings?.targetLUFS ?? -12.6
  const ceiling = settings?.truePeak ?? -0.5
  if (!audit || audit.lufsBefore == null || audit.tpBefore == null) {
    return null
  }

  const lufs = Number(audit.lufsBefore)
  const peak = Number(audit.tpBefore)
  const bitRate = Number(audit.bitrateBefore || record?.bitRate || 0)

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
    bitRate,
    best: bestWithoutDistortion(lufs, peak, target, bitRate),
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
