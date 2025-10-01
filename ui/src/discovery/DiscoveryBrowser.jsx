import React, {
  cloneElement,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useHistory, useLocation } from 'react-router-dom'
import {
  Breadcrumbs,
  Link as MuiLink,
  Typography,
  makeStyles,
  List as MuiList,
  ListItem,
  ListItemIcon,
  ListItemText,
  Switch,
  Button as MuiButton,
} from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'
import MusicNoteIcon from '@material-ui/icons/MusicNote'
import EditIcon from '@material-ui/icons/Edit'
import { RiFolder3Fill } from 'react-icons/ri'
import {
  DateField,
  Filter,
  SearchInput,
  TextField,
  Title,
  TopToolbar,
  sanitizeListRestProps,
  useNotify,
  useRefresh,
  useRecordContext,
  useTranslate,
} from 'react-admin'
import { List } from '../common'
import DiscoveryDataGrid from './DiscoveryDataGrid'
import httpClient from '../dataProvider/httpClient'
import { emitDiscoveryChanged, addDiscoveryChangedListener } from './events'

const AUDIO_ACCEPT = '.mp3,.m4a,.flac,.wav,.ogg'

const useStyles = makeStyles((theme) => ({
  breadcrumbs: {
    marginBottom: theme.spacing(2),
    '& a': {
      cursor: 'pointer',
      color: theme.palette.text.secondary,
    },
  },
  filesSection: {
    marginTop: theme.spacing(2),
  },
  filesTitle: {
    fontWeight: theme.typography.fontWeightMedium,
    marginBottom: theme.spacing(1),
  },
}))

const DiscoveryFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="q" alwaysOn />
  </Filter>
)

const DiscoveryTypeIconField = () => {
  const record = useRecordContext()
  if (!record) return null
  const isFolder = record.type === 'folder'
  const Icon = isFolder ? RiFolder3Fill : MusicNoteIcon
  const color = isFolder ? '#1976d2' : '#9c27b0'
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        width: '100%',
      }}
      aria-label={isFolder ? 'Folder' : 'File'}
    >
      <Icon style={{ fontSize: 18, color }} />
    </div>
  )
}

const DiscoveryPublicField = () => <Switch size="small" color="primary" disabled />

const DiscoveryEditButton = () => {
  const record = useRecordContext()
  const history = useHistory()

  const handleClick = useCallback(
    (event) => {
      event.stopPropagation()
      if (!record || record.type !== 'folder') return
      const target = record.id || ''
      const query = target ? `?path=${encodeURIComponent(target)}` : ''
      history.push(`/discovery${query}`)
    },
    [history, record],
  )

  const disabled = !record || record.type !== 'folder'

  return (
    <MuiButton
      onClick={handleClick}
      disabled={disabled}
      size="small"
      style={{ minWidth: 0, padding: '0px 0px', fontSize: 12 }}
    >
      <EditIcon fontSize="small" style={{ fontSize: 14 }} />
    </MuiButton>
  )
}

const DiscoveryListActions = ({ className, onItemsChanged, ...rest }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const fileInputRef = useRef(null)
  const path = rest?.filterValues?.path ?? ''

  const emitChange = useCallback(() => {
    emitDiscoveryChanged({ path })
    onItemsChanged?.()
  }, [onItemsChanged, path])

  const createLabel = useMemo(
    () => `+ ${translate('ra.action.create', { _: 'Create' }).toUpperCase()}`,
    [translate],
  )

  const uploadLabel = useMemo(
    () => translate('resources.discovery.actions.upload', { _: 'Upload' }).toUpperCase(),
    [translate],
  )

  const handleCreate = useCallback(() => {
    const value = window.prompt(
      translate('resources.discovery.new_folder', { _: 'Folder name' }),
    )
    const trimmed = value ? value.trim() : ''
    if (!trimmed) return

    httpClient('/api/discoveryfs/folder', {
      method: 'POST',
      body: JSON.stringify({ path, name: trimmed }),
      headers: new Headers({
        Accept: 'application/json',
        'Content-Type': 'application/json',
      }),
    })
      .then(() => {
        refresh()
        emitChange()
      })
      .catch((error) => {
        const message =
          error?.body?.error ||
          error?.message ||
          translate('resources.discovery.notifications.create_error', {
            _: 'Unable to create folder',
          })
        notify(message, 'warning')
      })
  }, [emitChange, notify, path, refresh, translate])

  const handleUploadClick = useCallback(() => {
    fileInputRef.current?.click()
  }, [])

  const handleFileChange = useCallback(
    (event) => {
      const input = event.target
      const { files } = input
      if (!files || files.length === 0) {
        return
      }
      const formData = new FormData()
      Array.from(files).forEach((file) => {
        formData.append('files', file)
      })
      const query = path ? `?path=${encodeURIComponent(path)}` : ''
      httpClient(`/api/discoveryfs/upload${query}`, {
        method: 'POST',
        body: formData,
      })
        .then(() => {
          refresh()
          emitChange()
        })
        .catch((error) => {
          const message =
            error?.body?.error ||
            error?.message ||
            translate('resources.discovery.notifications.upload_error', {
              _: 'Unable to upload files',
            })
          notify(message, 'warning')
        })
        .finally(() => {
          input.value = null
        })
    },
    [emitChange, notify, path, refresh, translate],
  )

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {rest.filters ? cloneElement(rest.filters, { context: 'button' }) : null}
      <MuiButton
        color="primary"
        variant="contained"
        onClick={handleCreate}
        startIcon={<AddIcon />}
        size="small"
      >
        {createLabel}
      </MuiButton>
      <input
        ref={fileInputRef}
        type="file"
        accept={AUDIO_ACCEPT}
        multiple
        style={{ display: 'none' }}
        onChange={handleFileChange}
      />
      <MuiButton
        color="default"
        variant="contained"
        onClick={handleUploadClick}
        startIcon={<CloudUploadIcon />}
        size="small"
      >
        {uploadLabel}
      </MuiButton>
    </TopToolbar>
  )
}

