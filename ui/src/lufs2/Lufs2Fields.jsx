import React, { useCallback, useState } from 'react'
import { useListContext, useRecordContext, useTranslate } from 'react-admin'
import { Chip, Collapse, Link, Tooltip } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import {
  DECISION_CEILING,
  DECISION_LIMIT,
  DECISION_SKIP,
  fmtDb,
  fmtLufs,
  fmtMag,
  optionsFor,
  recommendationFor,
} from './recommendation'
import { reasonFor } from './reason'

const useStyles = makeStyles((theme) => ({
  muted: { color: theme.palette.text.secondary },
  bad: { color: '#ef5350', fontWeight: 600 },
  ok: { color: '#66bb6a' },
  warn: { color: '#ffa726' },
  nowrap: { whiteSpace: 'nowrap' },
  chip: { height: 20, fontSize: '0.72rem', fontWeight: 600 },
  pending: { backgroundColor: theme.palette.action.selected },
  limit: { backgroundColor: '#ed6c02', color: '#fff' },
  ceiling: { backgroundColor: '#2e7d32', color: '#fff' },
  skip: { backgroundColor: '#546e7a', color: '#fff' },
  sub: { fontSize: '0.72rem', display: 'block' },
  // The reason wraps now. It used to be nowrap because it was an error string
  // nobody could act on anyway; a sentence written to be read has to be
  // allowed to be read.
  wrap: { display: 'block', whiteSpace: 'normal', maxWidth: 320 },
  cell: { display: 'block', maxWidth: 340 },
  more: {
    fontSize: '0.72rem',
    cursor: 'pointer',
    display: 'inline-block',
    marginTop: 2,
  },
  detail: {
    fontSize: '0.72rem',
    lineHeight: 1.5,
    marginTop: theme.spacing(0.75),
    paddingLeft: theme.spacing(1),
    borderLeft: `2px solid ${theme.palette.divider}`,
    whiteSpace: 'normal',
  },
  detailRow: { marginBottom: 3 },
  // Underlined on hover only: eight underlined headlines at once would read as
  // a page of warnings rather than a page of links.
  reasonLink: {
    fontWeight: 600,
    textAlign: 'left',
    textDecoration: 'none',
    '&:hover': { textDecoration: 'underline' },
  },
  detailKey: { fontWeight: 700 },
}))

// Why this track cannot simply be turned up.
export const HeadroomField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  return (
    <Tooltip
      // Spelled out in full because the two corrections are exactly what makes
      // the plain arithmetic wrong: part of the gain is spent replacing what
      // re-encoding costs, and the encoder puts some of the peak back after
      // the gain has been applied. A tooltip that named only the distance to
      // target described a file the engine would never produce.
      title={
        `Reaching ${fmtLufs(rec.target)} needs ${fmtDb(rec.gainToReachTarget)} dB` +
        (rec.rewriteCost > 0
          ? ` (${fmtDb(rec.gainToTarget)} to the target, plus ${rec.rewriteCost.toFixed(2)} lost to re-encoding at ${rec.bitRate}k)`
          : '') +
        `. That puts the peaks at ${fmtLufs(rec.predictedPeak)} dBTP` +
        (rec.springBack > 0
          ? `, and ${rec.springBack.toFixed(2)} dB of encoder spring-back takes them to ${fmtLufs(rec.predictedPeakEncoded)}`
          : '') +
        ` - ${fmtDb(rec.peakOverBy)} dB above the ${fmtLufs(rec.ceiling, 1)} ceiling.`
      }
    >
      <span className={`${classes.nowrap} ${classes.bad}`}>
        {`${fmtDb(rec.peakOverBy)} dB over`}
      </span>
    </Tooltip>
  )
}

export const CurrentField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  return (
    <span className={classes.nowrap}>
      {`${fmtLufs(rec.lufs)} LUFS`}
      <span className={`${classes.sub} ${classes.muted}`}>
        {`peak ${fmtLufs(rec.peak)} dBTP`}
      </span>
    </span>
  )
}

// Reach the target by shaving the peaks.
export const OptionLimitField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  return (
    <span className={classes.nowrap}>
      <span className={classes.ok}>{`${fmtLufs(rec.target)} LUFS`}</span>
      <span className={`${classes.sub} ${classes.warn}`}>
        {`${fmtDb(rec.peakOverBy)} dB of limiting`}
      </span>
    </span>
  )
}

// Leave the audio untouched and accept a quieter result. This always lands
// short of the target - that is what put the track on this page - so it is
// never shown as a clean outcome.
export const OptionCeilingField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  return (
    <span className={classes.nowrap}>
      <span className={classes.warn}>
        {`${fmtLufs(rec.loudnessAtCeiling)} LUFS`}
      </span>
      <span className={`${classes.sub} ${classes.muted}`}>
        {`${fmtDb(-rec.shortfall)} dB quieter`}
      </span>
    </span>
  )
}

