import React from 'react'
import {
  Button as RaButton,
  ExportButton,
  TopToolbar,
  useListContext,
} from 'react-admin'
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
}))

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

  return (
    <TopToolbar className={`${className || ''} ${classes.toolbar}`} {...rest}>
      {(libraryStatus?.running || analyzeStatus?.running) && (
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
        </div>
      )}
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
    </TopToolbar>
  )
}

export default LufsListActions
