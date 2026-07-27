import React from 'react'
import { useRecordContext, useTranslate } from 'react-admin'
import { Chip, Tooltip } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import {
  DECISION_CEILING,
  DECISION_LIMIT,
  DECISION_SKIP,
  fmtDb,
  fmtLufs,
  recommendationFor,
} from './recommendation'

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
}))

// Why this track cannot simply be turned up.
export const HeadroomField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  return (
    <Tooltip
      title={`Turning it up by ${fmtDb(rec.gainToTarget)} dB would put the peaks at ${fmtLufs(rec.predictedPeak)} dBTP, ${fmtDb(rec.peakOverBy)} dB above the ${fmtLufs(rec.ceiling, 1)} ceiling`}
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

// Option A: reach the target by shaving the peaks.
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

// Option B: leave the audio untouched and accept a quieter result.
export const OptionCeilingField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  return (
    <span className={classes.nowrap}>
      <span className={rec.shortfall <= 1 ? classes.ok : classes.warn}>
        {`${fmtLufs(rec.loudnessAtCeiling)} LUFS`}
      </span>
      <span className={`${classes.sub} ${classes.muted}`}>
        {`${fmtDb(-rec.shortfall)} dB quieter`}
      </span>
    </span>
  )
}

export const SuggestionField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const rec = recommendationFor(record, props.settings)
  if (!rec) return <span className={classes.muted}>-</span>
  const key =
    rec.suggested === DECISION_CEILING
      ? 'resources.lufs2.decision.gain_ceiling'
      : 'resources.lufs2.decision.limit'
  return (
    <span className={classes.muted}>
      {translate(key, { _: rec.suggested })}
    </span>
  )
}

export const DecisionField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const decision = record?.loudnessAudit?.decision || ''
  const styleFor = {
    [DECISION_LIMIT]: classes.limit,
    [DECISION_CEILING]: classes.ceiling,
    [DECISION_SKIP]: classes.skip,
  }
  return (
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
}