// Where the suggested option actually leaves the song.
//
// This used to be spread over four columns - the headroom shortfall and the
// two options and the best volume-only result - which between them restated
// the same arithmetic four ways and made a page of eight rows look like an
// audit. Everything anyone acts on is the landing loudness and what it costs,
// so that is what the row says; the four columns still exist behind the column
// picker for whoever wants to check the working.
const landing = (rec, decision = rec.suggested) => {
  const outcome = optionsFor(rec)[decision]
  if (!outcome) return `${fmtLufs(rec.lufs)} LUFS · left as it is`
  return `${outcome.lands} · ${outcome.short}`
}

const DECISION_KEY = {
  [DECISION_LIMIT]: 'resources.lufs2.decision.limit',
  [DECISION_CEILING]: 'resources.lufs2.decision.gain_ceiling',
  [DECISION_SKIP]: 'resources.lufs2.decision.skip',
}

// SuggestionField: the recommendation, and on request the case for it.
//
// A suggestion with no reasoning has to be taken on trust, and a page of them
// is approved or not approved as a block - which is the wrong unit, because
// these rows differ from each other. But the reasoning is three or four
// sentences and eight rows of it is a wall nobody reads either. So it is folded
// away: the row stays one line, and the argument is one click from anyone who
// wants to see it.
//
// The alternatives are in there too. What a recommendation is worth depends
// entirely on what it was chosen over, and "hits the target" means nothing
// until you can see that the other option stops 0.66 dB short.
export const SuggestionField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const [open, setOpen] = useState(false)
  // The row underneath navigates on click, so the toggle has to keep its own
  // click to itself.
  const toggle = useCallback((e) => {
    e.stopPropagation()
    e.preventDefault()
    setOpen((wasOpen) => !wasOpen)
  }, [])

  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>

  const options = optionsFor(rec)
  const chosen = options[rec.suggested]
  const others = [DECISION_LIMIT, DECISION_CEILING, DECISION_SKIP].filter(
    (d) => d !== rec.suggested,
  )
  const name = (d) => translate(DECISION_KEY[d], { _: d })

  return (
    <span className={classes.cell}>
      <span className={classes.nowrap}>
        {translate(
          DECISION_KEY[rec.suggested] || DECISION_KEY[DECISION_LIMIT],
          {
            _: rec.suggested,
          },
        )}
      </span>
      <span className={`${classes.sub} ${classes.muted}`}>{landing(rec)}</span>
      <Link
        component="button"
        type="button"
        onClick={toggle}
        className={classes.more}
        aria-expanded={open}
      >
        {open ? 'Hide explanation' : 'Why this?'}
      </Link>
      <Collapse in={open} timeout="auto" unmountOnExit>
        <div className={classes.detail}>
          <div className={classes.detailRow}>
            <span className={classes.detailKey}>Why: </span>
            {rec.suggestedBecause}.
          </div>
          {chosen && (
            <div className={classes.detailRow}>
              <span className={classes.detailKey}>What you get: </span>
              {chosen.sole
                ? `lands at ${chosen.lands} - ${chosen.sole}.`
                : `lands at ${chosen.lands} - ${chosen.gain}. The trade is that ${chosen.cost}.`}
            </div>
          )}
          {others.map((d) => (
            <div key={d} className={classes.detailRow}>
              <span
                className={classes.detailKey}
              >{`${name(d)} instead: `}</span>
              {options[d].sole
                ? `${options[d].lands} - ${options[d].sole}.`
                : `${options[d].lands} - ${options[d].gain}, but ${options[d].cost}.`}
            </div>
          ))}
        </div>
      </Collapse>
    </span>
  )
}

// The closest a volume-only change can bring this song, and whether it is worth
// doing. Answers the question the numbers on this page raise but never settle:
// if the audio must not be touched, how near can we actually get?
//
// For many degraded sources the honest answer is "no nearer than it already
// is", because rewriting the file costs more loudness than the peaks leave room
// to add. Saying so plainly is the point of the column - otherwise every row
// looks like outstanding work.
export const BestWithoutDistortionField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec?.best) return <span className={classes.muted}>-</span>

  const { best } = rec
  const tone = best.onTarget
    ? classes.ok
    : best.worthDoing
      ? classes.warn
      : classes.muted

  const note = best.onTarget
    ? 'reaches the target with volume alone'
    : best.worthDoing
      ? `${fmtDb(best.gains)} dB closer than it is now`
      : 'no closer than leaving it alone'

  return (
    <Tooltip
      title={
        best.estimated
          ? 'Estimated. Rewriting a low-bitrate file costs loudness and can lift its peaks unpredictably, so the real result may fall a little short.'
          : 'On a source this clean the peak follows the gain exactly, so this is what it will do.'
      }
    >
      <span className={classes.nowrap}>
        <span className={tone}>{`${fmtLufs(best.lufs)} LUFS`}</span>
        <span className={`${classes.sub} ${classes.muted}`}>
          {`${fmtDb(-best.offBy)} off · ${note}`}
        </span>
      </span>
    </Tooltip>
  )
}

