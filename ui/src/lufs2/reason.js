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

import { AUDIBLE_SHAVE_DB, distanceTo, fmtLufs, fmtMag } from './recommendation'

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

// The causes a song can be listed under, and the filter values that select
// them. These are the ids the server's loudness_reason filter understands - the
// two sets have to stay identical, because a row offering to select "everything
// like this one" and then selecting something else is worse than not offering.
export const REASON_PEAK = 'peak_over'
export const REASON_FORMAT = 'format_changed'
export const REASON_MISSED = 'missed_target'
export const REASON_TRIMMED = 'peaks_trimmed'
export const REASON_TRADE = 'needs_trade'

// explainRefusal turns one engine rejection into a category, a headline and a
// sentence. A shape this file cannot read gets no category, so it is described
// but not offered as something to bulk-select: the server would not agree about
// what "like this one" meant.
export const explainRefusal = (error) => {
  const text = String(error || '')

  const both = BOTH.exec(text)
  if (both) {
    const [, got, want, peak, ceiling] = both.map(Number)
    return {
      // rejection() writes this when both checks failed. It opens "landed at",
      // so the server files it under missed_target and this must agree.
      id: REASON_MISSED,
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
      id: REASON_PEAK,
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
      id: REASON_MISSED,
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
      id: REASON_FORMAT,
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
    // Deliberately no id: the server has no category for an error shape this
    // file does not recognise, so there is nothing honest to select.
    id: null,
    headline: 'The rewrite was rejected',
    detail: text ? `${text}. ${INTACT}` : INTACT,
  }
}

// reasonFor answers "why is this song listed", for one row.
//
// The precedence is the load-bearing part and it is mirrored in
// loudnessReasonFilter in persistence/mediafile_repository.go: a refusal is
// reported as a refusal even when the row also carries a processed status, so a
// bulk action aimed at trimmed songs can never reach a refused one.
export const reasonFor = (record, rec) => {
  const a = record?.loudnessAudit
  if (a?.action === 'refused') return explainRefusal(a.error)

  if (a?.status === 'processed' && a?.action === 'limited') {
    const from = a.lufsBefore == null ? null : Number(a.lufsBefore)
    const to = a.lufsAfter == null ? null : Number(a.lufsAfter)
    return {
      id: REASON_TRIMMED,
      headline: 'Peaks trimmed - done',
      detail:
        from != null && to != null
          ? `${fmtLufs(from)} -> ${fmtLufs(to)} · restorable`
          : 'restorable',
      tone: 'ok',
    }
  }

  if (!rec) return null

  // The three cases below all carry REASON_TRADE, because the server defines
  // that category by exclusion - not refused, not already peak-trimmed - and
  // groups all three into it. The id is what "show me everything like this"
  // selects, so it has to match the server's grouping exactly; the wording is
  // what the row says about itself, and that can be honest per song. Giving the
  // two new cases no id instead left the dropdown returning rows whose headline
  // disagreed with the filter that had just fetched them.

  // A song inside the tolerance is finished, and saying anything else about it
  // is false. This used to fall through to the trade line below, which told a
  // client a song sitting exactly on -12.60 was "too quiet to fix" and "sits
  // 0.00 dB below the target" - a sentence that contradicts itself, on the row
  // most likely to be questioned. Reaching this at all now means the song is
  // here for some other reason: a stored decision, or the exception latch.
  if (rec.alreadyDone) {
    return {
      id: REASON_TRADE,
      headline: 'Already within tolerance',
      headlineTone: 'ok',
      detail:
        `This song is ${distanceTo(rec.lufs, rec.target)} and needs no correction. ` +
        `It is listed because a choice was recorded against it, not because anything is wrong with it.`,
      tone: 'ok',
    }
  }

  // The genuine trade: the level alone cannot reach the target, and the peak
  // cut that would is deep enough to hear.
  if (rec.peakOverBy > AUDIBLE_SHAVE_DB) {
    return {
      id: REASON_TRADE,
      headline: 'Too quiet to fix without a trade',
      detail:
        `This song is ${distanceTo(rec.lufs, rec.target)} and its peaks are already near the limit. ` +
        `Reaching ${fmtLufs(rec.target)} means cutting ${fmtMag(rec.peakOverBy)} dB off them, ` +
        `which is deep enough to hear on drums and transients.`,
    }
  }

  // Outside the tolerance, but no audible peak cut is needed to get there. The
  // engine handles this shape on its own, so a song reaching here is waiting on
  // something else - and guessing "too quiet to fix" about it would be a story
  // rather than a reason.
  return {
    id: REASON_TRADE,
    headline: 'Waiting for a decision',
    detail:
      `This song is ${distanceTo(rec.lufs, rec.target)}. ` +
      (rec.peakOverBy > 0.005
        ? `Reaching ${fmtLufs(rec.target)} means taking ${fmtMag(rec.peakOverBy)} dB off the peaks, which is too little to hear.`
        : `Reaching ${fmtLufs(rec.target)} is a volume change - nothing comes off the peaks.`),
  }
}

// What the filter dropdown offers. Same ids, same order the page tends to show
// them in.
export const REASON_CHOICES = [
  { id: REASON_PEAK, name: 'Re-encoding pushed the peaks back up' },
  // Named for the group rather than for one member of it. The server files
  // three different situations here - a genuine trade, a song already within
  // tolerance, and one waiting on a choice - so a label naming only the first
  // described two thirds of its own results wrongly.
  { id: REASON_TRADE, name: 'Needs a decision' },
  { id: REASON_FORMAT, name: 'The rewrite came back a different file' },
  { id: REASON_MISSED, name: 'The rewrite did not land on the target' },
  { id: REASON_TRIMMED, name: 'Peaks trimmed - done' },
]
