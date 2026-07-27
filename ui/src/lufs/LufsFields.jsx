import React from 'react'
import PropTypes from 'prop-types'
import { useRecordContext, useTranslate } from 'react-admin'
import { Chip, Tooltip } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'

const useStyles = makeStyles((theme) => ({
  chip: {
    height: 20,
    fontSize: '0.72rem',
    fontWeight: 600,
  },
  safe: { backgroundColor: '#2e7d32', color: '#fff' },
  warn: { backgroundColor: '#ed6c02', color: '#fff' },
  bad: { backgroundColor: '#c62828', color: '#fff' },
  neutral: { backgroundColor: theme.palette.action.selected },
  pair: { whiteSpace: 'nowrap' },
  changed: { color: '#ef5350', fontWeight: 600 },
  same: { color: theme.palette.text.secondary },
  issues: { color: '#ef5350', fontSize: '0.78rem', whiteSpace: 'nowrap' },
  ok: { color: '#66bb6a' },
}))

const audit = (record) => record?.loudnessAudit
const has = (v) => v !== null && v !== undefined
const num = (v, digits = 2) => (has(v) ? Number(v).toFixed(digits) : null)

// Renders "before → after", greying the pair when nothing changed and
// highlighting it when it did.
const Pair = ({ before, after, changed, suffix = '' }) => {
  const classes = useStyles()
  if (!has(before) && !has(after))
    return <span className={classes.same}>-</span>
  if (!has(after)) {
    return (
      <span className={classes.pair}>
        {before}
        {suffix}
      </span>
    )
  }
  return (
    <span className={`${classes.pair} ${changed ? classes.changed : ''}`}>
      {has(before) ? `${before}${suffix}` : '?'} → {after}
      {suffix}
    </span>
  )
}

const verdictClass = (classes, verdict) => {
  switch (verdict) {
    case 'safe':
    case 'untouched':
      return classes.safe
    case 'dynamics_changed':
      return classes.warn
    case 'reencoded':
    case 'failed':
      return classes.bad
    default:
      return classes.neutral
  }
}

export const VerdictField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!a?.verdict) {
    return <span className={classes.same}>-</span>
  }
  const label = translate(`resources.lufs.verdict.${a.verdict}`, {
    _: a.verdict,
  })
  const chip = (
    <Chip
      size="small"
      label={label}
      className={`${classes.chip} ${verdictClass(classes, a.verdict)}`}
    />
  )
  return a.error ? <Tooltip title={a.error}>{chip}</Tooltip> : chip
}

export const StatusField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  const status = a?.status || 'not_analyzed'
  return (
    <Chip
      size="small"
      label={translate(`resources.lufs.status.${status}`, { _: status })}
      className={`${classes.chip} ${classes.neutral}`}
    />
  )
}

export const ActionField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!a?.action) return <span className={classes.same}>-</span>
  return (
    <span className={a.action === 'limited' ? classes.changed : undefined}>
      {translate(`resources.lufs.action.${a.action}`, { _: a.action })}
    </span>
  )
}

export const LufsPairField = (props) => {
  const record = useRecordContext(props)
  const a = audit(record)
  return <Pair before={num(a?.lufsBefore)} after={num(a?.lufsAfter)} />
}

// OriginalLufsField shows the loudness the song had before anything was done
// to it. For a processed track that is measured from the stored original; for
// one that has only been analysed, the file has never been rewritten, so its
// current measurement is the original. Either way this value never changes
// once recorded, which makes it the stable reference for the whole audit.
export const OriginalLufsField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!has(a?.lufsBefore)) {
    return <span className={classes.same}>-</span>
  }
  const fromBackup = a.status === 'processed' && a.hasBackup
  return (
    <Tooltip
      title={translate(
        fromBackup
          ? 'resources.lufs.originalFromBackup'
          : 'resources.lufs.originalFromFile',
      )}
    >
      <span className={classes.pair}>{num(a.lufsBefore)}</span>
    </Tooltip>
  )
}

export const GainField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!has(a?.gainApplied)) return <span className={classes.same}>-</span>
  const g = Number(a.gainApplied)
  return <span>{`${g > 0 ? '+' : ''}${g.toFixed(2)} dB`}</span>
}

export const TruePeakField = (props) => {
  const record = useRecordContext(props)
  const a = audit(record)
  // A rising true peak means the peaks were pushed up rather than just scaled
  const changed =
    has(a?.tpBefore) &&
    has(a?.tpAfter) &&
    Number(a.tpAfter) > Number(a.tpBefore)
  return (
    <Pair before={num(a?.tpBefore)} after={num(a?.tpAfter)} changed={changed} />
  )
}

export const LraField = (props) => {
  const record = useRecordContext(props)
  const a = audit(record)
  const changed =
    has(a?.lraBefore) &&
    has(a?.lraAfter) &&
    Math.abs(Number(a.lraAfter) - Number(a.lraBefore)) > 0.5
  return (
    <Pair
      before={num(a?.lraBefore, 1)}
      after={num(a?.lraAfter, 1)}
      changed={changed}
    />
  )
}

