import React, { useEffect, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from '@material-ui/core'
import { useTranslate } from 'react-admin'

const clampPosition = (value, max) => {
  if (typeof max !== 'number' || max <= 0) {
    return 1
  }
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) {
    return 1
  }
  return Math.min(Math.max(Math.round(parsed), 1), max)
}

const MoveTrackDialog = ({
  open,
  record,
  onClose,
  onSubmit,
  maxPosition,
  currentPosition,
}) => {
  const translate = useTranslate()
  const [position, setPosition] = useState(1)

  const effectiveMax = Math.max(1, maxPosition || 1)

  const normalizedCurrent = useMemo(() => {
    if (typeof currentPosition === 'number') {
      return clampPosition(currentPosition, effectiveMax)
    }
    if (record?.id) {
      if (typeof record.playlistPosition === 'number') {
        return clampPosition(record.playlistPosition, effectiveMax)
      }
      if (typeof record.id === 'string' && /^\d+$/.test(record.id)) {
        return clampPosition(parseInt(record.id, 10), effectiveMax)
      }
      if (typeof record.id === 'number') {
        return clampPosition(record.id, effectiveMax)
      }
    }
    return 1
  }, [currentPosition, record, effectiveMax])

  useEffect(() => {
    if (!open) {
      return
    }
    setPosition(normalizedCurrent)
  }, [open, normalizedCurrent])

  const handleChange = (event) => {
    setPosition(event.target.value)
  }

  const numericValue = Number(position)
  const isValid =
    Number.isInteger(numericValue) &&
    numericValue >= 1 &&
    numericValue <= effectiveMax

  const handleSubmit = (event) => {
    if (event) {
      event.preventDefault()
    }
    if (!isValid) {
      return
    }
    onSubmit(clampPosition(numericValue, effectiveMax))
  }

  return (
    <Dialog open={open} onClose={onClose} aria-labelledby="move-track-dialog">
      <form onSubmit={handleSubmit}>
        <DialogTitle id="move-track-dialog">
          {translate('resources.playlist.dialog.moveTrackTitle')}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {translate('resources.playlist.dialog.moveTrackSubtitle', {
              title: record?.title || '',
            })}
          </DialogContentText>
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            type="number"
            id="move-track-position"
            label={translate(
              'resources.playlist.dialog.moveTrackPositionLabel',
            )}
            value={position}
            onChange={handleChange}
            inputProps={{ min: 1, max: effectiveMax }}
            helperText={translate(
              'resources.playlist.dialog.moveTrackHelperText',
              { max: effectiveMax },
            )}
            error={!isValid}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose} color="primary">
            {translate('ra.action.cancel')}
          </Button>
          <Button type="submit" color="primary" disabled={!isValid}>
            {translate('resources.playlist.dialog.moveTrackConfirm')}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

MoveTrackDialog.propTypes = {
  open: PropTypes.bool,
  record: PropTypes.object,
  onClose: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
  maxPosition: PropTypes.number,
  currentPosition: PropTypes.number,
}

MoveTrackDialog.defaultProps = {
  open: false,
  record: null,
  maxPosition: 0,
  currentPosition: undefined,
}

export default MoveTrackDialog
