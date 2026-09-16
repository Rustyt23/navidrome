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

// ERROR_CHARS is as much of a job error as the page shows: about one line.
const ERROR_CHARS = 160

// shortError is the first line of an error, cut to about one line of text.
//
// A song stopped part way used to put its whole ffmpeg log here - a page of
// progress numbers in red. The server now sends a short message, but records
// written before that, and any unusually long reason, are still cut here.
const shortError = (message) => {
  const firstLine = String(message).split('\n')[0].trim()
  if (firstLine.length <= ERROR_CHARS) return firstLine
  const cut = firstLine.slice(0, ERROR_CHARS)
  const lastSpace = cut.lastIndexOf(' ')
  return `${(lastSpace > ERROR_CHARS / 2 ? cut.slice(0, lastSpace) : cut).trimEnd()}…`
}

// The full text stays one hover away, for whoever needs to read all of it.
const JobError = ({ message }) => (
  <Typography role="alert" color="error" title={String(message)}>
    {shortError(message)}
  </Typography>
)

// JobProgress shows how far a long run has got. Counting the work up front
// means this is a real fraction rather than an open-ended counter, which
// matters when a run takes days.
export const JobProgress = ({ label, status, detail }) => {
  const classes = useStyles()
  if (!status?.running) {
    return (
      <>
        {status?.error && <JobError message={status.error} />}
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
        {status.error && <JobError message={status.error} />}
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

JobError.propTypes = {
  message: PropTypes.string.isRequired,
}

JobProgress.propTypes = {
  label: PropTypes.string.isRequired,
  status: PropTypes.object,
  detail: PropTypes.string,
}

export default JobProgress
