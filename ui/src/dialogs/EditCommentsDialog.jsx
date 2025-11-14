import React, { useEffect, useMemo, useState } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  useDataProvider,
  useNotify,
  useTranslate,
} from 'react-admin'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@material-ui/core'
import { closeEditComments } from '../actions'

export const EditCommentsDialog = () => {
  const { open, targetIds = [], initialComment = '', onSuccess } =
    useSelector((state) => state.editCommentsDialog)
  const dispatch = useDispatch()
  const translate = useTranslate()
  const notify = useNotify()
  const dataProvider = useDataProvider()
  const [comment, setComment] = useState(initialComment || '')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) {
      setComment(initialComment || '')
    } else {
      setComment('')
    }
  }, [open, initialComment])

  const handleClose = (event) => {
    event?.stopPropagation?.()
    if (saving) {
      return
    }
    dispatch(closeEditComments())
  }

  const payloadComment = useMemo(() => {
    const trimmed = comment.trim()
    return trimmed.length === 0 ? null : comment
  }, [comment])

  const handleSubmit = async (event) => {
    event?.preventDefault?.()
    if (!targetIds.length) {
      dispatch(closeEditComments())
      return
    }
    setSaving(true)
    try {
      await dataProvider.setSongComments({
        ids: targetIds,
        comment: payloadComment,
      })
      notify('message.commentsUpdated', {
        type: 'info',
        messageArgs: { smart_count: targetIds.length },
      })
      if (typeof onSuccess === 'function') {
        onSuccess()
      }
      dispatch(closeEditComments())
    } catch (error) {
      notify('ra.notification.http_error', { type: 'warning' })
    } finally {
      setSaving(false)
    }
  }

  const selectionMessage = useMemo(
    () =>
      translate('message.editCommentsDescription', {
        smart_count: targetIds.length,
      }),
    [targetIds.length, translate],
  )

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {translate('resources.song.actions.editComments')}
      </DialogTitle>
      <DialogContent>
        <Typography variant="body2" paragraph>
          {selectionMessage}
        </Typography>
        <TextField
          autoFocus
          fullWidth
          multiline
          minRows={3}
          variant="outlined"
          label={translate('resources.song.fields.comment')}
          value={comment}
          onChange={(event) => setComment(event.target.value)}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} color="primary" disabled={saving}>
          {translate('ra.action.cancel')}
        </Button>
        <Button
          onClick={handleSubmit}
          color="primary"
          disabled={saving || targetIds.length === 0}
        >
          {translate('ra.action.save')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
