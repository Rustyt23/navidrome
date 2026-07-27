import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import StopIcon from '@material-ui/icons/Stop'
import DeleteSweepIcon from '@material-ui/icons/DeleteSweep'
import { makeStyles, alpha } from '@material-ui/core/styles'
import { Button, Confirm, useNotify, useRefresh } from 'react-admin'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  danger: {
    color: theme.palette.error.main,
    '&:hover': {
      backgroundColor: alpha(theme.palette.error.main, 0.12),
    },
  },
}))

export const StopLufsJobButton = ({ url, label, disabled, onStopped }) => {
  const classes = useStyles()
  const notify = useNotify()
  const [saving, setSaving] = useState(false)

  const stop = useCallback(() => {
    if (saving || disabled) return
    setSaving(true)
    httpClient(url, { method: 'POST' })
      .then(({ json }) => {
        notify(json?.message || 'Stop requested', 'info')
        onStopped?.()
      })
      .catch((error) => {
        notify(error?.body?.message || error?.message, 'warning')
      })
      .finally(() => setSaving(false))
  }, [disabled, notify, onStopped, saving, url])

  return (
    <Button
      className={classes.danger}
      disabled={saving || disabled}
      label={label}
      onClick={stop}
    >
      <StopIcon />
    </Button>
  )
}

StopLufsJobButton.propTypes = {
  url: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  disabled: PropTypes.bool,
  onStopped: PropTypes.func,
}

StopLufsJobButton.defaultProps = {
  disabled: false,
}

export const ClearLufsAnalysisButton = ({ disabled }) => {
  const classes = useStyles()
  const notify = useNotify()
  const refresh = useRefresh()
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)

  const clear = useCallback(() => {
    setSaving(true)
    httpClient('/api/song/loudness/analyze/results', { method: 'DELETE' })
      .then(({ json }) => {
        setOpen(false)
        notify(json?.message || 'LUFS analysis data cleared', 'info')
        refresh()
      })
      .catch((error) => {
        notify(error?.body?.message || error?.message, 'warning')
      })
      .finally(() => setSaving(false))
  }, [notify, refresh])

  return (
    <>
      <Button
        className={classes.danger}
        disabled={saving || disabled}
        label="resources.lufs.actions.clearAnalysis"
        onClick={() => setOpen(true)}
      >
        <DeleteSweepIcon />
      </Button>
      <Confirm
        isOpen={open}
        loading={saving}
        title="resources.lufs.clear.title"
        content="resources.lufs.clear.content"
        onConfirm={clear}
        onClose={() => setOpen(false)}
      />
    </>
  )
}

ClearLufsAnalysisButton.propTypes = {
  disabled: PropTypes.bool,
}

ClearLufsAnalysisButton.defaultProps = {
  disabled: false,
}
