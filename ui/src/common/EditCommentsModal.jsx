import React, { useCallback, useEffect, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Button as MuiButton,
  TextField as MuiTextField,
} from '@material-ui/core'
import {
  Button,
  useDataProvider,
  useListContext,
  useNotify,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import RateReviewIcon from '@material-ui/icons/RateReview'

const sanitizeIds = (ids) => {
  if (!Array.isArray(ids)) {
    return []
  }
  return Array.from(new Set(ids.filter((id) => id !== undefined && id !== null)))
}

export const EditCommentsButton = ({
  resource,
  selectedIds = [],
  className,
  getTargetIds,
  selectionResource,
  updateResource = 'song',
  onUnselectItems,
}) => {
  const translate = useTranslate()
  const notify = useNotify()
  const dataProvider = useDataProvider()
  const refresh = useRefresh()
  const listContext = useListContext()
  const unselectAll = useUnselectAll()

  const [open, setOpen] = useState(false)
  const [commentValue, setCommentValue] = useState('')
  const [saving, setSaving] = useState(false)

  const selectionKey = selectionResource || resource

  const selectedRecords = useMemo(() => {
    const data = listContext?.data || {}
    if (!Array.isArray(selectedIds)) {
      return []
    }
    return selectedIds
      .map((id) => data?.[id])
      .filter((record) => record !== undefined && record !== null)
  }, [listContext?.data, selectedIds])

  const resolvedIds = useMemo(() => {
    const data = listContext?.data || {}
    const ids = getTargetIds
      ? getTargetIds(selectedIds, data)
      : selectedIds
    return sanitizeIds(ids)
  }, [getTargetIds, listContext?.data, selectedIds])

  const recordLookup = useMemo(() => {
    const map = new Map()
    selectedRecords.forEach((record) => {
      if (!record) {
        return
      }
      if (record.id !== undefined && record.id !== null) {
        map.set(record.id, record)
      }
      if (
        record.mediaFileId !== undefined &&
        record.mediaFileId !== null &&
        !map.has(record.mediaFileId)
      ) {
        map.set(record.mediaFileId, record)
      }
    })
    return map
  }, [selectedRecords])

  const initialComment = useMemo(() => {
    if (!selectedRecords.length) {
      return ''
    }
    const comments = selectedRecords.map((record) => record?.comment ?? '')
    const unique = Array.from(new Set(comments))
    if (unique.length === 1) {
      return unique[0] ?? ''
    }
    return ''
  }, [selectedRecords])

  useEffect(() => {
    if (open) {
      setCommentValue(initialComment ?? '')
    }
  }, [initialComment, open])

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
    setCommentValue(initialComment ?? '')
  }, [initialComment, saving])

  const handleSave = useCallback(async () => {
    if (!resolvedIds.length) {
      notify(
        translate('resources.song.notifications.noSongsSelected'),
        {
          type: 'warning',
        },
      )
      setOpen(false)
      return
    }

    setSaving(true)
    try {
      const payload = { comment: commentValue ?? '' }
      const updatePromises = resolvedIds.map((id) =>
        dataProvider.update(updateResource, {
          id,
          data: payload,
          previousData: recordLookup.get(id),
        }),
      )

      await Promise.all(updatePromises)

      notify(
        translate('resources.song.notifications.commentsUpdated', {
          smart_count: resolvedIds.length,
        }),
        { type: 'info' },
      )

      if (typeof listContext?.refetch === 'function') {
        await listContext.refetch()
      } else {
        refresh()
      }

      if (typeof onUnselectItems === 'function') {
        onUnselectItems()
      } else if (selectionKey) {
        unselectAll(selectionKey)
      }

      setOpen(false)
    } catch (error) {
      const message =
        error?.body?.message ||
        error?.message ||
        translate('ra.notification.http_error')
      notify(message, { type: 'warning' })
    } finally {
      setSaving(false)
    }
  }, [
    commentValue,
    dataProvider,
    listContext,
    notify,
    onUnselectItems,
    refresh,
    recordLookup,
    resolvedIds,
    selectionKey,
    translate,
    unselectAll,
    updateResource,
  ])

  return (
    <>
      <Button
        className={className}
        onClick={handleOpen}
        label={translate('resources.song.actions.editComments')}
        disabled={!selectedIds?.length}
      >
        <RateReviewIcon />
      </Button>
      <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
        <DialogTitle>
          {translate('resources.song.dialogs.editComments.title', {
            smart_count: resolvedIds.length,
          })}
        </DialogTitle>
        <DialogContent>
          <MuiTextField
            multiline
            minRows={4}
            fullWidth
            variant="outlined"
            value={commentValue}
            onChange={(event) => setCommentValue(event.target.value)}
            label={translate(
              'resources.song.dialogs.editComments.commentLabel',
            )}
            autoFocus
          />
        </DialogContent>
        <DialogActions>
          <MuiButton onClick={handleClose} disabled={saving}>
            {translate('ra.action.cancel')}
          </MuiButton>
          <MuiButton color="primary" onClick={handleSave} disabled={saving}>
            {translate('ra.action.save')}
          </MuiButton>
        </DialogActions>
      </Dialog>
    </>
  )
}

EditCommentsButton.propTypes = {
  resource: PropTypes.string,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  className: PropTypes.string,
  getTargetIds: PropTypes.func,
  selectionResource: PropTypes.string,
  updateResource: PropTypes.string,
  onUnselectItems: PropTypes.func,
}

export default EditCommentsButton
