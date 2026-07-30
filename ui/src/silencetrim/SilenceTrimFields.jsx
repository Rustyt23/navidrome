import React from 'react'
import PropTypes from 'prop-types'
import { Chip, Tooltip, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { useRecordContext } from 'react-admin'

const useStyles = makeStyles((theme) => ({
  chip: {
    height: 24,
    fontWeight: 600,
  },
  safe: {
    backgroundColor: theme.palette.success.main,
    color: theme.palette.success.contrastText,
  },
  review: {
    backgroundColor: theme.palette.warning.main,
    color: theme.palette.getContrastText(theme.palette.warning.main),
  },
  blocked: {
    backgroundColor: theme.palette.error.main,
    color: theme.palette.error.contrastText,
  },
  neutral: {
    backgroundColor: theme.palette.action.selected,
    color: theme.palette.text.primary,
  },
  verified: {
    color: theme.palette.success.main,
    fontWeight: 600,
  },
  error: {
    color: theme.palette.error.main,
    fontWeight: 600,
  },
  secondary: {
    color: theme.palette.text.secondary,
    fontSize: '0.78rem',
  },
}))

const auditOf = (record) => record?.silenceTrimAudit

const formatEdgeTime = (seconds) => {
  if (!seconds || seconds <= 0) return '—'
  if (seconds < 1) return `${Math.round(seconds * 1000)} ms`
  return `${seconds.toFixed(2)} s`
}

const chipDetails = {
  safe: ['Safe to trim', 'safe'],
  review: ['Review first', 'review'],
  blocked: ['Protected', 'blocked'],
  none: ['No trim needed', 'neutral'],
}

export const ClassificationField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) return <span>Not analyzed</span>
  const [label, style] = chipDetails[audit.classification] || [
    'Unknown',
    'neutral',
  ]
  return (
    <Chip
      size="small"
      label={label}
      className={`${classes.chip} ${classes[style]}`}
    />
  )
}

ClassificationField.propTypes = {
  label: PropTypes.string,
  record: PropTypes.object,
}

ClassificationField.defaultProps = { addLabel: true }

export const StatusField = (props) => {
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) return <span>Not analyzed</span>
  const labels = {
    analyzed: 'Dry run only',
    processed: 'Trimmed',
    failed: 'Could not verify',
  }
  return <span>{labels[audit.status] || audit.status || 'Not analyzed'}</span>
}

StatusField.propTypes = ClassificationField.propTypes
StatusField.defaultProps = { addLabel: true }

const kindLabel = {
  confirmed_silence: 'confirmed blank',
  digital_silence: 'confirmed blank',
  near_silence: 'near-silence',
  none: '',
}

export const EdgeField = ({ edge, ...props }) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) return <span>—</span>
  const seconds =
    edge === 'leading' ? audit.leadingSilence : audit.trailingSilence
  const kind = edge === 'leading' ? audit.leadingKind : audit.trailingKind
  return (
    <div>
      <div>{formatEdgeTime(seconds)}</div>
      {!!kindLabel[kind] && (
        <div className={classes.secondary}>{kindLabel[kind]}</div>
      )}
    </div>
  )
}

EdgeField.propTypes = {
  edge: PropTypes.oneOf(['leading', 'trailing']).isRequired,
  label: PropTypes.string,
  record: PropTypes.object,
}

EdgeField.defaultProps = { addLabel: true }

export const ProposalField = ({ edge, ...props }) => {
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) return <span>—</span>
  const seconds =
    edge === 'leading' ? audit.proposedStartTrim : audit.proposedEndTrim
  return <span>{formatEdgeTime(seconds)}</span>
}

ProposalField.propTypes = EdgeField.propTypes
ProposalField.defaultProps = { addLabel: true }

export const PaddingField = (props) => {
  const record = useRecordContext(props)
  const audit = auditOf(record)
  return <span>{audit ? formatEdgeTime(audit.retainedPadding) : '—'}</span>
}

PaddingField.propTypes = ClassificationField.propTypes
PaddingField.defaultProps = { addLabel: true }

export const MethodField = (props) => {
  const record = useRecordContext(props)
  const method = auditOf(record)?.method
  const labels = {
    lossless_sample_trim: 'Lossless sample trim',
    frame_aligned_copy: 'Frame-aligned copy',
  }
  return <span>{labels[method] || '—'}</span>
}

MethodField.propTypes = ClassificationField.propTypes
MethodField.defaultProps = { addLabel: true }

export const IntegrityField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) return <span>—</span>
  if (audit.integrity === 'verified') {
    return <span className={classes.verified}>Verified</span>
  }
  if (audit.integrity === 'failed') {
    return (
      <Tooltip title={audit.error || 'Candidate rejected'}>
        <span className={classes.error}>Not verified</span>
      </Tooltip>
    )
  }
  return <span>Pending</span>
}

IntegrityField.propTypes = ClassificationField.propTypes
IntegrityField.defaultProps = { addLabel: true }

export const DecisionField = (props) => {
  const record = useRecordContext(props)
  const decision = auditOf(record)?.decision
  const labels = {
    approve: 'Approved',
    skip: 'Leave unchanged',
  }
  return <span>{labels[decision] || 'Not decided'}</span>
}

DecisionField.propTypes = ClassificationField.propTypes
DecisionField.defaultProps = { addLabel: true }

export const DurationAfterField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) return <span>—</span>
  const after =
    audit.durationAfter > 0
      ? audit.durationAfter
      : audit.durationBefore - audit.proposedStartTrim - audit.proposedEndTrim
  const removed = Math.max(0, audit.durationBefore - after)
  return (
    <div>
      <div>{formatEdgeTime(after)}</div>
      {removed > 0 && (
        <div className={classes.secondary}>
          {formatEdgeTime(removed)} shorter
        </div>
      )}
    </div>
  )
}

DurationAfterField.propTypes = ClassificationField.propTypes
DurationAfterField.defaultProps = { addLabel: true }

export const ReasonField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const audit = auditOf(record)
  if (!audit) {
    return (
      <Typography variant="body2" className={classes.secondary}>
        Run analysis to create a dry-run proposal
      </Typography>
    )
  }
  const text = audit.error || audit.reason || '—'
  return (
    <Tooltip title={text}>
      <Typography variant="body2" noWrap style={{ maxWidth: 360 }}>
        {text}
      </Typography>
    </Tooltip>
  )
}

ReasonField.propTypes = ClassificationField.propTypes
ReasonField.defaultProps = { addLabel: true }
