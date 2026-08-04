// What actually happened to a song, and whether any of it matters.
//
// Three buckets, because one word was doing two jobs. "Significant" used to
// hold both "something is wrong" and "we did the thing you asked for", so a
// song that was rescued from clipping came out looking like a song that had
// been damaged. Capping a peak to reach the target is the job working, not a
// fault, and reporting it as a warning taught the reader to distrust the page.
//
//   applied     - what optimisation did on purpose
//   significant - what is actually wrong: less data per second, a different
//                 sample rate, squashed dynamics, lost artwork, a shifted
//                 duration, audio that no longer matches the original
//   minor       - unavoidable, inaudible side effects of rewriting a lossy file
//
// The loudness change itself is never a fault: it is the job.

const has = (v) => v !== null && v !== undefined && !Number.isNaN(Number(v))
const n = (v) => Number(v)
const f2 = (v) => n(v).toFixed(2)
const signed = (v) => `${n(v) > 0 ? '+' : ''}${n(v).toFixed(2)}`

// Thresholds, each tied to a physical reason rather than a round number.
const LRA_NOISE = 0.5 // loudness range moves this much on measurement noise alone

// How far the peak may stray from the gain before something other than the
// level moved it. Two tiers, mirroring core/loudness: evidence has to be
// stronger when it stands alone than when the null test agrees with it.
//
// One number here and a different one in the engine had the two disagreeing on
// the same row - the verdict saying "volume only" beside a report calling the
// same 0.3 dB a significant problem. They read the same measurement, so they
// have to read it the same way.
const PEAK_TRACKING = 0.15 // below this the peak tracked its gain; noise
const PEAK_STANDALONE = 0.5 // enough on its own to call it limiting

// Beyond this a gapless album develops a seam.
//
// Split by codec because only a lossy re-encode can move the length at all.
// Measured: an mp3 with a Xing/LAME header round-trips at 0.0 ms, and one
// without - an old or badly-made file, where the decoder cannot know how much
// padding to drop - drifts 28 to 39 ms. One frame is 26 ms, so two frames sit
// just over the old 50 ms bound. Lossless re-encoding moved nothing at all, so
// it keeps the tighter figure.
const DURATION_AUDIBLE_LOSSY = 0.1
const DURATION_AUDIBLE_LOSSLESS = 0.05
const DURATION_NOISE = 0.001

// Mirrors fallbackCeilingDB in core/loudness: where a degraded source cannot
// hold the configured ceiling, this is as high as its peak may ship. Still
// below 0, so the file cannot clip either way.
const FALLBACK_CEILING = -0.1

// The null test measures how much of the file changed, by energy. Rewriting a
// lossy file costs a generation of codec noise and lands at -39 to -47 dB on
// real music; a lossless round trip leaves almost nothing. Audio that was
// actually reshaped lands near -10. Anything above these floors means rewriting
// cost more than it should have - not that the dynamics were touched, which
// only the loudness range and the peak can tell us.
const LOSSLESS = [
  'flac',
  'alac',
  'wavpack',
  'pcm_s16le',
  'pcm_s24le',
  'pcm_s32le',
]
export const nullFloorFor = (codec, bitRate) => {
  if (LOSSLESS.includes(String(codec || '').toLowerCase())) return -60
  if (!bitRate) return -25 // unknown: assume the noisiest case
  if (bitRate <= 160) return -25
  if (bitRate <= 256) return -30
  return -35
}

