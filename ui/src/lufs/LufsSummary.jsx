import React, { useCallback, useEffect, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Collapse,
  IconButton,
  LinearProgress,
  Tooltip,
  Typography,
} from '@material-ui/core'
import EqualizerIcon from '@material-ui/icons/Equalizer'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import { makeStyles } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'

const SUMMARY_URL = '/api/song/loudness/summary'
// Remembered, because a panel that folds itself away again on every page load
// is one nobody opens twice.
const OPEN_KEY = 'lufs.summary.open'

// The same meanings the chips in the table carry, so a colour means one thing
// on this page rather than one thing per component.
const GOOD = '#2e7d32'
const INFO = '#1565c0'
const WARN = '#ef6c00'

const useStyles = makeStyles((theme) => ({
  root: {
    marginBottom: theme.spacing(1.5),
    borderRadius: 6,
    border: `1px solid ${theme.palette.divider}`,
    backgroundColor: theme.palette.background.paper,
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(1.25, 1.5),
    cursor: 'pointer',
    userSelect: 'none',
    '&:hover': { backgroundColor: theme.palette.action.hover },
  },
  title: { fontWeight: 700, letterSpacing: 0.3, flex: 1 },
  icon: { opacity: 0.7, display: 'flex' },
  body: { padding: theme.spacing(0, 2, 2) },

  // The headline, now inside the panel rather than in the bar.
  headline: {
    display: 'flex',
    alignItems: 'baseline',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
  },
  percent: {
    fontSize: '2.75rem',
    fontWeight: 800,
    lineHeight: 1,
    color: GOOD,
  },
  headlineNote: { color: theme.palette.text.secondary },
  bar: {
    height: 8,
    borderRadius: 4,
    margin: theme.spacing(1.5, 0, 2.5),
  },

  section: {
    fontWeight: 700,
    fontSize: '0.7rem',
    textTransform: 'uppercase',
    letterSpacing: 0.8,
    color: theme.palette.text.secondary,
    margin: theme.spacing(0, 0, 0.25),
  },
  // Says which of the two cuts a block is, because the cards look alike and
  // the second one overlaps the first. Without it "298 on target" beside "299
  // files changed" reads as an arithmetic error rather than as two different
  // questions about the same songs.
  sectionNote: {
    fontSize: '0.72rem',
    color: theme.palette.text.secondary,
    opacity: 0.8,
    margin: theme.spacing(0, 0, 1.25),
  },
  // auto-fit rather than a fixed column count: the panel sits above a wide
  // table and has to survive a narrow window without the cards collapsing into
  // unreadable slivers.
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))',
    gap: theme.spacing(1.5),
    marginBottom: theme.spacing(2.5),
  },
  card: {
    padding: theme.spacing(1.25, 1.5),
    borderRadius: 4,
    backgroundColor: theme.palette.action.hover,
    borderLeft: `3px solid ${theme.palette.divider}`,
  },
  value: {
    fontSize: '1.75rem',
    fontWeight: 700,
    lineHeight: 1.1,
    fontVariantNumeric: 'tabular-nums',
  },
  label: { fontSize: '0.82rem', fontWeight: 600, marginTop: 2 },
  note: {
    fontSize: '0.72rem',
    color: theme.palette.text.secondary,
    marginTop: 2,
  },
}))

