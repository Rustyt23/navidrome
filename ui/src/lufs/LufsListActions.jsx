import React, { useCallback, useState } from 'react'
import {
  Button as RaButton,
  ExportButton,
  TopToolbar,
  useListContext,
} from 'react-admin'
import { Collapse, Typography } from '@material-ui/core'
import BuildIcon from '@material-ui/icons/Build'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import { makeStyles } from '@material-ui/core/styles'
import { ToggleFieldsMenu } from '../common'
import LufsToggle from './LufsToggle'
import BackupToggle from './BackupToggle'
import { AnalyzeLufsButton } from './LufsAnalyzeButton'
import SpeedIcon from '@material-ui/icons/Speed'
import TuneIcon from '@material-ui/icons/Tune'
import { useHistory } from 'react-router-dom'
import JobProgress from './JobProgress'
import { ClearLufsAnalysisButton, StopLufsJobButton } from './LufsJobButtons'
import { BackupCleanupButton } from './BackupCleanupButton'
import { RestoreAuditDbButton, SaveAuditDbButton } from './AuditDbButtons'
import { ANALYZE_URL } from './useAnalyzeStatus'
import { LIBRARY_URL } from './useLibraryStatus'
import { RESTORE_URL, useRestoreStatus } from './useRestoreStatus'

const useStyles = makeStyles((theme) => ({
  toolbar: {
    display: 'flex',
    flexDirection: 'column !important',
    alignItems: 'flex-end !important',
    width: '100%',
    minHeight: 'auto',
    padding: `0 0 ${theme.spacing(1)}px !important`,
    gap: theme.spacing(0.75),
    '& .MuiButton-root, & .MuiButton-label': {
      whiteSpace: 'nowrap',
    },
    [theme.breakpoints.down('sm')]: {
      alignItems: 'stretch !important',
    },
  },
  row: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    flexWrap: 'wrap',
    columnGap: theme.spacing(0.5),
    rowGap: theme.spacing(0.5),
  },
  exceptionsButton: {
    color: '#2196f3',
  },
  utilityRow: {
    paddingTop: theme.spacing(0.5),
    borderTop: `1px solid ${theme.palette.divider}`,
    '& > div:last-child': {
      top: 0,
    },
    '& .MuiIconButton-root': {
      marginRight: '0 !important',
    },
  },

  // Layout only - no border of its own. The box belongs to the content, so a
  // folded panel leaves the corner toggle and nothing else. Drawn on the
  // wrapper instead, it collapsed to a full-width empty rectangle: a long line
  // across the page marking out the space where the buttons used to be.
  panel: { width: '100%' },
  // The bar is not the control. A full-width click target the height of a row
  // gives no clue where to click and swallows clicks meant for nothing at all,
  // so the header only positions the toggle - at the right, over the buttons
  // it folds away.
  panelHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    padding: theme.spacing(0.5, 0.75),
  },
  panelToggle: {
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
  panelTitle: { fontWeight: 700, letterSpacing: 0.3 },
  panelBody: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.75),
    padding: theme.spacing(1, 1.5),
    borderRadius: 6,
    border: `1px solid ${theme.palette.divider}`,
    backgroundColor: theme.palette.background.paper,
  },
}))

// Remembered, so the panel does not spring open again on every page load for
// someone who folded it away.
const OPEN_KEY = 'lufs.actions.open'

// The exception list is a view of the same songs, not a separate area of the
// app, so it is reached from here rather than from the sidebar - where it sat
// next to Albums and Artists as though it were a library of its own.
const OpenExceptionsButton = () => {
  const classes = useStyles()
  const history = useHistory()
  return (
    <RaButton
      className={classes.exceptionsButton}
      onClick={() => history.push('/lufs2')}
      label="resources.lufs.actions.openExceptions"
    >
      <TuneIcon />
    </RaButton>
  )
}

