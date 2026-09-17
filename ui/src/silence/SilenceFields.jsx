import React from 'react'
import PropTypes from 'prop-types'
import { Chip, Tooltip, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import {
  formatBytes,
  formatPeakDb,
  formatSeconds,
  METHOD_LABELS,
  SILENT_PEAK_DB,
  skipReasonLabel,
  VERDICT_LABELS,
} from './format'

const useStyles = makeStyles((theme) => ({
  chip: { height: 22, fontSize: '0.72rem' },
  trimmable: { backgroundColor: '#2e7d32', color: '#fff' },
  clean: { backgroundColor: theme.palette.action.selected },
  skipped: { backgroundColor: '#ed6c02', color: '#fff' },
  failed: { backgroundColor: '#c62828', color: '#fff' },
  // The headline number. Bigger and bolder than everything else on the row
  // because it is the one thing the page exists to answer.
  total: { fontWeight: 700, fontVariantNumeric: 'tabular-nums' },
  totalZero: { color: theme.palette.text.disabled, fontVariantNumeric: 'tabular-nums' },
  ends: {
    display: 'flex',
    gap: theme.spacing(0.5),
    alignItems: 'baseline',
    whiteSpace: 'nowrap',
    fontVariantNumeric: 'tabular-nums',
  },
  endLabel: { color: theme.palette.text.secondary, fontSize: '0.68rem' },
  muted: { color: theme.palette.text.secondary },
  lossless: { color: '#2e7d32', fontWeight: 600 },
  reencoded: { color: '#ed6c02', fontWeight: 600 },
  done: { color: '#2e7d32' },
}))

const auditOf = (record) => record?.silenceAudit

// TotalTrimField is the page's headline: how many seconds come off this song.
//
// Shown as a single number rather than as head+tail, because "how much shorter
// will this song get" is the question, and two numbers make the reader do the
// addition. The breakdown is one column over for anyone who wants it.
export const TotalTrimField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>-</span>

  const total = (audit.leadTrim || 0) + (audit.trailTrim || 0)
  if (total <= 0) {
    // A song with silence that is deliberately being left alone reads very
    // differently from one with no silence at all, so it does not simply show
    // a dash.
    const found = (audit.leadSilence || 0) + (audit.trailSilence || 0)
    if (audit.verdict === 'skipped' && found > 0) {
      return (
        <Tooltip title={`${formatSeconds(found)} of silence found, none removed`}>
          <span className={classes.totalZero}>none</span>
        </Tooltip>
      )
    }
    return <span className={classes.totalZero}>0s</span>
  }

  const detail = [
    audit.leadTrim > 0 ? `${formatSeconds(audit.leadTrim)} from the start` : null,
    audit.trailTrim > 0 ? `${formatSeconds(audit.trailTrim)} from the end` : null,
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <Tooltip title={detail}>
      <span className={classes.total}>{formatSeconds(total)}</span>
    </Tooltip>
  )
}

TotalTrimField.propTypes = { record: PropTypes.object }
TotalTrimField.defaultProps = { addLabel: true }

// SilenceFoundField shows what the detector actually measured at each end,
// before any decision was taken. Kept next to the trim so the margin is
// visible as the difference between the two.
export const SilenceFoundField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>-</span>
  const lead = audit.leadSilence || 0
  const trail = audit.trailSilence || 0
  if (lead === 0 && trail === 0) return <span className={classes.muted}>none</span>
  return (
    <Tooltip title="Silence measured at the start and end, before the margin is kept back">
      <span className={classes.ends}>
        <span className={classes.endLabel}>start</span>
        {formatSeconds(lead)}
        <span className={classes.endLabel}>end</span>
        {formatSeconds(trail)}
      </span>
    </Tooltip>
  )
}

SilenceFoundField.propTypes = { record: PropTypes.object }
SilenceFoundField.defaultProps = { addLabel: true }

// TrimBreakdownField is the same split for what will actually be removed.
export const TrimBreakdownField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>-</span>
  const lead = audit.leadTrim || 0
  const trail = audit.trailTrim || 0
  if (lead === 0 && trail === 0) return <span className={classes.muted}>-</span>
  return (
    <span className={classes.ends}>
      <span className={classes.endLabel}>start</span>
      {formatSeconds(lead)}
      <span className={classes.endLabel}>end</span>
      {formatSeconds(trail)}
    </span>
  )
}

TrimBreakdownField.propTypes = { record: PropTypes.object }
TrimBreakdownField.defaultProps = { addLabel: true }

// VerdictField is what the analysis concluded, and for a skipped song, why.
//
// The reason is on the chip rather than in a tooltip: "left alone" with no
// explanation is the state that generates questions, and the answer is short
// enough to fit.
export const SilenceVerdictField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>not analysed</span>

  const label =
    audit.verdict === 'skipped' && audit.skipReason
      ? skipReasonLabel(audit.skipReason)
      : VERDICT_LABELS[audit.verdict] || audit.verdict

  const tooltip =
    audit.verdict === 'failed' && audit.error
      ? audit.error
      : audit.verdict === 'skipped'
        ? 'Silence was found but removing it would damage the song'
        : ''

  return (
    <Tooltip title={tooltip}>
      <Chip
        size="small"
        label={label}
        className={`${classes.chip} ${classes[audit.verdict] || ''}`}
      />
    </Tooltip>
  )
}

SilenceVerdictField.propTypes = { record: PropTypes.object }
SilenceVerdictField.defaultProps = { addLabel: true }

