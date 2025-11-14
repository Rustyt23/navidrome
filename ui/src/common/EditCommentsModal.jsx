import React, { useState, useCallback } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  useDataProvider,
  useListContext,
  useNotify,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import {
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Button,
  CircularProgress,
} from '@material-ui/core'

const getInitialComment = (selectedIds, records) => {
  if (!selectedIds || selectedIds.length === 0) {
    return ''
  }

  const comments = selectedIds.map((id) => {
    const record = records?.[id]
    return record?.comment ?? ''
  })

  const first = comments[0]
  const allEqual = comments.every((value) => value === first)
  return allEqual ? first : ''
}

export const EditCommentsModal = ({
  resource,
  selectedIds,
  songIds,
  updateResource,
  className,
}) => {
  const translate = useTranslate()
  const notify = useNotify()
  const unselectAll = useUnselectAll()
  const dataProvider = useDataProvider()
  const { data, refetch } = useListContext()

  const [open, setOpen] = useState(false)
  const [comment, setComment] = useState('')
  const [saving, setSaving] = useState(false)

  const targetIds = songIds && songIds.length ? songIds : selectedIds
  const selectedCount = targetIds?.length ?? 0

  const handleOpen = useCallback(() => {
    setComment(getInitialComment(selectedIds, data))
    setOpen(true)
  }, [selectedIds, data])

  const handleClose = useCallback(() => {
    if (saving) {
      return
    }
    setOpen(false)
    setComment('')
  }, [saving])

  const handleSave = useCallback(async () => {
    if (!selectedCount) {
      return
    }

    setSaving(true)
    try {
      await Promise.all(
        targetIds.map((id) =>
          dataProvider.update(updateResource, {
            id,
            data: { comment },
          }),
        ),
      )

      notify('resources.song.notifications.commentsUpdated', 'info', {
        smart_count: selectedCount,
      })

      if (typeof refetch === 'function') {
        refetch()
      }

      unselectAll(resource)
      setOpen(false)
    } catch (error) {
      const message = error?.body?.message || error?.message || 'ra.page.error'
      notify(message, 'warning')
    } finally {
      setSaving(false)
    }
  }, [
    comment,
    dataProvider,
    notify,
    refetch,
    resource,
    selectedCount,
    targetIds,
    unselectAll,
    updateResource,
  ])

  const openLabel = translate('resources.song.actions.editComments')

  return (
    <>
      <RaButton
        aria-label={openLabel}
        className={className}
        disabled={!selectedCount}
        label={openLabel}
        onClick={handleOpen}
      />
      <Dialog
        open={open}
        onClose={handleClose}
        aria-labelledby="edit-comments-title"
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle id="edit-comments-title">
          {translate('resources.song.dialogs.editComments.title', {
            smart_count: selectedCount,
          })}
        </DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            multiline
            minRows={4}
            variant="outlined"
            margin="dense"
            label={translate('resources.song.dialogs.editComments.commentLabel')}
            value={comment}
            onChange={(event) => setComment(event.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose} disabled={saving}>
            {translate('resources.song.dialogs.editComments.cancel')}
          </Button>
          <Button onClick={handleSave} color="primary" disabled={saving}>
            {saving ? <CircularProgress size={18} /> : translate('resources.song.dialogs.editComments.save')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}

EditCommentsModal.propTypes = {
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  songIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  updateResource: PropTypes.string,
  className: PropTypes.string,
}

EditCommentsModal.defaultProps = {
  selectedIds: [],
  songIds: undefined,
  updateResource: 'song',
  className: undefined,
}

export default EditCommentsModal
