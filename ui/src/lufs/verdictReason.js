// Why a song carries the verdict it carries.
//
// The verdicts are short by design - "Volume only", "Levelled + peaks capped" -
// and short labels are what makes a wide table readable. What they cannot do is
// say why, and nothing else on the page said it either: a tooltip existed only
// on rows that had gone wrong, so every song that worked offered no explanation
// at all.
//
// Built from the record rather than written per verdict, so the sentence quotes
// the song's own numbers. A generic explanation of what "Levelled + peaks
// capped" means is a glossary; "reached -12.60 by trimming 0.71 dB off the
// peaks" is an answer.

const has = (v) => v !== null && v !== undefined && !Number.isNaN(Number(v))
const f2 = (v) => Number(v).toFixed(2)

// How far the peaks moved relative to the level, which is what separates a
// plain volume change from one where the peaks were held back.
const peakShave = (a) => {
  if (!has(a?.tpBefore) || !has(a?.tpAfter) || !has(a?.gainApplied)) return null
  return Number(a.tpBefore) + Number(a.gainApplied) - Number(a.tpAfter)
}

const movement = (a) =>
  has(a?.lufsBefore) && has(a?.lufsAfter)
    ? `${f2(a.lufsBefore)} to ${f2(a.lufsAfter)} LUFS`
    : null

export const verdictReason = (audit, settings, verdict) => {
  const a = audit
  if (!a) return ''
  const target = settings?.targetLUFS ?? -12.6
  const tolerance = settings?.tolerance ?? 0.2
  const moved = movement(a)

  switch (verdict) {
    case 'needs_decision':
      return (
        'Reaching the target would mean a trade nobody should make on your behalf - ' +
        'either altering the audio or accepting a quieter result. ' +
        'It is waiting for you on the Exception LUFS page. Nothing has been changed.'
      )

    case 'untouched':
      return has(a.lufsBefore)
        ? `Measured at ${f2(a.lufsBefore)} LUFS, already within ${tolerance} dB of ${f2(target)}. The file was never opened.`
        : 'Already within tolerance of the target, so the file was never opened.'

    case 'level_two':
      return has(a.lufsBefore)
        ? `At ${f2(a.lufsBefore)} LUFS it sits within half a decibel of ${f2(target)}. ` +
            'Correcting that would cost a full re-encode for a difference nobody can hear, so it was left alone.'
        : 'Within half a decibel of the target - close enough that correcting it was not worth a re-encode.'

    case 'safe': {
      const shave = peakShave(a)
      const base = moved
        ? `Level changed from ${moved}.`
        : 'The level was changed.'
      return (
        `${base} The waveform was not reshaped - every sample moved by the same amount, ` +
        (shave !== null && shave < 0.15
          ? 'and the peaks followed it exactly.'
          : 'so nothing about the audio changed except how loud it is.')
      )
    }

    case 'dynamics_changed': {
      const shave = peakShave(a)
      return (
        (moved ? `Reached ${moved}` : 'Reached the target') +
        (shave !== null
          ? ` by turning it up and holding the loudest peaks back by ${f2(shave)} dB.`
          : ' by turning it up and holding the loudest peaks back.') +
        ' A peak is a few samples at the tip of one transient, so a cut this small is not audible - ' +
        'but the peaks were touched, which is why this is not filed as a plain level change.'
      )
    }

    case 'rewrite_costly':
      return (
        'The format survived intact and nothing reshaped the audio, but comparing the result against ' +
        'the stored original left more difference than a clean re-encode of this bitrate should. ' +
        'Most likely a source that was already heavily compressed. The file is on target and restorable.'
      )

    case 'reencoded':
      return (
        'The finished file came back in a worse format than it went in - codec, bitrate, sample rate, ' +
        'bit depth, channels or artwork. That is quality lost for good, so it is flagged rather than accepted quietly.'
      )

    case 'no_audio':
      return (
        'There is no playable audio in this file at all - normally a download that was truncated or ' +
        'never finished. Re-running will not help; the song needs fetching again from its source.'
      )

    case 'failed':
      return a.error
        ? `The engine could not get through this file. It reported: ${a.error}`
        : 'The engine could not get through this file, and recorded no reason.'

    case 'left_as_is':
      return a.error
        ? `A corrected file was built, judged not good enough to ship, and thrown away - your original is untouched. The engine reported: ${a.error}`
        : 'A corrected file was built, judged not good enough to ship, and thrown away. Your original is untouched.'

    default:
      return ''
  }
}
