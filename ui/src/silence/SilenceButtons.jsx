import React, { useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button,
  useNotify,
  useRefresh,
  useUnselectAll,
  useListContext,
} from 'react-admin'
import {
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Button as MuiButton,
} from '@material-ui/core'
import GraphicEqIcon from '@material-ui/icons/GraphicEq'
import ContentCutIcon from '@material-ui/icons/Crop'
import StopIcon from '@material-ui/icons/Stop'
import DeleteSweepIcon from '@material-ui/icons/DeleteSweep'
import { httpClient } from '../dataProvider'
import {
  ANALYZE_URL,
  CLEAR_URL,
  TRIM_URL,
  useSilenceStatus,
} from './useSilenceStatus'
import { formatTotalTime } from './format'

// AnalyzeSilenceButton measures songs without touching them.
//
// Always safe to press, which is why it needs no confirmation: it reads the
// files and writes a record, and the worst it can do is take a while.
export const AnalyzeSilenceButton = ({ all, selectedIds, disabled, onStarted }) => {
  const notify = useNotify()
  const unselectAll = useUnselectAll()
  const [starting, setStarting] = useState(false)
  const { watch } = useSilenceStatus(ANALYZE_URL)

  const handleClick = (e) => {
    e.stopPropagation()
    setStarting(true)
    const body = all ? '{}' : JSON.stringify({ ids: selectedIds || [] })
    httpClient(ANALYZE_URL, { method: 'POST', body })
      .then(({ json }) => {
        notify(json?.message || 'Analysing for silence', 'info')
        if (!all) unselectAll('silence')
        onStarted?.()
        watch(() => setStarting(false))
      })
      .catch((error) => {
        setStarting(false)
        notify(`Could not start the analysis: ${error.message}`, 'warning')
      })
  }

  return (
    <Button
      label={all ? 'Analyse all' : 'Analyse selected'}
      onClick={handleClick}
      disabled={disabled || starting || (!all && !selectedIds?.length)}
    >
      <GraphicEqIcon />
    </Button>
  )
}

AnalyzeSilenceButton.propTypes = {
  all: PropTypes.bool,
  selectedIds: PropTypes.array,
  disabled: PropTypes.bool,
  onStarted: PropTypes.func,
}

// TrimSilenceButton is the destructive one, and behaves like it.
//
// A trim cannot be undone from here - there is no backup by design, the client
// keeps their own - so it asks first, and the question states how many songs
// and how much audio, not merely "are you sure". A confirmation that does not
// say what will happen is a click-through.
export const TrimSilenceButton = ({
  all,
  selectedIds,
  disabled,
  onStarted,
  summary,
}) => {
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const [open, setOpen] = useState(false)
  const [starting, setStarting] = useState(false)
  const { watch } = useSilenceStatus(TRIM_URL)

  const count = all ? summary?.trimmable : selectedIds?.length
  const seconds = all ? summary?.pendingSeconds : null

  const run = () => {
    setOpen(false)
    setStarting(true)
    const body = all ? '{}' : JSON.stringify({ ids: selectedIds || [] })
    httpClient(TRIM_URL, { method: 'POST', body })
      .then(({ json }) => {
        notify(json?.message || 'Trimming silence', 'info')
        if (!all) unselectAll('silence')
        onStarted?.()
        watch(() => {
          setStarting(false)
          refresh()
        })
      })
      .catch((error) => {
        setStarting(false)
        notify(`Could not start the trim: ${error.message}`, 'warning')
      })
  }

  return (
    <>
      <Button
        label={all ? 'Trim all' : 'Trim selected'}
        onClick={(e) => {
          e.stopPropagation()
          setOpen(true)
        }}
        disabled={disabled || starting || (!all && !selectedIds?.length)}
      >
        <ContentCutIcon />
      </Button>
      <Dialog open={open} onClose={() => setOpen(false)}>
        <DialogTitle>Trim silence?</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">
            {all ? (
              <>
                Every song found to have removable silence will be cut -{' '}
                <strong>{count ?? 0} song(s)</strong>
                {seconds ? (
                  <>
                    , removing about <strong>{formatTotalTime(seconds)}</strong> in
                    total
                  </>
                ) : null}
                .
              </>
            ) : (
              <>
                <strong>{count ?? 0} selected song(s)</strong> will be cut. Only
                songs the analysis marked as safe to trim are affected; anything
                left alone stays as it is.
              </>
            )}
            <p>
              Half a second of quiet is kept at each end. Songs that fade in or
              out, and albums that play continuously, are not touched.
            </p>
            <p>
              <strong>This rewrites the files and cannot be undone from here.</strong>
            </p>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <MuiButton onClick={() => setOpen(false)}>Cancel</MuiButton>
          <MuiButton onClick={run} color="secondary" variant="contained">
            Trim
          </MuiButton>
        </DialogActions>
      </Dialog>
    </>
  )
}

TrimSilenceButton.propTypes = {
  all: PropTypes.bool,
  selectedIds: PropTypes.array,
  disabled: PropTypes.bool,
  onStarted: PropTypes.func,
  summary: PropTypes.object,
}

export const StopSilenceJobButton = ({ url, label, disabled, onStopped }) => {
  const notify = useNotify()
  const handleClick = () => {
    httpClient(`${url}/stop`, { method: 'POST' })
      .then(({ json }) => {
        notify(json?.message || 'Stopping', 'info')
        onStopped?.()
      })
      .catch((error) => notify(`Could not stop: ${error.message}`, 'warning'))
  }
  return (
    <Button label={label} onClick={handleClick} disabled={disabled}>
      <StopIcon />
    </Button>
  )
}

StopSilenceJobButton.propTypes = {
  url: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  disabled: PropTypes.bool,
  onStopped: PropTypes.func,
}

// ClearSilenceAnalysisButton forgets every measurement.
//
// It destroys no audio - songs already trimmed stay trimmed - so the warning
// says exactly that. Overstating it would train people to ignore the dialogs
// that do matter.
export const ClearSilenceAnalysisButton = ({ disabled, onCleared }) => {
  const notify = useNotify()
  const refresh = useRefresh()
  const [open, setOpen] = useState(false)

  const run = () => {
    setOpen(false)
    httpClient(CLEAR_URL, { method: 'DELETE' })
      .then(() => {
        notify('Silence analysis cleared', 'info')
        onCleared?.()
        refresh()
      })
      .catch((error) => notify(`Could not clear: ${error.message}`, 'warning'))
  }

  return (
    <>
      <Button label="Clear analysis" onClick={() => setOpen(true)} disabled={disabled}>
        <DeleteSweepIcon />
      </Button>
      <Dialog open={open} onClose={() => setOpen(false)}>
        <DialogTitle>Clear the silence analysis?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            This removes the measurements only. No audio is changed and songs
            already trimmed stay trimmed - they will simply need analysing again
            before they show anything here.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <MuiButton onClick={() => setOpen(false)}>Cancel</MuiButton>
          <MuiButton onClick={run} color="primary">
            Clear
          </MuiButton>
        </DialogActions>
      </Dialog>
    </>
  )
}

ClearSilenceAnalysisButton.propTypes = {
  disabled: PropTypes.bool,
  onCleared: PropTypes.func,
}

// SilenceBulkActions is the selection toolbar: analyse or trim exactly the
// songs the client ticked.
export const SilenceBulkActions = (props) => {
  const { selectedIds } = useListContext()
  return (
    <>
      <AnalyzeSilenceButton {...props} selectedIds={selectedIds} />
      <TrimSilenceButton {...props} selectedIds={selectedIds} />
    </>
  )
}