const LufsListActions = ({
  settings,
  onSettingsChange,
  analyzeStatus,
  onAnalyzeStarted,
  libraryStatus,
  onOptimiseAllToggled,
  className,
  ...rest
}) => {
  const classes = useStyles()
  const { total } = useListContext()
  // Read here rather than passed down like the other two: a restore is started
  // from the selection toolbar, which is a different component, so this is the
  // only place that can draw its progress. The shared store means both see the
  // same job.
  const { status: restoreStatus } = useRestoreStatus()

  // Open unless it was closed before. A control panel that hides itself the
  // first time someone opens the page has hidden the page's controls.
  const [open, setOpen] = useState(
    () => localStorage.getItem(OPEN_KEY) !== 'false',
  )
  const toggle = useCallback(() => {
    setOpen((wasOpen) => {
      localStorage.setItem(OPEN_KEY, String(!wasOpen))
      return !wasOpen
    })
  }, [])

  // Abandoned tracks are shown only once there are some, and worded so they do
  // not read as damage: they were left alone and the next run picks them up.
  const optimiseDetail = libraryStatus?.running
    ? [
        `${libraryStatus.normalized || 0} changed`,
        `${libraryStatus.skipped || 0} already fine`,
        `${libraryStatus.failed || 0} failed`,
        libraryStatus.cancelled
          ? `${libraryStatus.cancelled} left for next run`
          : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : undefined
  const analyzeDetail = analyzeStatus?.running
    ? `${analyzeStatus.failed || 0} failed`
    : undefined
  const restoreDetail = restoreStatus?.running
    ? [
        `${restoreStatus.restored || 0} restored`,
        `${restoreStatus.skipped || 0} nothing stored`,
        `${restoreStatus.failed || 0} failed`,
      ].join(' · ')
    : undefined

  return (
    <TopToolbar className={`${className || ''} ${classes.toolbar}`} {...rest}>
      {(libraryStatus?.running ||
        analyzeStatus?.running ||
        restoreStatus?.running) && (
        <div className={classes.row}>
          <JobProgress
            label="Optimising"
            status={libraryStatus}
            detail={optimiseDetail}
          />
          <JobProgress
            label="Analysing"
            status={analyzeStatus}
            detail={analyzeDetail}
          />
          <JobProgress
            label="Restoring"
            status={restoreStatus}
            detail={restoreDetail}
          />
          {libraryStatus?.running && (
            <StopLufsJobButton
              url={`${LIBRARY_URL}/stop`}
              label="resources.lufs.actions.stopOptimisation"
              disabled={libraryStatus?.stopping}
              onStopped={onOptimiseAllToggled}
            />
          )}
          {analyzeStatus?.running && (
            <StopLufsJobButton
              url={`${ANALYZE_URL}/stop`}
              label="resources.lufs.actions.stopAnalysis"
              disabled={analyzeStatus?.stopping}
              onStopped={onAnalyzeStarted}
            />
          )}
          {restoreStatus?.running && (
            <StopLufsJobButton
              url={`${RESTORE_URL}/stop`}
              label="resources.lufs.actions.stopRestore"
              disabled={restoreStatus?.stopping}
            />
          )}
        </div>
      )}
      <div className={classes.panel}>
        <div className={classes.panelHeader}>
          {/* One element, not a label beside a button: two click targets that
              do the same thing means half the clicks land on the half that
              looks less like a control. */}
          <span
            className={classes.panelToggle}
            onClick={toggle}
            onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && toggle()}
            role="button"
            tabIndex={0}
            aria-expanded={open}
          >
            <BuildIcon fontSize="small" />
            <Typography variant="body2" className={classes.panelTitle}>
              LUFS Controls
            </Typography>
            {open ? (
              <ExpandLessIcon fontSize="small" />
            ) : (
              <ExpandMoreIcon fontSize="small" />
            )}
          </span>
        </div>

        {/* No unmountOnExit. These buttons carry their own state and a couple
            poll while mounted, so folding the panel should hide them rather
            than tear them down and rebuild them. Collapse marks the collapsed
            content visibility:hidden, which keeps it out of the tab order. */}
        <Collapse in={open} timeout="auto">
          <div className={classes.panelBody}>
            <div className={classes.row}>
              <OpenExceptionsButton />
              <LufsToggle
                onChange={onSettingsChange}
                onToggled={onOptimiseAllToggled}
                disabled={!!analyzeStatus?.running}
                libraryStatus={libraryStatus}
              />
              <BackupToggle
                settings={settings}
                onChange={onSettingsChange}
                disabled={!!analyzeStatus?.running}
                libraryStatus={libraryStatus}
              />
              <AnalyzeLufsButton
                all
                mode="original"
                label="resources.lufs.actions.fetchOriginal"
                icon={<SpeedIcon />}
                onStarted={onAnalyzeStarted}
                disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
              />
              <AnalyzeLufsButton
                all
                onStarted={onAnalyzeStarted}
                disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
              />
            </div>
            <div className={`${classes.row} ${classes.utilityRow}`}>
              <ClearLufsAnalysisButton
                disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
              />
              <SaveAuditDbButton />
              {/* Restoring rewrites the same rows a running job is writing, so it is
            held back while one is going - the server refuses it as well. */}
              <RestoreAuditDbButton
                disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
              />
              <BackupCleanupButton />
              <ExportButton maxResults={total} />
              <ToggleFieldsMenu resource="lufs" />
            </div>
          </div>
        </Collapse>
      </div>
    </TopToolbar>
  )
}

export default LufsListActions