const EmptyDiscovery = () => {
  const translate = useTranslate()
  return (
    <Typography variant="body2" style={{ padding: 16 }}>
      {translate('resources.discovery.empty', { _: 'This folder is empty.' })}
    </Typography>
  )
}

const DiscoveryFilesSection = ({ files, loading }) => {
  const translate = useTranslate()

  if (loading) {
    return (
      <Typography variant="body2" color="textSecondary">
        {translate('ra.page.loading', { _: 'Loading' })}
      </Typography>
    )
  }

  if (!files.length) {
    return (
      <Typography variant="body2" color="textSecondary">
        {translate('resources.discovery.files_empty', { _: 'No files in this folder.' })}
      </Typography>
    )
  }

  return (
    <MuiList dense>
      {files.map((file) => (
        <ListItem key={file.path}>
          <ListItemIcon>
            <MusicNoteIcon />
          </ListItemIcon>
          <ListItemText primary={file.name} />
        </ListItem>
      ))}
    </MuiList>
  )
}

const DiscoveryBrowser = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const history = useHistory()
  const location = useLocation()
  const classes = useStyles()

  const path = useMemo(() => {
    const params = new URLSearchParams(location.search)
    const raw = params.get('path') || ''
    return raw.replace(/^\/+/, '')
  }, [location.search])

  const [files, setFiles] = useState([])
  const [filesLoading, setFilesLoading] = useState(false)
  const [filesVersion, setFilesVersion] = useState(0)

  const currentTitle = useMemo(
    () => translate('menu.discovery', { _: 'Discovery' }),
    [translate],
  )

  const pathSegments = useMemo(() => (path ? path.split('/') : []), [path])

  const fetchFiles = useCallback(async () => {
    setFilesLoading(true)
    try {
      const query = path ? `?path=${encodeURIComponent(path)}` : ''
      const { json } = await httpClient(`/api/discoveryfs/list${query}`)
      const items = Array.isArray(json?.items) ? json.items : []
      const nextFiles = items
        .filter((item) => item?.type === 'file')
        .map((item) => ({
          name: item.name,
          path: path ? `${path}/${item.name}` : item.name,
        }))
      setFiles(nextFiles)
    } catch (error) {
      const message =
        error?.body?.error ||
        error?.message ||
        translate('resources.discovery.notifications.load_error', {
          _: 'Unable to load discovery items',
        })
      notify(message, 'warning')
    } finally {
      setFilesLoading(false)
    }
  }, [notify, path, translate])

  useEffect(() => {
    fetchFiles()
  }, [fetchFiles, filesVersion])

  useEffect(() => {
    return addDiscoveryChangedListener(() => setFilesVersion((value) => value + 1))
  }, [])

  const navigateTo = useCallback(
    (nextPath) => {
      const query = nextPath ? `?path=${encodeURIComponent(nextPath)}` : ''
      history.push(`/discovery${query}`)
    },
    [history],
  )

  const handleBreadcrumbClick = useCallback(
    (index) => {
      if (index < 0) {
        navigateTo('')
        return
      }
      const next = pathSegments.slice(0, index + 1).join('/')
      navigateTo(next)
    },
    [navigateTo, pathSegments],
  )

  const rowClick = useCallback((id, record) => {
    if (record?.type !== 'folder') {
      return false
    }
    const target = id || ''
    return `/discovery${target ? `?path=${encodeURIComponent(target)}` : ''}`
  }, [])

  return (
    <div>
      <Title title={`Navidrome - ${currentTitle}`} />
      <Typography variant="h5" gutterBottom>
        {currentTitle}
      </Typography>
      <Breadcrumbs aria-label="breadcrumb" className={classes.breadcrumbs}>
        <MuiLink color="inherit" onClick={() => handleBreadcrumbClick(-1)}>
          {currentTitle}
        </MuiLink>
        {pathSegments.map((segment, index) => {
          const isLast = index === pathSegments.length - 1
          if (isLast) {
            return (
              <Typography color="textPrimary" key={`${segment}-${index}`}>
                {segment}
              </Typography>
            )
          }
          return (
            <MuiLink
              color="inherit"
              key={`${segment}-${index}`}
              onClick={() => handleBreadcrumbClick(index)}
            >
              {segment}
            </MuiLink>
          )
        })}
      </Breadcrumbs>
      <List
        key={path}
        resource="discoveryFolder"
        basePath="/discovery"
        sort={{ field: 'name', order: 'ASC' }}
        filter={{ path }}
        filters={<DiscoveryFilter />}
        actions={<DiscoveryListActions onItemsChanged={() => setFilesVersion((value) => value + 1)} />}
        bulkActionButtons={false}
        empty={<EmptyDiscovery />}
        exporter={false}
      >
        <DiscoveryDataGrid rowClick={rowClick}>
          <DiscoveryTypeIconField label={false} />
          <TextField source="name" />
          <TextField source="ownerName" />
          <DateField source="updatedAt" />
          <DiscoveryPublicField />
          <DiscoveryEditButton />
        </DiscoveryDataGrid>
      </List>
      <div className={classes.filesSection}>
        <Typography variant="subtitle1" className={classes.filesTitle}>
          {translate('resources.discovery.files', { _: 'Files' })}
        </Typography>
        <DiscoveryFilesSection files={files} loading={filesLoading} />
      </div>
    </div>
  )
}

export default DiscoveryBrowser
