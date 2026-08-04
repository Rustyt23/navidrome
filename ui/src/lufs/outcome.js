// Did it work?
//
// The verdict answers a different question - was the file harmed - and answers
// it well. What it cannot say is whether the song ended up where the client
// asked for it, and that is the question the whole exercise exists to settle. A
// track that landed exactly on -12.6 and one that stopped 4 dB short both read
// "Volume only", because neither was harmed. Both statements are true and only
// one of them is what someone came to the page to find out.
//
// Derived rather than stored. Every input is already in the record, so a column
// would only be a slower copy of this arithmetic that could fall out of step
// with it.

const has = (v) => v !== null && v !== undefined && !Number.isNaN(Number(v))

export const OUTCOME_ON_TARGET = 'on_target'
export const OUTCOME_SHORT_BY_CHOICE = 'short_by_choice'
export const OUTCOME_SHORT_SOURCE = 'short_source_limited'
export const OUTCOME_NOT_ATTEMPTED = 'not_attempted'
export const OUTCOME_LEVEL_TWO = 'level_two'

// Mirrors PhaseCloseEnough in core/loudness.
const PHASE_CLOSE_ENOUGH = 4
// Mirrors leaveAloneToleranceDB in core/loudness.
const LEVEL_TWO_TOLERANCE = 0.5

// isLevelTwo: held to the wider tolerance rather than corrected.
//
// Two ways in, and they have to be read together or half of them show up as
// something else. The planner can decide in advance not to ask about a track;
// or it can try, fail to ship a result, and the track turns out to have been
// close enough that the failure was not worth reporting. Both end up untouched
// and near target, which is one state, not two.
export const isLevelTwo = (audit, settings) => {
  if (!audit) return false
  const target = settings?.targetLUFS ?? -12.6
  const tolerance = settings?.tolerance ?? 0.2
  const now = has(audit.lufsAfter)
    ? Number(audit.lufsAfter)
    : Number(audit.lufsBefore)
  if (!has(now)) return false

  const offBy = Math.abs(now - target)
  if (offBy <= tolerance || offBy > LEVEL_TWO_TOLERANCE) return false
  return audit.phase === PHASE_CLOSE_ENOUGH || audit.action === 'refused'
}

// outcomeFor reports where a song ended up relative to the target.
//
// The loudness it reads is whatever the song measures now: lufsAfter once it
// has been rewritten, lufsBefore while it has only been measured. A song that
// was already on target and never opened is on target - it does not need to
// have been processed to count.
export const outcomeFor = (record, settings) => {
  const a = record?.loudnessAudit
  if (!a) return null

  const target = settings?.targetLUFS ?? -12.6
  const tolerance = settings?.tolerance ?? 0.2

  const now = has(a.lufsAfter) ? Number(a.lufsAfter) : Number(a.lufsBefore)
  if (!has(now)) return null

  const offBy = Math.abs(now - target)
  // Reported to two places against a tolerance held to one: a song 0.249 from
  // target would otherwise show "0.25 off" beside a claim that it is within
  // 0.2, which reads as the page contradicting itself.
  const detail = `${offBy.toFixed(2)} dB from ${target.toFixed(2)}`

  const base = { offBy, now }

  if (offBy <= tolerance) {
    return { ...base, id: OUTCOME_ON_TARGET, tone: 'good', detail }
  }
  if (isLevelTwo(a, settings)) {
    return {
      ...base,
      id: OUTCOME_LEVEL_TWO,
      tone: 'info',
      detail: `${detail} - close enough that correcting it was not worth a re-encode`,
    }
  }
  if (a.decision === 'gain_ceiling') {
    return {
      ...base,
      id: OUTCOME_SHORT_BY_CHOICE,
      tone: 'neutral',
      detail: `${detail} - lifted as far as it could go without altering it`,
    }
  }
  if (a.status === 'processed') {
    return {
      ...base,
      id: OUTCOME_SHORT_SOURCE,
      tone: 'warn',
      detail: `${detail} - the source had no more headroom`,
    }
  }
  return { ...base, id: OUTCOME_NOT_ATTEMPTED, tone: 'neutral', detail }
}
