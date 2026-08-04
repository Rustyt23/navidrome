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

// What a rewrite costs a source of this bitrate in loudness, before any gain is
// applied. Measured, not estimated: sources were rewritten at their own bitrate
// with no gain at all, three tracks each.
//
//   128k -0.46   192k -0.26   256k 0.00   320k 0.00
//
// Mirrors RewriteLoudnessCost in core/ffmpeg, so the page and the engine agree
// about what a given file will cost to rewrite.
const rewriteCost = (bitRate) =>
  !bitRate ? 0 : bitRate <= 160 ? 0.46 : bitRate <= 224 ? 0.26 : 0

// How far above the arithmetic a finished file's true peak lands, for a plain
// gain with no limiting.
//
// An mp3 does not store samples, it stores a recipe for rebuilding them. The
// rebuilt waveform is close to the original but not identical, and some
// reconstructed points land higher than the ones they replace. The coarser the
// recipe, the further.
//
// These are upper bounds, not averages, and that is the whole point. The
// previous figure - a flat 0.25 below 224k and nothing above it - was the
// middle of the observed range, so roughly half of all low-bitrate files
// exceeded it. Two 128k tracks measured on this library sprang back 0.24 and
// 0.42: the page showed both as having headroom to spare, the engine tried
// them, and both came back over the ceiling and were refused. An allowance
// that is right on average is wrong for a safety margin - it has to hold for
// the worst file, not the typical one, which is how rewriteCost above is
// already built ("never optimistic").
//
// Zero above 224k was not measured at all. Nothing is the one value that cannot
// fail safe, so a small allowance stands in until it is.
const peakSpringBack = (bitRate) => {
  if (!bitRate) return 0.5 // unknown: assume the worst rather than promise the best
  if (bitRate <= 160) return 0.5
  if (bitRate <= 256) return 0.3
  return 0.15
}

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
  const springBack = peakSpringBack(bitRate)

  // What it would take to hit the target, and where the peaks would land.
  //
  // Two answers, because the arithmetic and the finished file disagree.
  // predictedPeak is exact for the decoded waveform: a gain moves every sample
  // by itself, so it moves the peak by itself. What ships is that waveform
  // re-encoded, and re-encoding puts some of the peak back.
  //
  // Every number below is built on the encoded figure, because that is the one
  // the engine will measure and judge the file by. Reading the headroom off the
  // arithmetic alone told this page a track had room to spare while the file it
  // produced landed over the ceiling and was thrown away.
  const gainToTarget = target - lufs
  // What actually has to be applied. Rewriting a degraded file loses loudness
  // before any gain takes effect, so part of the gain is spent standing still -
  // and that part raises the peaks along with the rest of it. bestWithoutDistortion
  // below has always accounted for this; the headroom shown on the page did not,
  // which left the two halves of the same file disagreeing about the same track.
  const cost = rewriteCost(bitRate)
  const gainToReachTarget = gainToTarget + cost
  const predictedPeak = peak + gainToReachTarget
  const predictedPeakEncoded = predictedPeak + springBack
  const peakOverBy = Math.max(0, predictedPeakEncoded - ceiling)

  // The most we can lift it without pushing peaks past the ceiling - again
  // measured on what ships, so the allowance is spent here rather than
  // discovered afterwards.
  //
  // The shortfall this leaves and the cut the other option needs are the same
  // number, necessarily: both are the headroom the track does not have.
  //
  // It floors at zero: a track whose peaks already leave no room cannot be
  // lifted at all, and a negative "gain" here would be an offer to turn the
  // song down in the name of making it louder.
  const transparentGain = Math.max(0, ceiling - peak - springBack)
  const loudnessAtCeiling = lufs + transparentGain
  const shortfall = Math.max(0, target - loudnessAtCeiling)

  const best = bestWithoutDistortion(lufs, peak, target, bitRate)

  return {
    lufs,
    peak,
    bitRate,
    best,
    // Peak-to-loudness ratio: how much headroom the track's dynamics demand.
    plr: peak - lufs,
    gainToTarget,
    gainToReachTarget,
    rewriteCost: cost,
    predictedPeak,
    predictedPeakEncoded,
    springBack,
    peakOverBy,
    transparentGain,
    loudnessAtCeiling,
    shortfall,
    target,
    ceiling,
    ...suggest(best, peakOverBy, target, lufs, cost),
  }
}

// suggest picks the option to put in front of the client.
//
// Whether the peak cut is audible is the second question, not the first. The
// first is whether rewriting the file is worth doing at all: on a degraded
// source a rewrite costs a generation of quality outright, and a track already
// close to target buys back less than it spends. Recommending "limit to target"
// there contradicted the column beside it, which had done that arithmetic and
// said plainly that leaving the song alone was as close as it would get.
const suggest = (best, peakOverBy, target, lufs, cost) => {
  if (!best.worthDoing) {
    const offBy = Math.abs(lufs - target).toFixed(2)
    return {
      suggested: DECISION_SKIP,
      suggestedBecause: best.onTarget
        ? `already within ${offBy} dB of ${target.toFixed(2)}`
        : cost > 0
          ? `rewriting costs ${cost.toFixed(2)} dB of loudness on a file this size, more than the ${offBy} dB it would gain`
          : `no closer than leaving it alone`,
    }
  }
  if (peakOverBy > AUDIBLE_SHAVE_DB) {
    return {
      suggested: DECISION_CEILING,
      suggestedBecause: `cutting ${peakOverBy.toFixed(2)} dB off the peaks would be audible`,
    }
  }
  return {
    suggested: DECISION_LIMIT,
    suggestedBecause: `${peakOverBy.toFixed(2)} dB off the peaks is inaudible, and it reaches ${target.toFixed(2)}`,
  }
}

export const fmtDb = (v, digits = 2) =>
  v == null || Number.isNaN(v)
    ? '-'
    : `${v > 0 ? '+' : ''}${Number(v).toFixed(digits)}`

export const fmtLufs = (v, digits = 2) =>
  v == null || Number.isNaN(v) ? '-' : Number(v).toFixed(digits)
