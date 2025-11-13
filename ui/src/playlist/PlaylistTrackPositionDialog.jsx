import React, { useEffect, useState, useMemo } from 'react'
import PropTypes from 'prop-types'
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
} from '@material-ui/core'
import { useTranslate } from 'react-admin'

const clampValue = (value, maxPosition) => {
  const numericValue = Number.parseInt(value, 10)
  if (Number.isNaN(numericValue)) {
    return null
  }
  if (numericValue < 1 || numericValue > maxPosition) {
    return null
  }
  return numericValue
}

const normalizeInitialValue = (value) => {
  if (value == null) {
    return ''
  }
  if (typeof value === 'string') {
    return value.replace(/^_+/, '')
  }
  return String(value)
}

const PlaylistTrackPositionDialog = ({
  open,
  track,
  maxPosition,
  onCancel,
  onSubmit,
}) => {
  const translate = useTranslate()
  const [value, setValue] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) {
      setValue(normalizeInitialValue(track?.id))
      setError('')
    }
  }, [open, track])

  const hint = useMemo(
    () =>
      translate('resources.playlist.dialog.setPositionHint', {
        max: maxPosition,
        _: `Enter a value between 1 and ${maxPosition}.`,
      }),
    [translate, maxPosition],
  )

  const errorMessage = useMemo(
    () =>
      translate('resources.playlist.dialog.setPositionError', {
        max: maxPosition,
        _: `Please enter a value between 1 and ${maxPosition}.`,
      }),
    [translate, maxPosition],
  )

  const handleChange = (event) => {
    const newValue = event.target.value
    setValue(newValue)
    if (!newValue) {
      setError('')
      return
    }

    const numericValue = clampValue(newValue, maxPosition)
    setError(numericValue == null ? errorMessage : '')
  }

  const handleSubmit = (event) => {
    event.preventDefault()
    const numericValue = clampValue(value, maxPosition)
    if (numericValue == null || !track) {
      setError(errorMessage)
      return
    }

    onSubmit(track, numericValue)
  }

  return (
    <Dialog open={open} onClose={onCancel} maxWidth="xs" fullWidth>
      <form onSubmit={handleSubmit}>
        <DialogTitle>
          {translate('resources.playlist.dialog.setPositionTitle', {
            _: 'Set song position',
          })}
        </DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            type="number"
            label={translate('resources.playlist.dialog.setPositionLabel', {
              _: 'New position',
            })}
            value={value}
            onChange={handleChange}
            inputProps={{ min: 1, max: maxPosition }}
            helperText={error || hint}
            error={Boolean(error)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancel} color="primary">
            {translate('ra.action.cancel', { _: 'Cancel' })}
          </Button>
          <Button
            type="submit"
            color="primary"
            variant="contained"
            disabled={!value || Boolean(error)}
          >
            {translate('resources.playlist.dialog.setPositionConfirm', {
              _: 'Set position',
            })}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

PlaylistTrackPositionDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  track: PropTypes.object,
  maxPosition: PropTypes.number,
  onCancel: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
}

PlaylistTrackPositionDialog.defaultProps = {
  track: null,
  maxPosition: 1,
}

export default PlaylistTrackPositionDialog
