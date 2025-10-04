import React, { cloneElement, useMemo, useRef, useState } from 'react'
import {
  sanitizeListRestProps,
  TopToolbar,
  useListContext,
  useNotify,
  useTranslate,
  useDataProvider,
  useRefresh,
  useResourceContext,
} from 'react-admin'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  makeStyles,
  useMediaQuery,
} from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'

import PropTypes from 'prop-types'

import { ToggleFieldsMenu } from '../common'
import { REST_URL } from '../consts'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  toolbar: { display: 'flex', gap: theme.spacing(1), alignItems: 'center' },
  hiddenInput: { display: 'none' },
}))

const normalizeFolderId = (value) => {
  if (value === undefined || value === null) return null
  if (value === '') return null
  return value
}

const DiscoveryListActions = ({ className, fallbackActions, ...rest }) => {
  const resource = useResourceContext() || 'discovery'
  if (resource !== 'discovery') {
    return fallbackActions ? cloneElement(fallbackActions, rest) : null
  }

  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const dataProvider = useDataProvider()
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  const fileInputRef = useRef(null)

  const { filterValues = {} } = useListContext() || {}
  const currentFolderId = useMemo(
    () =>
      normalizeFolderId(
        filterValues.parent_id ??
          filterValues.folder_id ??
          filterValues.folderId ??
          filterValues.discoveryFolderId,
      ),
    [filterValues],
  )

  const [createOpen, setCreateOpen] = useState(false)
  const [folderName, setFolderName] = useState('')
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)

  const closeDialog = () => {
    setCreateOpen(false)
    setFolderName('')
    setSaving(false)
  }

  const handleCreate = async (event) => {
    event.preventDefault()
    if (!folderName.trim()) return
    setSaving(true)
    try {
      const payload = {
        name: folderName.trim(),
        public: true,
      }
      if (currentFolderId) {
        payload.parentId = currentFolderId
      }
      await dataProvider.create('discoveryFolder', { data: payload })
      notify('ra.notification.created', 'info', { smart_count: 1 })
      refresh()
      closeDialog()
    } catch (error) {
      if (error?.status === 409) {
        notify('message.folderExists', 'warning')
      } else {
        notify('ra.page.error', 'warning')
      }
      setSaving(false)
    }
  }

  const ensureFolderSelected = () => {
    if (!currentFolderId) {
      notify('message.selectDiscoveryFolder', 'warning')
      return false
    }
    return true
  }

  const handleUploadClick = () => {
    if (!ensureFolderSelected()) {
      return
    }
    fileInputRef.current?.click()
  }

  const handleFilesSelected = async (event) => {
    const files = Array.from(event.target.files || [])
    event.target.value = ''
    if (!files.length) return
    if (!ensureFolderSelected()) {
      return
    }

    const formData = new FormData()
    files.forEach((file) => formData.append('files', file))

    try {
      setUploading(true)
      await httpClient(`${REST_URL}/discovery/folder/${currentFolderId}/upload`, {
        method: 'POST',
        body: formData,
        headers: new Headers(),
      })
      notify('message.uploadSuccess', 'info', { smart_count: files.length })
      refresh()
    } catch (error) {
      notify('ra.page.error', 'warning')
    } finally {
      setUploading(false)
    }
  }

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {rest.filters && cloneElement(rest.filters, { context: 'button' })}
      <div className={classes.toolbar}>
        <Button
          variant="contained"
          color="primary"
          startIcon={<AddIcon />}
          onClick={() => setCreateOpen(true)}
        >
          {translate('resources.discovery.actions.createFolder')}
        </Button>
        <Button
          variant="contained"
          color="primary"
          startIcon={<CloudUploadIcon />}
          onClick={handleUploadClick}
          disabled={uploading}
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
      {isNotSmall && <ToggleFieldsMenu resource={resource} />}
      <Dialog open={createOpen} onClose={closeDialog} aria-labelledby="create-discovery-folder-dialog">
        <DialogTitle id="create-discovery-folder-dialog">
          {translate('resources.discovery.actions.createFolder')}
        </DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            fullWidth
            label={
              translate('resources.discovery.fields.name') ||
              translate('resources.discoveryFolder.fields.name') ||
              translate('ra.field.name')
            }
            value={folderName}
            onChange={(e) => setFolderName(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDialog} color="primary">
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
  fallbackActions: PropTypes.element,
}

DiscoveryListActions.defaultProps = {
  fallbackActions: null,
}

export default DiscoveryListActions
