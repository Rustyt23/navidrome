import React from 'react'
import PropTypes from 'prop-types'
import PublishIcon from '@material-ui/icons/Publish'
import {
  Button as RaButton,
  useTranslate,
  useNotify,
} from 'react-admin'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'

import { REST_URL } from '../consts'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  dialogPaper: {
    backgroundColor: theme.palette.background.default,
    padding: theme.spacing(3, 4, 2, 4),
  },
  dialogTitle: {
    color: theme.palette.text.primary,
    fontSize: '1.25rem',
    fontWeight: theme.typography.fontWeightMedium,
    paddingBottom: theme.spacing(1),
  },
  dialogContentText: {
    color: theme.palette.text.secondary,
    fontSize: theme.typography.body1.fontSize,
  },
  cancelButton: {
    color: theme.palette.text.secondary,
    '&:hover': {
      backgroundColor: theme.palette.grey[700],
    },
  },
  confirmButton: {
    backgroundColor: theme.palette.secondary.main,
    color: theme.palette.getContrastText(theme.palette.secondary.main),
    '&:hover': {
      backgroundColor: theme.palette.grey[700],
    },
  },
}))

const PublishPlaylistButton = ({ record }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const classes = useStyles()
  const [open, setOpen] = React.useState(false)
  const [loading, setLoading] = React.useState(false)

  const handleDialogOpen = React.useCallback(() => {
    setOpen(true)
  }, [])

  const handleDialogClose = React.useCallback(() => {
    if (!loading) {
      setOpen(false)
    }
  }, [loading])

  const handlePublish = React.useCallback(() => {
    if (!record?.id) {
      setOpen(false)
      return Promise.resolve()
    }

    setLoading(true)
    return httpClient(`${REST_URL}/playlist/${record.id}/publish`, {
      method: 'POST',
    })
      .then(() => {
        notify('resources.playlist.notifications.published', 'info', {
          smart_count: record.songCount,
          name: record.name,
        })
        setOpen(false)
      })
      .catch(() => {
        notify('ra.page.error', 'warning')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [notify, record])

  return (
    <>
      <RaButton
        onClick={handleDialogOpen}
        disabled={loading}
        label={translate('resources.playlist.actions.publish')}
      >
        <PublishIcon />
      </RaButton>
      <Dialog
        open={open}
        onClose={handleDialogClose}
        aria-labelledby="publish-playlist-dialog"
        PaperProps={{ className: classes.dialogPaper }}
      >
        <DialogTitle id="publish-playlist-dialog" className={classes.dialogTitle}>
          {translate('resources.playlist.actions.publish')}
        </DialogTitle>
        <DialogContent>
          <DialogContentText className={classes.dialogContentText}>
            {translate('resources.playlist.dialog.publish_confirmation', {
              _: 'Are you sure you want to publish this playlist?',
            })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={handleDialogClose}
            disabled={loading}
            className={classes.cancelButton}
          >
            {translate('ra.action.no', { _: 'No' }).toUpperCase()}
          </Button>
          <Button
            onClick={handlePublish}
            disabled={loading}
            className={classes.confirmButton}
          >
            {translate('ra.action.yes', { _: 'Yes' }).toUpperCase()}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}

PublishPlaylistButton.propTypes = {
  record: PropTypes.shape({
    id: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
    name: PropTypes.string,
    songCount: PropTypes.number,
  }),
}

PublishPlaylistButton.defaultProps = {
  record: null,
}

export default PublishPlaylistButton
