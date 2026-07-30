// What actually happened to a song, and whether any of it matters.
//
// The split is not a matter of taste. A change is significant when it alters
// what the listener receives or permanently costs quality: less data per
// second, a different sample rate, reshaped dynamics, lost artwork, a shifted
// duration. It is minor when it is an unavoidable and inaudible side effect of
// rewriting a lossy file, or a measurement moving within its own noise.
//
// The loudness change itself is never a fault: it is the job.

const has = (v) => v !== null && v !== undefined && !Number.isNaN(Number(v))
const n = (v) => Number(v)
const f2 = (v) => n(v).toFixed(2)
const signed = (v) => `${n(v) > 0 ? '+' : ''}${n(v).toFixed(2)}`

// Thresholds, each tied to a physical reason rather than a round number.
const LRA_NOISE = 0.5 // loudness range moves this much on measurement noise alone
const PEAK_TRACKING = 0.15 // a pure gain moves the peak by exactly the gain
const DURATION_AUDIBLE = 0.05 // beyond this a gapless album develops a seam
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
export const nullFloorFor = (codec) =>
  LOSSLESS.includes(String(codec || '').toLowerCase()) ? -60 : -30

export const reportFor = (record, settings) => {
  const a = record?.loudnessAudit
  if (!a || a.status !== 'processed') return null

  const ceiling = settings?.truePeak ?? -0.5
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
    if (abs > DURATION_AUDIBLE) {
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
  if (a.action === 'limited') {
    significant.push(
      'Peak limiting applied — the loudest moments were reshaped',
    )
  }

  if (has(a.lraBefore) && has(a.lraAfter)) {
    const d = n(a.lraAfter) - n(a.lraBefore)
    if (Math.abs(d) > LRA_NOISE) {
      significant.push(
        `Dynamics ${d < 0 ? 'compressed' : 'widened'}: loudness range ${f2(a.lraBefore)} → ${f2(a.lraAfter)} LU`,
      )
    } else if (Math.abs(d) > 0.05) {
      minor.push(`Loudness range moved ${signed(d)} LU (measurement noise)`)
    } else {
      unchanged.push('dynamics')
    }
  }

  // Under a pure gain the true peak moves by exactly the gain. A deviation
  // means something other than the level was altered.
  if (has(a.tpBefore) && has(a.tpAfter)) {
    const moved = n(a.tpAfter) - n(a.tpBefore)
    const expected = has(a.gainApplied) ? n(a.gainApplied) : moved
    if (n(a.tpAfter) > 0) {
      significant.push(
        `True peak ${f2(a.tpAfter)} dBTP — above 0, the file can clip`,
      )
    } else if (n(a.tpAfter) > FALLBACK_CEILING + 0.1) {
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
    } else if (expected - moved > PEAK_TRACKING) {
      // The peak came out lower than the level change alone would put it, so
      // something pushed it down. That is what limiting does.
      significant.push(
        `Peaks were pushed down ${(expected - moved).toFixed(2)} dB beyond the level change`,
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
    const floor = nullFloorFor(codecAfter || a.codecBefore)
    const v = n(a.nullResidual)
    if (v > floor) {
      significant.push(
        `Null test ${v.toFixed(1)} dB — rewriting the file cost more than it should`,
      )
    } else {
      minor.push(
        `Null test ${v.toFixed(1)} dB — only the cost of rewriting, inaudible`,
      )
    }
  }

  return {
    intended,
    significant,
    minor,
    unchanged,
    clean: significant.length === 0,
  }
}
