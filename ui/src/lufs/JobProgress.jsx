import React from 'react'
import PropTypes from 'prop-types'
import { LinearProgress, Tooltip, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    flexDirection: 'column',
    minWidth: 190,
    marginRight: theme.spacing(2),
  },
  line: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    whiteSpace: 'nowrap',
  },
  bar: { height: 4, borderRadius: 2, marginTop: 2 },
}))

const eta = (processed, total, startedAt) => {
  if (!startedAt || !processed || !total || processed >= total) return null
  const elapsedMs = Date.now() - new Date(startedAt).getTime()
  if (elapsedMs <= 0) return null
  const remaining = ((total - processed) * elapsedMs) / processed
  const minutes = Math.round(remaining / 60000)
  if (minutes < 1) return 'less than a minute left'
  if (minutes < 60) return `about ${minutes} min left`
  const hours = (minutes / 60).toFixed(1)
  return `about ${hours} h left`
}

// JobProgress shows how far a long run has got. Counting the work up front
// means this is a real fraction rather than an open-ended counter, which
// matters when a run takes days.
export const JobProgress = ({ label, status, detail }) => {
  const classes = useStyles()
  if (!status?.running) {
    return (
      <>
        {status?.error && (
          <Typography role="alert" color="error">
            {status.error}
          </Typography>
        )}
        {!!status?.rejected && (
          <Typography role="alert" color="error">
            {`${status.rejected} rejected; working audio unchanged. Review the LUFS exceptions for details.`}
          </Typography>
        )}
      </>
    )
  }

  const processed = status.processed || 0
  const total = status.total || 0
  const pct =
    total > 0 ? Math.min(100, Math.round((processed / total) * 100)) : null
  const remaining = eta(processed, total, status.startedAt)

  // While stopping, the count of songs still open is the only number that is
  // going anywhere, and watching it fall is what tells the user the stop is
  // working. The overall progress bar is frozen by then and says nothing.
  const inFlight = status.inFlight || 0
  const stoppingText =
    inFlight > 0
      ? `Stopping - finishing ${inFlight.toLocaleString()} ${inFlight === 1 ? 'song' : 'songs'}…`
      : 'Stopping…'

  const text = status.stopping
    ? stoppingText
    : total > 0
      ? `${label} ${processed.toLocaleString()} / ${total.toLocaleString()}`
      : `${label} ${processed.toLocaleString()}`

  return (
    <Tooltip title={remaining || ''}>
      <div className={classes.root}>
        {status.error && (
          <Typography role="alert" color="error">
            {status.error}
          </Typography>
        )}
        <div className={classes.line}>
          <Typography variant="caption" color="textSecondary">
            {text}
          </Typography>
          {pct !== null && !status.stopping && (
            <Typography variant="caption" color="textSecondary">
              {`${pct}%`}
            </Typography>
          )}
        </div>
        <LinearProgress
          className={classes.bar}
          variant={pct !== null ? 'determinate' : 'indeterminate'}
          value={pct ?? 0}
        />
        {detail && (
          <Typography variant="caption" color="textSecondary">
            {detail}
          </Typography>
        )}
      </div>
    </Tooltip>
  )
}

JobProgress.propTypes = {
  label: PropTypes.string.isRequired,
  status: PropTypes.object,
  detail: PropTypes.string,
}

export default JobProgress
