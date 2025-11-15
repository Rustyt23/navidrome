import React, { useCallback, useEffect, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import clsx from 'clsx'
import {
  Button as RaButton,
  useDataProvider,
  useListContext,
  useNotify,
  usePermissions,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import {
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
  Button as MuiButton,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import CommentIcon from '@material-ui/icons/Comment'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

const findRecord = (data, id) => {
  if (!data) {
    return undefined
  }

  if (Array.isArray(data)) {
    return data.find(
      (record) =>
        record &&
        (record.id === id ||
          record.id === String(id) ||
          record.id === Number(id)),
    )
  }

  return data[id] ?? data[String(id)] ?? data[Number(id)]
}

export const EditSongCommentButton = ({
  resource,
  selectedIds,
  className,
}) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const dataProvider = useDataProvider()
  const listContext = useListContext()
  const { permissions } = usePermissions()

  const [open, setOpen] = useState(false)
  const [comment, setComment] = useState('')
  const [saving, setSaving] = useState(false)

  const selectedRecords = useMemo(() => {
    if (!selectedIds?.length) {
      return []
    }

    const data = listContext?.data
    return selectedIds
      .map((id) => findRecord(data, id))
      .filter((record) => record != null)
  }, [listContext?.data, selectedIds])

  const sharedComment = useMemo(() => {
    if (!selectedRecords.length) {
      return ''
    }

    const firstComment = selectedRecords[0]?.comment ?? ''
    return selectedRecords.every(
      (record) => (record?.comment ?? '') === firstComment,
    )
      ? firstComment
      : ''
  }, [selectedRecords])

  useEffect(() => {
    if (open) {
      setComment(sharedComment || '')
    }
  }, [open, sharedComment])

  const handleOpen = useCallback(() => {
    if (!selectedIds?.length) {
      return
    }
    setOpen(true)
  }, [selectedIds])

  const handleClose = useCallback(() => {
    if (saving) {
      return
    }
    setOpen(false)
  }, [saving])

  const handleSubmit = useCallback(
    async (event) => {
      event.preventDefault()

      if (!selectedIds?.length || saving) {
        return
      }

      setSaving(true)
      try {
        const response = await dataProvider.updateMany(resource, {
          ids: selectedIds,
          data: { comment: comment || '' },
        })
        const updatedIds = Array.isArray(response?.data)
          ? response.data
          : selectedIds

        if (updatedIds.length > 0) {
          notify('ra.notification.updated', {
            type: 'info',
            messageArgs: { smart_count: updatedIds.length },
          })
        }

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
    },
    [
      comment,
      dataProvider,
      notify,
      refresh,
      resource,
      saving,
      selectedIds,
      unselectAll,
    ],
  )

  const selectedCount = selectedIds?.length || 0
  const translationBase = useMemo(
    () => `resources.${resource || 'song'}`,
    [resource],
  )
  const title = translate(`${translationBase}.dialogs.editComment.title`, {
    smart_count: selectedCount,
  })
  const description = translate(
    `${translationBase}.dialogs.editComment.description`,
    { smart_count: selectedCount },
  )
  const buttonLabel = translate(`${translationBase}.actions.editComment`)
  const dialogTitleId = `edit-${resource || 'record'}-comment-dialog-title`
  const inputId = `edit-${resource || 'record'}-comment-input`

  if (permissions !== 'admin') {
    return null
  }

  return (
    <>
      <RaButton
        onClick={handleOpen}
        className={clsx(classes.button, className)}
        label={buttonLabel}
        disabled={!selectedIds?.length}
      >
        <CommentIcon />
      </RaButton>
      <Dialog
        open={open}
        onClose={handleClose}
        aria-labelledby={dialogTitleId}
        fullWidth
        maxWidth="sm"
      >
        <form onSubmit={handleSubmit}>
          <DialogTitle id={dialogTitleId}>
            {title}
          </DialogTitle>
          <DialogContent>
            <DialogContentText>{description}</DialogContentText>
            <TextField
              autoFocus
              margin="dense"
              id={inputId}
              type="text"
              fullWidth
              multiline
              rows={3}
              variant="outlined"
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              disabled={saving}
            />
          </DialogContent>
          <DialogActions>
            <MuiButton onClick={handleClose} disabled={saving}>
              {translate('ra.action.cancel')}
            </MuiButton>
            <MuiButton color="primary" type="submit" disabled={saving}>
              {translate('ra.action.save')}
            </MuiButton>
          </DialogActions>
        </form>
      </Dialog>
    </>
  )
}

EditSongCommentButton.propTypes = {
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  className: PropTypes.string,
}

EditSongCommentButton.defaultProps = {
  selectedIds: [],
  className: undefined,
}

export default EditSongCommentButton
