// Why a song is on this page, said in words rather than in engine output.
//
// "Could not be processed" is not a reason, it is a refusal to give one - and
// on this page it was shown in red beside an error string written for whoever
// wrote the optimiser. A client reading "true peak -0.08 dBTP exceeds the -0.50
// dBTP ceiling" learns nothing except that something went wrong, which is the
// one impression the row should not leave: in every case here the attempt was
// thrown away and the original file was never touched.
//
// The strings parsed below are produced by rejection() and IntegrityIssues() in
// core/loudness. They are matched loosely and always fall back to the raw text,
// so an error shape this file has not seen degrades to what the page showed
// before rather than to a blank cell.

import { fmtLufs, fmtMag } from './recommendation'

const PEAK_ONLY =
  /true peak (-?[\d.]+) dBTP exceeds the (-?[\d.]+) dBTP ceiling/
const BOTH =
  /landed at (-?[\d.]+) LUFS \(expected (-?[\d.]+)\) and (-?[\d.]+) dBTP \(ceiling (-?[\d.]+)\)/
const LOUDNESS_ONLY = /landed at (-?[\d.]+) LUFS, expected (-?[\d.]+)/
const FORMAT = /output was not format-identical: \[(.*)\]/

// Go prints a []string as [a b c], so the elements cannot be split apart again
// once they contain spaces - which all of these do. Detected by substring
// instead, which is why each phrase below is one the engine never varies.
const FORMAT_ISSUES = [
  [/duration changed/, 'its length came out different'],
  [/cover art lost/, 'the cover art was lost'],
  [/codec \S+->\S+/, 'the audio format changed'],
  [/bitrate \d+k->\d+k/, 'the bitrate dropped'],
  [/rate \d+->\d+/, 'the sample rate changed'],
  [/depth \d+->\d+/, 'the bit depth changed'],
  [/channels \d+->\d+/, 'the channel count changed'],
]

// Said on every refusal, because it is the part that decides whether this row
// is alarming. Nothing was lost: the rewrite is what failed, and it was
// discarded before it could replace anything.
const INTACT = 'Your original file is untouched.'

const list = (parts) =>
  parts.length <= 1
    ? parts.join('')
    : `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`

// explainRefusal turns one engine rejection into a headline and a sentence.
export const explainRefusal = (error) => {
  const text = String(error || '')

  const both = BOTH.exec(text)
  if (both) {
    const [, got, want, peak, ceiling] = both.map(Number)
    return {
      headline: 'The rewrite missed on loudness and peaks',
      detail:
        `The re-encoded copy came out at ${fmtLufs(got)} LUFS instead of ${fmtLufs(want)}, ` +
        `and its peaks reached ${fmtLufs(peak)} dBTP against a ${fmtLufs(ceiling)} limit. ` +
        INTACT,
    }
  }

  const peak = PEAK_ONLY.exec(text)
  if (peak) {
    const got = Number(peak[1])
    const ceiling = Number(peak[2])
    const over = got - ceiling
    return {
      headline: 'Re-encoding pushed the peaks back up',
      detail:
        `Turning the song up worked, but writing it back to MP3 lifted the peaks to ${fmtLufs(got)} dBTP, ` +
        `${fmtMag(over)} dB above the ${fmtLufs(ceiling)} safety line. ` +
        INTACT,
    }
  }

  const loudness = LOUDNESS_ONLY.exec(text)
  if (loudness) {
    const got = Number(loudness[1])
    const want = Number(loudness[2])
    return {
      headline: 'The rewrite did not land on the target',
      detail:
        `The re-encoded copy measured ${fmtLufs(got)} LUFS instead of ${fmtLufs(want)}, ` +
        `${fmtMag(got - want)} dB out. ` +
        INTACT,
    }
  }

  const format = FORMAT.exec(text)
  if (format) {
    const found = FORMAT_ISSUES.filter(([re]) => re.test(format[1])).map(
      ([, said]) => said,
    )
    return {
      headline: 'The rewrite came back a different file',
      detail:
        (found.length
          ? `The re-encoded copy was not the same file as the original - ${list(found)}. `
          : `The re-encoded copy was not the same file as the original (${format[1]}). `) +
        'This usually means the source has a damaged or missing header. ' +
        INTACT,
    }
  }

  return {
    headline: 'The rewrite was rejected',
    detail: text ? `${text}. ${INTACT}` : INTACT,
  }
}
