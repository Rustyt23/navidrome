import React, { useCallback, useEffect, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Collapse,
  LinearProgress,
  Tooltip,
  Typography,
} from '@material-ui/core'
import EqualizerIcon from '@material-ui/icons/Equalizer'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import { makeStyles } from '@material-ui/core/styles'
import { useHistory } from 'react-router-dom'
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
  // No border on the wrapper. Carried here it collapsed to a full-width empty
  // rectangle - a long line across the top of the page holding open the space
  // where the panel used to be. The box belongs to the content, so folding it
  // away leaves the corner toggle and nothing else.
  root: { marginBottom: theme.spacing(1) },
  // The bar is not the control: a click target the width of the page gives no
  // clue where to click. It only positions the toggle, in the same corner the
  // controls panel below it uses.
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    padding: theme.spacing(0.5, 0.75),
  },
  toggle: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.25, 0.75),
    borderRadius: 4,
    cursor: 'pointer',
    userSelect: 'none',
    '&:hover': { backgroundColor: theme.palette.action.hover },
    '&:focus-visible': {
      outline: `2px solid ${theme.palette.primary.main}`,
      outlineOffset: 2,
    },
    '& .MuiSvgIcon-root': { opacity: 0.7 },
  },
  title: { fontWeight: 700, letterSpacing: 0.3 },
  body: {
    padding: theme.spacing(2),
    borderRadius: 6,
    border: `1px solid ${theme.palette.divider}`,
    backgroundColor: theme.palette.background.paper,
  },

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
  // Only the cards that open something say so. A hover state on all of them
  // would promise every number is a way in, and most are not.
  clickable: {
    cursor: 'pointer',
    '&:hover': { backgroundColor: theme.palette.action.selected },
    '&:focus-visible': { outline: `2px solid ${theme.palette.primary.main}` },
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
  // Before the early return below: hooks cannot be called conditionally.
  const history = useHistory()
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
    rejected,
    shortOfTarget,
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
  // Short-of-target songs count as complete for the same reason level two does:
  // nothing more will be done to them. They were corrected as far as the song
  // allowed. Leaving them out would report the library as further from done
  // than it is, on songs that are finished.
  const complete = onTarget + levelTwo + (shortOfTarget || 0)
  const percent = Math.round((complete / songs) * 100)
  const share = (n) =>
    songs ? `${Math.round((n / songs) * 100)}% of library` : ''

  // The query string is taken whole by react-admin or not at all, so each
  // filter is sent on its own: a card opens exactly the songs it counted,
  // rather than narrowing whatever was already on screen. page=1 because the
  // page someone was on has no meaning in a different set of rows.
  //
  // Every card routes through here so the number and the list can never come
  // from different questions - the same reason the counts themselves are taken
  // from the list's own filters rather than from a second copy of the rule.
  const openFiltered = (filter) => () =>
    history.push({
      pathname: '/lufs',
      search: `?filter=${encodeURIComponent(JSON.stringify(filter))}&page=1`,
    })

  // onOpen turns a card into a way into the songs it counts. Without it a
  // number that says "35 of these exist" leaves someone to work out which 35,
  // on a page of ninety thousand rows.
  const card = ({ label, value, note, accent, title, onOpen }) => (
    <Tooltip key={label} title={title || ''}>
      <div
        className={`${classes.card} ${onOpen ? classes.clickable : ''}`}
        style={accent ? { borderLeftColor: accent } : undefined}
        onClick={onOpen}
        onKeyDown={
          onOpen
            ? (e) => (e.key === 'Enter' || e.key === ' ') && onOpen()
            : undefined
        }
        role={onOpen ? 'button' : undefined}
        tabIndex={onOpen ? 0 : undefined}
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
      <div className={classes.header}>
        {/* One element, not a label beside a button: two click targets doing
            the same thing means half the clicks land on whichever half looks
            less like a control. */}
        <span
          className={classes.toggle}
          onClick={toggle}
          onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && toggle()}
          role="button"
          tabIndex={0}
          aria-expanded={open}
        >
          <EqualizerIcon fontSize="small" />
          <Typography variant="body2" className={classes.title}>
            LUFS Optimisation Library
          </Typography>
          {open ? (
            <ExpandLessIcon fontSize="small" />
          ) : (
            <ExpandMoreIcon fontSize="small" />
          )}
        </span>
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
                'Within half a decibel of target and never opened: correcting them would have cost a re-encode for a difference nobody can hear. Click to see them.',
              onOpen:
                levelTwo > 0
                  ? openFiltered({ loudness_level_two: true })
                  : undefined,
            })}
            {card({
              label: 'Short of target',
              value: shortOfTarget || 0,
              note: share(shortOfTarget || 0),
              title:
                'Corrected as far as the song allowed, and still outside the ordinary tolerance. Nothing more will be done to them. Click to see them.',
              onOpen:
                shortOfTarget > 0
                  ? openFiltered({ loudness_short: true })
                  : undefined,
            })}
            {card({
              label: 'Could not process',
              value: notMeasured,
              note: share(notMeasured),
              accent: notMeasured > 0 ? WARN : undefined,
              title:
                'Songs nothing could be read from - no audio in the file, or a decode that failed. Nothing is known about their loudness. Click to see them.',
              // An array because the Outcome filter takes several at once, so
              // the chip that appears on the page reads as a normal selection
              // someone could have made themselves.
              onOpen:
                notMeasured > 0
                  ? openFiltered({ loudness_outcome: ['not_measured'] })
                  : undefined,
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
            {card({
              label: 'Rejected',
              value: rejected || 0,
              note: share(rejected || 0),
              accent: rejected > 0 ? WARN : undefined,
              title:
                'A corrected copy was built, measured, judged not good enough and thrown away. The song on disk was never touched. Click to see them and why each was refused.',
              onOpen:
                rejected > 0
                  ? openFiltered({ loudness_rejected: true })
                  : undefined,
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
