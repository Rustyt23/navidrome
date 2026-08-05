import React, { useEffect, useState } from 'react'
import PropTypes from 'prop-types'
import { Card, CardContent, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'
import { SUMMARY_URL } from './useSilenceStatus'
import { formatTotalTime } from './format'

const useStyles = makeStyles((theme) => ({
  card: { marginBottom: theme.spacing(1) },
  content: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(3),
    alignItems: 'flex-end',
    padding: `${theme.spacing(1.5)}px ${theme.spacing(2)}px !important`,
  },
  headline: { display: 'flex', flexDirection: 'column' },
  // The one number the client came for.
  bigValue: { fontSize: '1.9rem', fontWeight: 700, lineHeight: 1.1 },
  value: { fontSize: '1.15rem', fontWeight: 600, lineHeight: 1.2 },
  label: {
    color: theme.palette.text.secondary,
    fontSize: '0.72rem',
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  },
  hint: {
    width: '100%',
    color: theme.palette.text.secondary,
    fontSize: '0.78rem',
  },
}))

const Stat = ({ label, value, big }) => {
  const classes = useStyles()
  return (
    <div className={classes.headline}>
      <span className={big ? classes.bigValue : classes.value}>{value}</span>
      <span className={classes.label}>{label}</span>
    </div>
  )
}

Stat.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.node.isRequired,
  big: PropTypes.bool,
}

// SilenceSummary describes the library rather than the page of rows on screen.
//
// Counted server-side for that reason: "142 songs have silence to remove" is a
// fact about the library, and computing it from the visible rows would make it
// change every time someone turned a page.
export const SilenceSummary = ({ refreshKey, onLoaded }) => {
  const classes = useStyles()
  const [summary, setSummary] = useState(null)

  useEffect(() => {
    let cancelled = false
    httpClient(SUMMARY_URL)
      .then(({ json }) => {
        if (cancelled) return
        setSummary(json)
        onLoaded?.(json)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
    // onLoaded is deliberately not a dependency: it is recreated on every
    // render by the parent, and depending on it would refetch in a loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshKey])

  if (!summary) return null

  if (!summary.analyzed) {
    return (
      <Card className={classes.card}>
        <CardContent className={classes.content}>
          <Typography variant="body2" className={classes.hint}>
            Nothing has been analysed yet. Press <strong>Analyse all</strong> to
            measure the silence at the start and end of every song - it only
            reads the files and changes nothing.
          </Typography>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className={classes.card}>
      <CardContent className={classes.content}>
        <Stat
          big
          label="waiting to be removed"
          value={formatTotalTime(summary.pendingSeconds)}
        />
        <Stat label="songs ready to trim" value={summary.trimmable.toLocaleString()} />
        <Stat label="nothing to remove" value={summary.clean.toLocaleString()} />
        <Stat label="left alone" value={summary.skipped.toLocaleString()} />
        {summary.failed > 0 && (
          <Stat label="could not measure" value={summary.failed.toLocaleString()} />
        )}
        <Stat label="analysed" value={summary.analyzed.toLocaleString()} />
      </CardContent>
    </Card>
  )
}

SilenceSummary.propTypes = {
  refreshKey: PropTypes.string,
  onLoaded: PropTypes.func,
}

export default SilenceSummary