export const reportFor = (record, settings) => {
  const a = record?.loudnessAudit
  if (!a || a.status !== 'processed') return null

  const ceiling = settings?.truePeak ?? -0.5
  const applied = []
  const significant = []
  const minor = []
  const unchanged = []

  // The intended change.
  let intended = null
  if (has(a.lufsBefore) && has(a.lufsAfter)) {
    const delta = n(a.lufsAfter) - n(a.lufsBefore)
    intended = `Loudness ${f2(a.lufsBefore)} → ${f2(a.lufsAfter)} LUFS (${signed(delta)} dB)`
  }

  // Prefer the audit's own after snapshot: it came from the same probe as the
  // before snapshot. media_file is only a fallback for records written before
  // the after snapshot existed, and its duration comes from a different
  // measurement, so a comparison against it is not meaningful.
  const hasAfterSnapshot = !!a.codecAfter
  const pick = (afterVal, recordVal) =>
    hasAfterSnapshot ? afterVal : recordVal

  // --- Things that must not change -----------------------------------------
  const codecAfter = pick(a.codecAfter, record?.suffix)
  if (a.codecBefore && codecAfter && a.codecBefore !== codecAfter) {
    significant.push(`Format changed: ${a.codecBefore} → ${codecAfter}`)
  } else if (a.codecBefore) {
    unchanged.push('format')
  }

  const bitrateAfter = pick(a.bitrateAfter, record?.bitRate)
  if (a.bitrateBefore && has(bitrateAfter)) {
    if (bitrateAfter < a.bitrateBefore) {
      significant.push(
        `Bitrate dropped ${a.bitrateBefore}k → ${bitrateAfter}k — permanent quality loss`,
      )
    } else if (bitrateAfter > a.bitrateBefore) {
      minor.push(
        `Bitrate raised ${a.bitrateBefore}k → ${bitrateAfter}k (no quality gained)`,
      )
    } else {
      unchanged.push('bitrate')
    }
  }

  const sampleRateAfter = pick(a.sampleRateAfter, record?.sampleRate)
  if (a.sampleRateBefore && has(sampleRateAfter)) {
    if (a.sampleRateBefore !== sampleRateAfter) {
      significant.push(
        `Sample rate changed ${a.sampleRateBefore} → ${sampleRateAfter} Hz — audio was resampled`,
      )
    } else {
      unchanged.push('sample rate')
    }
  }

  const bitDepthAfter = pick(a.bitDepthAfter, record?.bitDepth)
  if (a.bitDepthBefore && bitDepthAfter) {
    if (a.bitDepthBefore !== bitDepthAfter) {
      significant.push(
        `Bit depth changed ${a.bitDepthBefore} → ${bitDepthAfter} bit`,
      )
    } else {
      unchanged.push('bit depth')
    }
  }

  const channelsAfter = pick(a.channelsAfter, record?.channels)
  if (a.channelsBefore && has(channelsAfter)) {
    if (a.channelsBefore !== channelsAfter) {
      significant.push(
        `Channels changed ${a.channelsBefore} → ${channelsAfter}`,
      )
    } else {
      unchanged.push('channels')
    }
  }

  if (a.artBefore && !a.artAfter) {
    significant.push('Cover art destroyed')
  } else if (a.artBefore) {
    unchanged.push('cover art')
  }

  // Only comparable when both sides came from the same probe.
  if (a.durationBefore && hasAfterSnapshot && a.durationAfter) {
    const d = n(a.durationAfter) - n(a.durationBefore)
    const abs = Math.abs(d)
    const durationBound = LOSSLESS.includes(
      String(codecAfter || a.codecBefore || '').toLowerCase(),
    )
      ? DURATION_AUDIBLE_LOSSLESS
      : DURATION_AUDIBLE_LOSSY
    if (abs > durationBound) {
      significant.push(
        `Duration shifted ${signed(d)} s — gapless album playback would break`,
      )
    } else if (abs > DURATION_NOISE) {
      minor.push(`Duration shifted ${signed(d)} s (inaudible)`)
    } else {
      unchanged.push('duration')
    }
  }

  // --- Was the audio itself reshaped? --------------------------------------
  //
  // Capping a peak is what reaching the target costs on a song whose peaks are
  // in the way. It belongs under what was done, not under what went wrong -
  // unless the loudness range moved with it, which is the one signal that says
  // the music was squashed rather than its tips shaved.
  const lraMoved =
    has(a.lraBefore) && has(a.lraAfter) ? n(a.lraAfter) - n(a.lraBefore) : null
  const dynamicsSquashed = lraMoved !== null && Math.abs(lraMoved) > LRA_NOISE

  let cappedBy = null
  if (a.action === 'limited' && has(a.tpBefore) && has(a.tpAfter)) {
    const expected = has(a.gainApplied) ? n(a.gainApplied) : 0
    cappedBy = n(a.tpBefore) + expected - n(a.tpAfter)
  }

  if (a.action === 'limited') {
    const depth = cappedBy && cappedBy > 0 ? `${cappedBy.toFixed(2)} dB ` : ''
    // A song that arrived over 0 dBTP was clipping on every play. Saying so is
    // the difference between "we altered your audio" and "we fixed it".
    const rescued =
      has(a.tpBefore) &&
      n(a.tpBefore) > 0 &&
      has(a.tpAfter) &&
      n(a.tpAfter) <= 0
    applied.push(
      rescued
        ? `Peaks capped ${depth}to reach ${f2(ceiling)} dBTP — this song was clipping before (${signed(a.tpBefore)} dBTP) and now peaks at ${f2(a.tpAfter)}`
        : `Peaks capped ${depth}so the louder level stays under the ${f2(ceiling)} dBTP ceiling`,
    )
  }

  if (lraMoved !== null) {
    if (dynamicsSquashed) {
      significant.push(
        `Dynamics ${lraMoved < 0 ? 'compressed' : 'widened'}: loudness range ${f2(a.lraBefore)} → ${f2(a.lraAfter)} LU`,
      )
    } else if (Math.abs(lraMoved) > 0.05) {
      minor.push(
        `Loudness range moved ${signed(lraMoved)} LU (measurement noise)`,
      )
    } else {
      unchanged.push('dynamics')
    }
  }

  // Worked out before the peak block, which needs it: a peak movement too small
  // to convict on alone counts once the null test agrees the file really did
  // change.
  const nullFloor = nullFloorFor(
    codecAfter || a.codecBefore,
    bitrateAfter || a.bitrateBefore,
  )
  const nullFlagged = has(a.nullResidual) && n(a.nullResidual) > nullFloor

  // Under a pure gain the true peak moves by exactly the gain. A deviation
  // means something other than the level was altered.
  if (has(a.tpBefore) && has(a.tpAfter)) {
    const moved = n(a.tpAfter) - n(a.tpBefore)
    const expected = has(a.gainApplied) ? n(a.gainApplied) : moved
    if (n(a.tpAfter) > 0) {
      significant.push(
        `True peak ${f2(a.tpAfter)} dBTP — above 0, the file can clip`,
      )
      // Above the lowest ceiling the engine will ever accept. `+ 0.1` used to
      // sit here, which made this `> 0` - identical to the branch above, so it
      // never ran and a file shipped past the fallback was reported as a minor
      // note. The engine's own check is a hard bound with no slack, and this
      // mirrors it.
    } else if (n(a.tpAfter) > FALLBACK_CEILING) {
      significant.push(
        `True peak ${f2(a.tpAfter)} dBTP is above the ${f2(ceiling)} ceiling`,
      )
    } else if (n(a.tpAfter) > ceiling + 0.1) {
      // A degraded source whose codec pushes the peak back up further than
      // limiting can pull it down. The audio is unaffected and it still cannot
      // clip; what is smaller is the reserve left for later handling.
      minor.push(
        `True peak ${f2(a.tpAfter)} dBTP — above the ${f2(ceiling)} ceiling, which this file could not hold, but still clear of clipping`,
      )
    } else if (a.action === 'limited') {
      // Nothing to add: the capping is already reported under what was
      // applied, with its depth and its reason. Saying it again here as a
      // warning made one event look like two, and put a song that reached the
      // target exactly as intended into the "check this" count.
    } else if (
      expected - moved > PEAK_STANDALONE ||
      (expected - moved > PEAK_TRACKING && nullFlagged)
    ) {
      // The peak came out lower than the level change alone would put it, so
      // something pushed it down. That is what limiting does.
      //
      // A movement between the two tiers only counts with the null test
      // agreeing. On its own it is the encoder rebuilding the waveform, and
      // calling that "peaks pushed down" contradicted the verdict beside it,
      // which reads the same number against the engine's rule.
      significant.push(
        `Peaks were pushed down ${(expected - moved).toFixed(2)} dB beyond the level change`,
      )
    } else if (expected - moved > PEAK_TRACKING) {
      minor.push(
        `Peak came out ${(expected - moved).toFixed(2)} dB lower than the level change, from re-encoding`,
      )
    } else if (moved - expected > PEAK_TRACKING) {
      // Higher than predicted cannot be limiting - nothing in the chain raises
      // peaks. It is the encoder rebuilding the waveform, which low-bitrate
      // sources do by up to 0.8 dB. Reporting it as reshaping made untouched
      // tracks look altered.
      minor.push(
        `Peak came back ${(moved - expected).toFixed(2)} dB higher than the level change, from re-encoding`,
      )
    } else {
      unchanged.push('peak shape')
    }
  }

  // The null test: subtract the original and see how much is left. It says how
  // much of the file changed, not what about it changed - the dynamics and peak
  // checks above answer that - so a large leftover is reported as the cost of
  // rewriting rather than as damage to the music.
  if (has(a.nullResidual)) {
    const v = n(a.nullResidual)
    if (!nullFlagged) {
      minor.push(
        `Null test ${v.toFixed(1)} dB — only the cost of rewriting, inaudible`,
      )
    } else if (a.action === 'limited') {
      // Limiting reshapes the waveform, so a large leftover is what it looks
      // like when it worked - the same fact as the line above, not a second
      // problem. Reporting both put two warnings on a song with one cause.
      applied.push(
        `Null test ${v.toFixed(1)} dB — consistent with the peak capping above`,
      )
    } else {
      // Nothing capped the peaks and the range held, yet the file differs from
      // the original by more than re-encoding explains. That is worth a look.
      significant.push(
        `Null test ${v.toFixed(1)} dB — differs from the original by more than re-encoding explains`,
      )
    }
  }

  return {
    intended,
    applied,
    significant,
    minor,
    unchanged,
    // Clean means nothing is wrong. Work having been applied on purpose does
    // not make a song unclean - that was the whole confusion.
    clean: significant.length === 0,
  }
}