// Why this song is on this page at all, and a way to see everything else here
// for the same reason.
//
// Eight rows with five distinct causes between them is five decisions, not
// eight, but only if the causes can be separated. Clicking the headline sets
// the server-side loudness_reason filter, so the selection that follows covers
// every song with that cause across the whole library rather than the ones that
// happen to be on this page.
export const ReasonField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const { setFilters, filterValues, displayedFilters } = useListContext(props)
  const rec = recommendationFor(record, props.settings)
  const reason = reasonFor(record, rec)

  const showOnlyThis = useCallback(
    (e) => {
      e.stopPropagation()
      e.preventDefault()
      // displayedFilters passed through rather than dropped: it is what keeps
      // the filter chips someone has already opened on screen. Clearing it
      // would fold away the Decision filter mid-review.
      setFilters(
        { ...filterValues, loudness_reason: reason?.id },
        displayedFilters,
      )
    },
    [setFilters, filterValues, displayedFilters, reason?.id],
  )

  if (!reason) return <span className={classes.muted}>-</span>

  // Amber for a refusal, not red. It means the engine produced a file, judged
  // it not good enough and threw it away - the song on disk is exactly as it
  // was. Red says damage was done, which is the opposite of what happened, and
  // on a page shown to a client that reads as a fault to answer for.
  const tone = reason.tone === 'ok' ? classes.ok : classes.warn
  const alreadyFiltered = filterValues?.loudness_reason === reason.id

  return (
    // The raw engine text stays reachable, because someone eventually has to
    // debug one of these and the sentence above is deliberately not it.
    <Tooltip title={record?.loudnessAudit?.error || ''}>
      <span className={classes.wrap}>
        {reason.id && !alreadyFiltered ? (
          <Link
            component="button"
            type="button"
            onClick={showOnlyThis}
            className={`${classes.reasonLink} ${tone}`}
            title="Show every song listed for this reason"
          >
            {reason.headline}
          </Link>
        ) : (
          <span className={tone}>{reason.headline}</span>
        )}
        <span className={`${classes.sub} ${classes.muted}`}>
          {reason.detail}
        </span>
      </span>
    </Tooltip>
  )
}

export const DecisionField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = record?.loudnessAudit
  const decision = a?.decision || ''
  const styleFor = {
    [DECISION_LIMIT]: classes.limit,
    [DECISION_CEILING]: classes.ceiling,
    [DECISION_SKIP]: classes.skip,
  }
  const chip = (
    <Chip
      size="small"
      className={`${classes.chip} ${styleFor[decision] || classes.pending}`}
      label={translate(
        decision
          ? `resources.lufs2.decision.${decision}`
          : 'resources.lufs2.decision.pending',
        { _: decision || 'Undecided' },
      )}
    />
  )

  const rec = recommendationFor(record, props.settings)
  // Applied, and the file on disk is the result of it. Measured rather than
  // predicted: once the work is done the estimate is no longer the interesting
  // number, and showing it beside a file it no longer describes is how a page
  // starts lying.
  //
  // A restored song is excluded: its audit still carries the measurements from
  // the run, but the file underneath is the original again, so those figures
  // describe audio that is no longer there.
  const applied =
    !a?.restoredAt && a?.lufsAfter != null ? Number(a.lufsAfter) : null

  if (applied != null) {
    const target = rec?.target ?? -12.6
    const tolerance = props.settings?.tolerance ?? 0.2
    const onTarget = Math.abs(applied - target) <= tolerance
    return (
      <span className={classes.cell}>
        <span className={classes.nowrap}>
          <span
            className={onTarget ? classes.ok : classes.warn}
          >{`${fmtLufs(applied)} LUFS`}</span>
        </span>
        <span className={`${classes.sub} ${classes.muted}`}>
          {(a.tpAfter != null
            ? `peak ${fmtLufs(Number(a.tpAfter))} dBTP · `
            : '') +
            (onTarget
              ? 'done, on target'
              : `done, ${fmtMag(applied - target)} dB ${applied > target ? 'louder than' : 'below'} ${fmtLufs(target)}`)}
        </span>
        {chip}
      </span>
    )
  }

  // Chosen but not run yet, so the honest thing to show is what it will do -
  // flagged as a forecast, because a bare number here is indistinguishable from
  // a measurement and this one has not happened.
  if (decision && rec) {
    return (
      <span className={classes.cell}>
        {chip}
        <span className={`${classes.sub} ${classes.muted}`}>
          {`expected: ${landing(rec, decision)}`}
        </span>
      </span>
    )
  }

  return (
    <span className={classes.cell}>
      {chip}
      <span className={`${classes.sub} ${classes.muted}`}>
        nothing applied yet
      </span>
    </span>
  )
}
