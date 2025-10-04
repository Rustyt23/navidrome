import React, { cloneElement, useCallback, useMemo, useRef, useState } from 'react'
import {
  sanitizeListRestProps,
  TopToolbar,
  useDataProvider,
  useListContext,
  useNotify,
  useRefresh,
  useTranslate,
} from 'react-admin'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  makeStyles,
} from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'
import PropTypes from 'prop-types'

import httpClient from '../dataProvider/httpClient'
import { REST_URL } from '../consts'

const useStyles = makeStyles((theme) => ({
  toolbar: { display: 'flex', gap: theme.spacing(1), alignItems: 'center' },
  hiddenInput: { display: 'none' },
}))

const isFolderRecord = (record) => {
  if (!record) return false
  const type = record.type || record?.Type
  return type === 'folder' || type === 'discoveryFolder'
}

const normalizeParentId = (value) => {
  if (value === undefined) return undefined
  if (value === null || value === '') return null
  return value
}

const DiscoveryListActions = ({ className, filters, parentId, ...rest }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const dataProvider = useDataProvider()
  const refresh = useRefresh()
  const { selectedIds = [], data = {} } = useListContext() || {}
  const [createOpen, setCreateOpen] = useState(false)
  const [folderName, setFolderName] = useState('')
  const [saving, setSaving] = useState(false)
  const fileInputRef = useRef(null)
  const targetParentId = useMemo(() => {
    if (parentId !== undefined) {
      return normalizeParentId(parentId)
    }
    const selectedFolderId = selectedIds.find((id) => isFolderRecord(data?.[id]))
    return normalizeParentId(selectedFolderId)
  }, [data, parentId, selectedIds])

  const handleDialogClose = () => {
    setCreateOpen(false)
    setFolderName('')
    setSaving(false)
  }

  const handleCreate = useCallback(
    async (event) => {
      event.preventDefault()
      if (!folderName.trim()) {
        return
      }
      setSaving(true)
      try {
        await dataProvider.create('discoveryFolder', {
          data: {
            name: folderName.trim(),
            public: true,
            parentId: targetParentId ?? null,
          },
        })
        notify('ra.notification.created', 'info', { smart_count: 1 })
        refresh()
        handleDialogClose()
      } catch (error) {
        if (error?.status === 409) {
          notify('message.folderExists', 'warning')
        } else {
          notify('ra.page.error', 'warning')
        }
        setSaving(false)
      }
    },
    [dataProvider, folderName, notify, refresh, targetParentId],
  )

  const ensureFolderIdForUpload = useCallback(() => {
    if (targetParentId) {
      return targetParentId
    }
    notify('message.selectDiscoveryFolder', 'warning')
    return null
  }, [notify, targetParentId])

  const handleUploadClick = () => {
    if (!ensureFolderIdForUpload()) {
      return
    }
    fileInputRef.current?.click()
  }

  const handleFilesSelected = useCallback(
    async (event) => {
      const files = Array.from(event.target.files || [])
      event.target.value = ''
      if (!files.length) return
      const folderId = ensureFolderIdForUpload()
      if (!folderId) {
        return
      }
      const formData = new FormData()
      files.forEach((file) => formData.append('files', file))
      try {
        await httpClient(`${REST_URL}/discovery/folder/${folderId}/upload`, {
          method: 'POST',
          body: formData,
          headers: new Headers(),
        })
        notify('message.uploadSuccess', 'info', { smart_count: files.length })
        refresh()
      } catch (error) {
        notify('ra.page.error', 'warning')
      }
    },
    [ensureFolderIdForUpload, notify, refresh],
  )

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {filters && cloneElement(filters, { context: 'button' })}
      <div className={classes.toolbar}>
        <Button
          variant="contained"
          color="primary"
          startIcon={<AddIcon />}
          onClick={() => setCreateOpen(true)}
        >
          {translate('ra.action.create')}
        </Button>
        <Button
          variant="contained"
          color="primary"
          startIcon={<CloudUploadIcon />}
          onClick={handleUploadClick}
        >
          {translate('ra.action.upload') || 'Upload'}
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept="audio/*"
          className={classes.hiddenInput}
          onChange={handleFilesSelected}
        />
      </div>
      <Dialog open={createOpen} onClose={handleDialogClose} aria-labelledby="create-discovery-folder">
        <DialogTitle id="create-discovery-folder">
          {translate('resources.discovery.actions.createFolder')}
        </DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            fullWidth
            label={translate('resources.discovery.fields.name') || translate('resources.discoveryFolder.fields.name') || translate('ra.field.name')}
            value={folderName}
            onChange={(e) => setFolderName(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDialogClose} color="primary">
            {translate('ra.action.cancel')}
          </Button>
          <Button onClick={handleCreate} color="primary" disabled={!folderName.trim() || saving}>
            {translate('ra.action.save')}
          </Button>
        </DialogActions>
      </Dialog>
    </TopToolbar>
  )
}

DiscoveryListActions.propTypes = {
  className: PropTypes.string,
  filters: PropTypes.element,
  parentId: PropTypes.oneOfType([PropTypes.string, PropTypes.number, PropTypes.oneOf([null])]),
}

DiscoveryListActions.defaultProps = {
  parentId: undefined,
}

export default DiscoveryListActions
