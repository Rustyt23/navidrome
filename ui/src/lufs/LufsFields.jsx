import React from 'react'
import PropTypes from 'prop-types'
import { useRecordContext, useTranslate } from 'react-admin'
import { Chip, Tooltip } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { nullFloorFor, reportFor } from './report'

const useStyles = makeStyles((theme) => ({
  chip: {
    height: 20,
    fontSize: '0.72rem',
    fontWeight: 600,
  },
  safe: { backgroundColor: '#2e7d32', color: '#fff' },
  warn: {
    backgroundColor: '#fdd835',
    '& .MuiChip-label': { color: '#1565c0 !important' },
  },
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

// A song the engine opened, worked on and then declined to change has no stored
// verdict - the verdicts describe changes, and nothing changed. Left blank it
// reads as an oversight, so it is named here from the reason the run recorded.
// Derived rather than stored: the run already wrote down why, and inventing a
// verdict for it would mean a column and a migration to say the same thing.
const LEFT_AS_IS = 'left_as_is'
const verdictOf = (a) => {
  if (a?.verdict) return a.verdict
  if (a?.action === 'refused') return LEFT_AS_IS
  return ''
}

export const VerdictField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  const verdict = verdictOf(a)
  if (!verdict) {
    return <span className={classes.same}>-</span>
  }
  const label = translate(`resources.lufs.verdict.${verdict}`, {
    _: verdict,
  })
  const chip = (
    <Chip
      size="small"
      label={label}
      className={`${classes.chip} ${verdictClass(classes, verdict)}`}
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

// The pass mark depends on the format, not on a fixed number: re-encoding a
// lossy file always leaves a floor of codec noise around -25 dB, while a
// lossless round trip leaves almost nothing. Using one hard-coded threshold
// for both flagged perfectly clean MP3s as problems.
export const NullResidualField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const a = audit(record)
  if (!has(a?.nullResidual)) return <span className={classes.same}>-</span>

  const v = Number(a.nullResidual)
  const floor = nullFloorFor(a.codecAfter || a.codecBefore || record?.suffix)
  const altered = v > floor

  return (
    <Tooltip
      title={translate(
        altered ? 'resources.lufs.nullAltered' : 'resources.lufs.nullClean',
        { floor: floor.toFixed(0) },
      )}
    >
      <span className={altered ? classes.changed : classes.ok}>
        {`${v.toFixed(1)} dB`}
      </span>
    </Tooltip>
  )
}

// Format pairs. Both sides come from the audit's own probe so they are directly
// comparable; the media_file column is only a fallback for records written
// before the after snapshot existed.
const formatField = (
  beforeKey,
  afterKey,
  recordKey,
  { suffix = '', digits } = {},
) => {
  const Field = (props) => {
    const classes = useStyles()
    const record = useRecordContext(props)
    const a = audit(record)
    const before = a?.[beforeKey]
    const after = a?.codecAfter ? a?.[afterKey] : record?.[recordKey]
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

export const CodecPairField = formatField('codecBefore', 'codecAfter', 'suffix')
export const BitratePairField = formatField(
  'bitrateBefore',
  'bitrateAfter',
  'bitRate',
  { suffix: 'k' },
)
export const SampleRatePairField = formatField(
  'sampleRateBefore',
  'sampleRateAfter',
  'sampleRate',
)
export const BitDepthPairField = formatField(
  'bitDepthBefore',
  'bitDepthAfter',
  'bitDepth',
)
export const ChannelsPairField = formatField(
  'channelsBefore',
  'channelsAfter',
  'channels',
)

export const DurationDiffField = (props) => {
  const classes = useStyles()
  const record = useRecordContext(props)
  const a = audit(record)
  // Both sides must come from the same probe. Comparing against the scanner's
  // stored duration measures the difference between two measurement methods -
  // consistently about 0.03s - rather than any change to the song.
  if (a?.status !== 'processed' || !a?.durationBefore || !a?.durationAfter) {
    return <span className={classes.same}>-</span>
  }
  const diff = Number(a.durationAfter) - Number(a.durationBefore)
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
  // Both sides have to come from the same probe. Reading the "after" side from
  // media_file instead compares an ffprobe codec name against a file extension
  // - so every processed .m4a read as "aac→m4a" and every .ogg as "vorbis→ogg"
  // - and compares against whatever the last scan happened to record, which for
  // a freshly rewritten or restored file is the wrong generation entirely. The
  // audit's own after snapshot is used wherever it exists; media_file is only a
  // fallback for records written before that snapshot was kept.
  const hasAfterSnapshot = !!a.codecAfter
  const pick = (afterVal, recordVal) =>
    hasAfterSnapshot ? afterVal : recordVal

  const codecAfter = pick(a.codecAfter, record.suffix)
  const bitrateAfter = pick(a.bitrateAfter, record.bitRate)
  const sampleRateAfter = pick(a.sampleRateAfter, record.sampleRate)
  const bitDepthAfter = pick(a.bitDepthAfter, record.bitDepth)
  const channelsAfter = pick(a.channelsAfter, record.channels)

  const issues = []
  if (a.codecBefore && codecAfter && a.codecBefore !== codecAfter) {
    issues.push(`${a.codecBefore}→${codecAfter}`)
  }
  if (a.bitrateBefore && has(bitrateAfter) && bitrateAfter < a.bitrateBefore) {
    issues.push(`${a.bitrateBefore}k→${bitrateAfter}k`)
  }
  if (
    a.sampleRateBefore &&
    sampleRateAfter &&
    a.sampleRateBefore !== sampleRateAfter
  ) {
    issues.push(`${a.sampleRateBefore / 1000}→${sampleRateAfter / 1000}kHz`)
  }
  if (a.bitDepthBefore && bitDepthAfter && a.bitDepthBefore !== bitDepthAfter) {
    issues.push(`${a.bitDepthBefore}→${bitDepthAfter}bit`)
  }
  if (a.channelsBefore && channelsAfter && a.channelsBefore !== channelsAfter) {
    issues.push(`${a.channelsBefore}→${channelsAfter}ch`)
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

// ReportField says, in one cell, what optimisation actually did to a song -
// and separates the changes that matter from the ones that do not, so a clean
// result reads as clean instead of as a wall of numbers to interpret.
export const ReportField = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const record = useRecordContext(props)
  const report = reportFor(record, props.settings)

  if (!report) {
    return <span className={classes.same}>-</span>
  }

  const line = (text, key, cls) => (
    <div key={key} className={cls}>
      {text}
    </div>
  )

  const detail = (
    <div>
      {report.intended && line(`• ${report.intended}`, 'intended')}
      {report.significant.map((s, i) => line(`⚠ ${s}`, `s${i}`))}
      {report.minor.map((s, i) => line(`· ${s}`, `m${i}`))}
      {report.unchanged.length > 0 &&
        line(`✓ Unchanged: ${report.unchanged.join(', ')}`, 'unchanged')}
    </div>
  )

  const summary = report.clean
    ? translate('resources.lufs.report.levelOnly', { _: 'Level only' })
    : translate('resources.lufs.report.issues', {
        smart_count: report.significant.length,
        _: `${report.significant.length} significant`,
      })

  return (
    <Tooltip title={detail}>
      <span className={report.clean ? classes.ok : classes.changed}>
        {report.clean ? `✓ ${summary}` : `⚠ ${summary}`}
      </span>
    </Tooltip>
  )
}