// MethodField says whether the song keeps its exact bits or gets re-encoded.
//
// Worth a column of its own: on a lossy format a re-encode costs a generation
// of quality, and the client should be able to see at a glance that almost
// nothing in an MP3 library pays that cost.
export const SilenceMethodField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit?.method) return <span className={classes.muted}>-</span>
  const lossless = audit.method === 'copy'
  return (
    <Tooltip
      title={
        lossless
          ? 'Cut without decoding - the audio that remains is bit-for-bit identical, and tags and cover art are untouched'
          : 'This format cannot be cut safely by copying, so it is decoded and re-encoded'
      }
    >
      <span className={lossless ? classes.lossless : classes.reencoded}>
        {METHOD_LABELS[audit.method] || audit.method}
      </span>
    </Tooltip>
  )
}

SilenceMethodField.propTypes = { record: PropTypes.object }
SilenceMethodField.defaultProps = { addLabel: true }

// StatusField distinguishes a song that has been cut from one merely measured.
export const SilenceStatusField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>-</span>
  if (audit.trimmedAt) {
    return (
      <Tooltip title={`Trimmed ${new Date(audit.trimmedAt).toLocaleString()}`}>
        <span className={classes.done}>Trimmed</span>
      </Tooltip>
    )
  }
  if (audit.status === 'failed') {
    return (
      <Tooltip title={audit.error || ''}>
        <span style={{ color: '#c62828' }}>Failed</span>
      </Tooltip>
    )
  }
  return <span className={classes.muted}>Measured</span>
}

SilenceStatusField.propTypes = { record: PropTypes.object }
SilenceStatusField.defaultProps = { addLabel: true }

// DurationChangeField shows the before -> after length of a trimmed song, and
// the predicted length of one that has not been cut yet.
export const DurationChangeField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit?.durationBefore) return <span className={classes.muted}>-</span>

  const before = audit.durationBefore
  const after = audit.trimmedAt
    ? audit.durationAfter
    : before - ((audit.leadTrim || 0) + (audit.trailTrim || 0))
  if (!after || Math.abs(after - before) < 0.005) {
    return <span className={classes.muted}>{formatSeconds(before)}</span>
  }
  return (
    <Tooltip title={audit.trimmedAt ? 'Actual length after trimming' : 'Length once trimmed'}>
      <span className={classes.ends}>
        {formatSeconds(before)}
        <span className={classes.endLabel}>→</span>
        <strong>{formatSeconds(after)}</strong>
      </span>
    </Tooltip>
  )
}

DurationChangeField.propTypes = { record: PropTypes.object }
DurationChangeField.defaultProps = { addLabel: true }

// OnsetField exposes the measurement the fade guard acts on.
//
// This is the number that decides whether a song is trimmed or refused, so it
// is available rather than hidden: when someone asks why a track was left
// alone, this column is the answer.
export const OnsetField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>-</span>
  const lead = audit.leadOnsetGap || 0
  const trail = audit.trailOnsetGap || 0
  if (lead === 0 && trail === 0) return <span className={classes.muted}>sharp</span>
  return (
    <Tooltip title="How gradually the audio arrives at the start and end. Informational - what decides whether a song is trimmed is the measured level of the audio being removed, not this.">
      <span className={classes.ends}>
        {`${(lead * 1000).toFixed(0)}ms`}
        <span className={classes.endLabel}>/</span>
        {`${(trail * 1000).toFixed(0)}ms`}
      </span>
    </Tooltip>
  )
}

OnsetField.propTypes = { record: PropTypes.object }
OnsetField.defaultProps = { addLabel: true }

export const SizeChangeField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit?.trimmedAt || !audit.sizeBefore) return <span className={classes.muted}>-</span>
  const saved = audit.sizeBefore - audit.sizeAfter
  if (saved <= 0) return <span className={classes.muted}>-</span>
  return <Typography variant="body2">{formatBytes(saved)}</Typography>
}

SizeChangeField.propTypes = { record: PropTypes.object }
SizeChangeField.defaultProps = { addLabel: true }

// RemovedPeakField is the evidence that a trim is safe: the loudest sample in
// the exact stretch about to be deleted, measured with a level meter rather
// than inferred from the silence detection.
//
// It earns a column because it is the answer to the only question that matters
// about this page - "how do you know that was nothing?" - and because it is
// checkable. The threshold it is judged against is SILENT_PEAK_DB, which is
// quiet enough to sit under the noise floor of a room, let alone a shop.
export const RemovedPeakField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  const peak = audit?.removedPeakDB
  if (peak === null || peak === undefined) {
    return <span className={classes.muted}>-</span>
  }
  const inaudible = peak <= SILENT_PEAK_DB
  return (
    <Tooltip
      title={
        inaudible
          ? `Loudest sample in the audio being removed. At or below ${SILENT_PEAK_DB} dB it is inaudible, so removing it changes nothing you can hear.`
          : `Loud enough to hear - this song is left alone.`
      }
    >
      <span className={inaudible ? classes.lossless : classes.reencoded}>
        {formatPeakDb(peak)}
      </span>
    </Tooltip>
  )
}

RemovedPeakField.propTypes = { record: PropTypes.object }
RemovedPeakField.defaultProps = { addLabel: true }

export const GaplessField = ({ record }) => {
  const classes = useStyles()
  const audit = auditOf(record)
  if (!audit) return <span className={classes.muted}>-</span>
  if (!audit.gapless) return <span className={classes.muted}>-</span>
  return (
    <Tooltip title="This album's tracks run into one another, so trimming would close gaps that belong to the recording">
      <Chip size="small" label="Continuous" className={`${classes.chip} ${classes.skipped}`} />
    </Tooltip>
  )
}

GaplessField.propTypes = { record: PropTypes.object }
GaplessField.defaultProps = { addLabel: true }
