import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import clsx from 'clsx'
import {
  Button as RaButton,
  useDataProvider,
  useNotify,
  usePermissions,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import {
  Button as MuiButton,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import EqualizerIcon from '@material-ui/icons/Equalizer'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

export const OptimizeLufsButton = ({ resource, selectedIds, className }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const dataProvider = useDataProvider()
  const { permissions } = usePermissions()
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)

  const selectedCount = selectedIds?.length || 0

  const handleOpen = useCallback(() => {
    if (selectedCount === 0) {
      return
    }
    setOpen(true)
  }, [selectedCount])

  const handleClose = useCallback(() => {
    if (!saving) {
      setOpen(false)
    }
  }, [saving])

  const handleConfirm = useCallback(async () => {
    if (!selectedCount || saving) {
      return
    }
    setSaving(true)
    try {
      const response = await dataProvider.optimizeSongLoudness(selectedIds)
      const normalized = response?.data?.normalized?.length || 0
      const skipped = response?.data?.skipped?.length || 0
      const failed = response?.data?.failed?.length || 0

      notify('resources.song.notifications.lufsOptimized', {
        type: failed > 0 ? 'warning' : 'info',
        messageArgs: { normalized, skipped, failed },
      })

      setOpen(false)
      unselectAll(resource)
      refresh({ hard: true })
    } catch (error) {
      notify(error?.message || 'ra.notification.http_error', {
        type: 'warning',
      })
    } finally {
      setSaving(false)
    }
  }, [
    dataProvider,
    notify,
    refresh,
    resource,
    saving,
    selectedCount,
    selectedIds,
    unselectAll,
  ])

  if (permissions !== 'admin') {
    return null
  }

  return (
    <>
      <RaButton
        onClick={handleOpen}
        className={clsx(classes.button, className)}
        label={translate('resources.song.actions.optimizeLufs')}
        disabled={!selectedCount}
      >
        <EqualizerIcon />
      </RaButton>
      <Dialog
        open={open}
        onClose={handleClose}
        aria-labelledby="optimize-lufs-dialog-title"
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle id="optimize-lufs-dialog-title">
          {translate('resources.song.dialogs.optimizeLufs.title', {
            smart_count: selectedCount,
          })}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {translate('resources.song.dialogs.optimizeLufs.description', {
              smart_count: selectedCount,
            })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <MuiButton onClick={handleClose} disabled={saving}>
            {translate('ra.action.cancel')}
          </MuiButton>
          <MuiButton color="primary" onClick={handleConfirm} disabled={saving}>
            {translate('resources.song.actions.optimizeLufs')}
          </MuiButton>
        </DialogActions>
      </Dialog>
    </>
  )
}

OptimizeLufsButton.propTypes = {
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  className: PropTypes.string,
}

OptimizeLufsButton.defaultProps = {
  selectedIds: [],
  className: undefined,
}

export default OptimizeLufsButton
