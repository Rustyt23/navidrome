import React from 'react'
import PropTypes from 'prop-types'
import { ExportButton, TopToolbar, useListContext } from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { ToggleFieldsMenu } from '../common'
import SilenceProgress from './SilenceProgress'
import {
  AnalyzeSilenceButton,
  ClearSilenceAnalysisButton,
  StopSilenceJobButton,
  TrimSilenceButton,
} from './SilenceButtons'
import { ANALYZE_URL, TRIM_URL } from './useSilenceStatus'
import { formatTotalTime } from './format'

const useStyles = makeStyles((theme) => ({
  toolbar: {
    display: 'flex',
    flexDirection: 'column !important',
    alignItems: 'flex-end !important',
    width: '100%',
    minHeight: 'auto',
    padding: `0 0 ${theme.spacing(1)}px !important`,
    gap: theme.spacing(0.75),
    '& .MuiButton-root, & .MuiButton-label': { whiteSpace: 'nowrap' },
    [theme.breakpoints.down('sm')]: { alignItems: 'stretch !important' },
  },
  row: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    flexWrap: 'wrap',
    columnGap: theme.spacing(0.5),
    rowGap: theme.spacing(0.5),
  },
  utilityRow: {
    paddingTop: theme.spacing(0.5),
    borderTop: `1px solid ${theme.palette.divider}`,
    '& > div:last-child': { top: 0 },
    '& .MuiIconButton-root': { marginRight: '0 !important' },
  },
}))

const SilenceListActions = ({
  analyzeStatus,
  trimStatus,
  summary,
  onAnalyzeStarted,
  onTrimStarted,
  className,
  ...rest
}) => {
  const classes = useStyles()
  const { total } = useListContext()

  const busy = !!analyzeStatus?.running || !!trimStatus?.running

  // Abandoned songs are only mentioned once there are some, and worded so they
  // do not read as damage: they were left untouched and the next run takes them.
  const analyzeDetail = analyzeStatus?.running
    ? [
        `${analyzeStatus.changed || 0} with silence`,
        `${analyzeStatus.failed || 0} failed`,
        analyzeStatus.cancelled ? `${analyzeStatus.cancelled} left for next run` : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : undefined

  const trimDetail = trimStatus?.running
    ? [
        `${trimStatus.changed || 0} trimmed`,
        trimStatus.secondsRemoved
          ? `${formatTotalTime(trimStatus.secondsRemoved)} removed`
          : null,
        `${trimStatus.failed || 0} failed`,
        trimStatus.cancelled ? `${trimStatus.cancelled} left for next run` : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : undefined

  return (
    <TopToolbar className={`${className || ''} ${classes.toolbar}`} {...rest}>
      {busy && (
        <div className={classes.row}>
          <SilenceProgress
            label="Analysing"
            status={analyzeStatus}
            detail={analyzeDetail}
          />
          <SilenceProgress label="Trimming" status={trimStatus} detail={trimDetail} />
          {analyzeStatus?.running && (
            <StopSilenceJobButton
              url={ANALYZE_URL}
              label="Stop analysis"
              disabled={analyzeStatus?.stopping}
              onStopped={onAnalyzeStarted}
            />
          )}
          {trimStatus?.running && (
            <StopSilenceJobButton
              url={TRIM_URL}
              label="Stop trim"
              disabled={trimStatus?.stopping}
              onStopped={onTrimStarted}
            />
          )}
        </div>
      )}
      <div className={classes.row}>
        <AnalyzeSilenceButton all disabled={busy} onStarted={onAnalyzeStarted} />
        <TrimSilenceButton
          all
          disabled={busy || !summary?.trimmable}
          summary={summary}
          onStarted={onTrimStarted}
        />
      </div>
      <div className={`${classes.row} ${classes.utilityRow}`}>
        <ClearSilenceAnalysisButton disabled={busy} onCleared={onAnalyzeStarted} />
        <ExportButton maxResults={total} />
        <ToggleFieldsMenu resource="silence" />
      </div>
    </TopToolbar>
  )
}

SilenceListActions.propTypes = {
  analyzeStatus: PropTypes.object,
  trimStatus: PropTypes.object,
  summary: PropTypes.object,
  onAnalyzeStarted: PropTypes.func,
  onTrimStarted: PropTypes.func,
  className: PropTypes.string,
}

export default SilenceListActions
