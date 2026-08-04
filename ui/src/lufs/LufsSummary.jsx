import React, { useCallback, useEffect, useState } from 'react'
import PropTypes from 'prop-types'
import { LinearProgress, Tooltip, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'

const SUMMARY_URL = '/api/song/loudness/summary'

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    padding: theme.spacing(1, 1.5),
    marginBottom: theme.spacing(1),
    borderRadius: 4,
    backgroundColor: theme.palette.action.hover,
  },
  headline: { display: 'flex', alignItems: 'baseline', gap: theme.spacing(1) },
  percent: { fontSize: '1.5rem', fontWeight: 700, lineHeight: 1 },
  bar: { flex: '1 1 160px', minWidth: 120, height: 6, borderRadius: 3 },
  stats: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1.5),
    marginLeft: 'auto',
  },
  stat: { whiteSpace: 'nowrap', fontSize: '0.8rem' },
  value: { fontWeight: 700, marginRight: 4 },
  muted: { color: theme.palette.text.secondary },
}))

// LufsSummary answers the question the table cannot.
//
// A page of rows says what happened to each song; nobody reads six hundred of
// them to find out whether the job is done. The client's requirement was a
// number for the whole library, so that is what goes at the top - and it is
// counted by the server using the same filters the list itself uses, so it can
// never disagree with what is underneath it.
export const LufsSummary = ({ refreshKey }) => {
  const classes = useStyles()
  const [summary, setSummary] = useState(null)

  const load = useCallback(() => {
    httpClient(SUMMARY_URL)
      .then(({ json }) => setSummary(json))
      .catch(() => setSummary(null))
  }, [])

  useEffect(load, [load, refreshKey])

  if (!summary?.songs) return null

  const {
    songs,
    onTarget,
    short,
    notMeasured,
    changed,
    restorable,
    exceptions,
    levelTwo,
  } = summary
  // Measured against the whole library rather than against what has been looked
  // at: "97% of the songs we got round to" is not the answer to "is my library
  // at -12.6".
  const percent = Math.round((onTarget / songs) * 100)

  const stat = (value, label, title) => (
    <Tooltip title={title}>
      <span className={`${classes.stat} ${classes.muted}`}>
        <span className={classes.value}>{value.toLocaleString()}</span>
        {label}
      </span>
    </Tooltip>
  )

  return (
    <div className={classes.root}>
      <div className={classes.headline}>
        <span className={classes.percent}>{percent}%</span>
        <Typography variant="body2" className={classes.muted}>
          {`on target (${summary.target.toFixed(2)} ±${summary.tolerance})`}
        </Typography>
      </div>
      <LinearProgress
        variant="determinate"
        value={percent}
        className={classes.bar}
      />
      <div className={classes.stats}>
        {stat(songs, 'songs', 'Every song in the library')}
        {stat(
          short,
          'short',
          'Measured, but not within tolerance of the target',
        )}
        {stat(notMeasured, 'not measured', 'Nothing is known about these yet')}
        {stat(changed, 'changed', 'Songs whose files were rewritten')}
        {stat(
          restorable,
          'restorable',
          'Songs whose untouched original is still stored',
        )}
        {stat(
          levelTwo,
          'level 2 tolerance',
          'Within half a decibel of target and left untouched: correcting them would have cost a re-encode for a difference nobody can hear. Nothing to do about these.',
        )}
        {stat(exceptions, 'exceptions', 'Songs that needed a person to look')}
      </div>
    </div>
  )
}

LufsSummary.propTypes = {
  // Changes whenever a job finishes, so the headline follows the work.
  refreshKey: PropTypes.any,
}

export default LufsSummary