// LufsSummary answers the question the table cannot.
//
// A page of rows says what happened to each song; nobody reads six hundred of
// them to find out whether the job is done. The client's requirement was a
// number for the whole library, so that is what this reports - counted by the
// server through the same filters the list itself uses, so the headline can
// never disagree with the page underneath it.
export const LufsSummary = ({ refreshKey }) => {
  const classes = useStyles()
  const [summary, setSummary] = useState(null)
  const [open, setOpen] = useState(
    () => localStorage.getItem(OPEN_KEY) === 'true',
  )

  const load = useCallback(() => {
    httpClient(SUMMARY_URL)
      .then(({ json }) => setSummary(json))
      .catch(() => setSummary(null))
  }, [])

  useEffect(load, [load, refreshKey])

  const toggle = useCallback(() => {
    setOpen((wasOpen) => {
      localStorage.setItem(OPEN_KEY, String(!wasOpen))
      return !wasOpen
    })
  }, [])

  if (!summary?.songs) return null

  const {
    songs,
    onTarget,
    notMeasured,
    changed,
    restorable,
    exceptions,
    levelTwo,
  } = summary
  // Level two counts as reached. Those songs are inside half a decibel with
  // nothing anyone would do about them, so excluding them reported the library
  // as further from done than it is - and the number people act on should be
  // the one that means "no work left", not "landed on the exact figure".
  //
  // Measured against the whole library rather than against what has been looked
  // at: "97% of the songs we got round to" is not the answer to "is my library
  // at -12.6".
  // "complete" rather than "on target": the level two songs are not on target,
  // they are near enough that nothing more will be done to them. Claiming they
  // hit the figure would be an overclaim the cards below immediately contradict.
  const complete = onTarget + levelTwo
  const percent = Math.round((complete / songs) * 100)
  const share = (n) =>
    songs ? `${Math.round((n / songs) * 100)}% of library` : ''

  const card = ({ label, value, note, accent, title }) => (
    <Tooltip key={label} title={title || ''}>
      <div
        className={classes.card}
        style={accent ? { borderLeftColor: accent } : undefined}
      >
        <div
          className={classes.value}
          style={accent ? { color: accent } : undefined}
        >
          {value.toLocaleString()}
        </div>
        <div className={classes.label}>{label}</div>
        <div className={classes.note}>{note}</div>
      </div>
    </Tooltip>
  )

  return (
    <div className={classes.root}>
      <div
        className={classes.header}
        onClick={toggle}
        onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && toggle()}
        role="button"
        tabIndex={0}
        aria-expanded={open}
      >
        <span className={classes.icon}>
          <EqualizerIcon fontSize="small" />
        </span>
        <Typography variant="body1" className={classes.title}>
          LUFS Optimisation Library
        </Typography>
        <IconButton size="small" aria-label={open ? 'Hide' : 'Show'}>
          {open ? <ExpandLessIcon /> : <ExpandMoreIcon />}
        </IconButton>
      </div>

      <Collapse in={open} timeout="auto" unmountOnExit>
        <div className={classes.body}>
          <div className={classes.headline}>
            <span className={classes.percent}>{percent}%</span>
            <Typography variant="body1" className={classes.headlineNote}>
              {`complete · ${songs.toLocaleString()} songs · ${summary.target.toFixed(2)} ±${summary.tolerance} LUFS`}
            </Typography>
          </div>
          <LinearProgress
            variant="determinate"
            value={percent}
            className={classes.bar}
          />

          <div className={classes.section}>Where the library stands</div>
          <div className={classes.sectionNote}>
            Every song is in exactly one of these — they add up to{' '}
            {songs.toLocaleString()}.
          </div>
          <div className={classes.grid}>
            {card({
              label: 'On target',
              value: onTarget,
              note: share(onTarget),
              accent: GOOD,
              title: `Within ${summary.tolerance} dB of ${summary.target.toFixed(2)} LUFS`,
            })}
            {card({
              label: 'Level 2 tolerance',
              value: levelTwo,
              note: share(levelTwo),
              accent: INFO,
              title:
                'Within half a decibel and left untouched: correcting them would have cost a re-encode for a difference nobody can hear',
            })}
            {card({
              label: 'Not measured',
              value: notMeasured,
              note: share(notMeasured),
              title: 'Nothing is known about these yet',
            })}
          </div>

          <div className={classes.section}>What was done</div>
          <div className={classes.sectionNote}>
            The same songs counted a different way — by what happened to the
            file. These overlap the figures above rather than adding to them.
          </div>
          <div className={classes.grid}>
            {card({
              label: 'Files changed',
              value: changed,
              note: share(changed),
              title:
                'Songs whose audio was rewritten. Slightly more than "on target" when a song was corrected as far as its peaks allowed and still landed a fraction short.',
            })}
            {card({
              label: 'Restorable',
              value: restorable,
              note: share(restorable),
              accent: GOOD,
              title: 'Songs whose untouched original is still stored',
            })}
            {card({
              label: 'Needs a decision',
              value: exceptions,
              note: share(exceptions),
              accent: exceptions > 0 ? WARN : undefined,
              title: 'Listed on the exceptions page, waiting on a person',
            })}
          </div>
        </div>
      </Collapse>
    </div>
  )
}

LufsSummary.propTypes = {
  // Changes whenever a job finishes, so the headline follows the work.
  refreshKey: PropTypes.any,
}

export default LufsSummary