export const NullResidualField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!has(a?.nullResidual)) return <span className={classes.same}>-</span>
  const v = Number(a.nullResidual)
  return (
    <span className={v > -40 ? classes.changed : classes.ok}>
      {`${v.toFixed(1)} dB`}
    </span>
  )
}

// Format pairs. "After" is the file as it stands now, which is the media_file
// record itself; "before" comes from the audit snapshot.
const formatField = (beforeKey, afterKey, { suffix = '', digits } = {}) => {
  const Field = (props) => {
    const classes = useStyles()
    const record = useRecordContext(props)
    const a = audit(record)
    const before = a?.[beforeKey]
    const after = record?.[afterKey]
    // Until a file has actually been rewritten there is no "after" to compare
    // against, so show the single current value. Rendering "320k -> 320k" here
    // would read as a conversion that never happened.
    if (!a || !before || a.status !== 'processed') {
      const value = has(before) ? before : after
      return has(value) ? (
        <span
          className={classes.same}
        >{`${digits !== undefined ? Number(value).toFixed(digits) : value}${suffix}`}</span>
      ) : (
        <span className={classes.same}>-</span>
      )
    }
    const b = digits !== undefined ? Number(before).toFixed(digits) : before
    const c = digits !== undefined ? Number(after).toFixed(digits) : after
    return (
      <Pair
        before={b}
        after={c}
        changed={String(b) !== String(c)}
        suffix={suffix}
      />
    )
  }
  return Field
}

export const CodecPairField = formatField('codecBefore', 'suffix')
export const BitratePairField = formatField('bitrateBefore', 'bitRate', {
  suffix: 'k',
})
export const SampleRatePairField = formatField('sampleRateBefore', 'sampleRate')
export const BitDepthPairField = formatField('bitDepthBefore', 'bitDepth')
export const ChannelsPairField = formatField('channelsBefore', 'channels')

export const DurationDiffField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const a = audit(record)
  // Only meaningful once a file has been rewritten. On an untouched track the
  // two numbers come from different measurements of the same file - ffprobe
  // here, the scanner's own metadata read there - so any difference is a
  // rounding artefact, not a change to the song.
  if (
    a?.status !== 'processed' ||
    !a?.durationBefore ||
    !has(record?.duration)
  ) {
    return <span className={classes.same}>-</span>
  }
  const diff = Number(record.duration) - Number(a.durationBefore)
  if (Math.abs(diff) < 0.001) return <span className={classes.ok}>0</span>
  return (
    <span className={Math.abs(diff) > 0.05 ? classes.changed : classes.same}>
      {`${diff > 0 ? '+' : ''}${diff.toFixed(3)}s`}
    </span>
  )
}

export const ArtField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!a || a.status === 'analyzed') {
    return <span className={classes.same}>{a?.artBefore ? '✓' : '-'}</span>
  }
  if (a.artBefore && !a.artAfter) {
    return (
      <Tooltip title={translate('resources.lufs.artLost')}>
        <span className={classes.changed}>✓ → ✗</span>
      </Tooltip>
    )
  }
  return <span className={classes.ok}>{a.artAfter ? '✓' : '-'}</span>
}

// IntegrityField collapses every "must not change" property into one cell.
export const IntegrityField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!a || a.status !== 'processed') {
    return <span className={classes.same}>-</span>
  }
  const issues = []
  if (a.codecBefore && record.suffix && a.codecBefore !== record.suffix) {
    issues.push(`${a.codecBefore}→${record.suffix}`)
  }
  if (a.bitrateBefore && record.bitRate < a.bitrateBefore) {
    issues.push(`${a.bitrateBefore}k→${record.bitRate}k`)
  }
  if (a.sampleRateBefore && a.sampleRateBefore !== record.sampleRate) {
    issues.push(`${a.sampleRateBefore / 1000}→${record.sampleRate / 1000}kHz`)
  }
  if (
    a.bitDepthBefore &&
    record.bitDepth &&
    a.bitDepthBefore !== record.bitDepth
  ) {
    issues.push(`${a.bitDepthBefore}→${record.bitDepth}bit`)
  }
  if (a.channelsBefore && a.channelsBefore !== record.channels) {
    issues.push(`${a.channelsBefore}→${record.channels}ch`)
  }
  if (a.artBefore && !a.artAfter) {
    issues.push(translate('resources.lufs.artLostShort'))
  }
  if (!issues.length) {
    return (
      <span className={classes.ok}>
        {translate('resources.lufs.integrityOk')}
      </span>
    )
  }
  return (
    <Tooltip title={issues.join(', ')}>
      <span className={classes.issues}>{issues.join(', ')}</span>
    </Tooltip>
  )
}

Pair.propTypes = {
  before: PropTypes.any,
  after: PropTypes.any,
  changed: PropTypes.bool,
  suffix: PropTypes.string,
}
