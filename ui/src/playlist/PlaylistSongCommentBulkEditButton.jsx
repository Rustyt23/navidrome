import React, { useCallback, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  useDataProvider,
  useListContext,
  useNotify,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import Button from '@material-ui/core/Button'
import Dialog from '@material-ui/core/Dialog'
import DialogTitle from '@material-ui/core/DialogTitle'
import DialogContent from '@material-ui/core/DialogContent'
import DialogActions from '@material-ui/core/DialogActions'
import TextField from '@material-ui/core/TextField'
import CommentIcon from '@material-ui/icons/Comment'

const getRecord = (data, id) =>
  Array.isArray(data) ? data.find((record) => record && record.id === id) : data?.[id]

const PlaylistSongCommentBulkEditButton = ({
  playlistId,
  resource,
  selectedIds,
  onUnselectItems,
  className,
}) => {
  const translate = useTranslate()
  const notify = useNotify()
  const dataProvider = useDataProvider()
  const unselectAll = useUnselectAll()
  const { data } = useListContext()
  const [open, setOpen] = useState(false)
  const [comment, setComment] = useState('')
  const [saving, setSaving] = useState(false)

  const editableIds = useMemo(() => {
    if (!Array.isArray(selectedIds)) {
      return []
    }
    return selectedIds.filter((id) => {
      const record = getRecord(data, id)
      if (!record) {
        return false
      }
      if (record.missing) {
        return false
      }
      return Boolean(record.mediaFileId)
    })
  }, [selectedIds, data])

  const computeInitialComment = useCallback(() => {
    if (!editableIds.length) {
      return ''
    }
    const firstRecord = getRecord(data, editableIds[0])
    const baseComment = firstRecord?.comment ?? ''
    const allSame = editableIds.every((id) => {
      const record = getRecord(data, id)
      return (record?.comment ?? '') === baseComment
    })
    return allSame ? baseComment : ''
  }, [data, editableIds])

  const handleOpen = useCallback(() => {
    if (!selectedIds?.length) {
      notify('resources.playlist.message.noEditableComment', { type: 'warning' })
      return
    }
    const initialComment = computeInitialComment()
    setComment(initialComment)
    setOpen(true)
  }, [selectedIds, computeInitialComment, notify])

  const handleClose = useCallback(() => {
    if (!saving) {
      setOpen(false)
    }
  }, [saving])

  const handleSubmit = useCallback(
    async (event) => {
      event.preventDefault()
      if (!playlistId || !editableIds.length) {
        notify('resources.playlist.message.noEditableComment', { type: 'warning' })
        return
      }
      setSaving(true)
      try {
        await dataProvider.updateMany('playlistTrack', {
          ids: editableIds,
          data: { comment },
          filter: { playlist_id: playlistId },
        })
        notify('resources.playlist.notifications.commentUpdated', {
          type: 'info',
          messageArgs: { smart_count: editableIds.length },
        })
        if (typeof onUnselectItems === 'function') {
          onUnselectItems()
        }
        unselectAll(resource || 'playlistTrack')
        setOpen(false)
      } catch (error) {
        const message =
          error?.body?.message || error?.message || 'ra.notification.http_error'
        notify(message, { type: 'warning' })
      } finally {
        setSaving(false)
      }
    },
    [comment, dataProvider, editableIds, notify, onUnselectItems, playlistId, resource, unselectAll],
  )

  const label = translate('resources.playlist.actions.bulkEditComment')

  return (
    <>
      <RaButton
        aria-label={label}
        onClick={handleOpen}
        className={className}
        label={label}
        disabled={!selectedIds?.length}
      >
        <CommentIcon />
      </RaButton>
      <Dialog
        open={open}
        onClose={handleClose}
        fullWidth
        maxWidth="sm"
        aria-labelledby="bulk-comment-dialog-title"
      >
        <form onSubmit={handleSubmit}>
          <DialogTitle id="bulk-comment-dialog-title">
            {translate('resources.playlist.dialog.bulkCommentTitle')}
          </DialogTitle>
          <DialogContent>
            <TextField
              label={translate('resources.playlist.dialog.bulkCommentLabel')}
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              fullWidth
              multiline
              rows={3}
              variant="filled"
              helperText={translate('resources.playlist.dialog.bulkCommentHelper')}
              autoFocus
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={handleClose} disabled={saving}>
              {translate('ra.action.cancel')}
            </Button>
            <Button
              type="submit"
              color="primary"
              disabled={saving || !editableIds.length}
            >
              {translate('resources.playlist.dialog.bulkCommentConfirm')}
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </>
  )
}

PlaylistSongCommentBulkEditButton.propTypes = {
  playlistId: PropTypes.string.isRequired,
  resource: PropTypes.string,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ).isRequired,
  onUnselectItems: PropTypes.func,
  className: PropTypes.string,
}

PlaylistSongCommentBulkEditButton.defaultProps = {
  resource: 'playlistTrack',
  onUnselectItems: undefined,
  className: undefined,
}

export default PlaylistSongCommentBulkEditButton
