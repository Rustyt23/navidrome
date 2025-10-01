import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useHistory, useLocation } from 'react-router-dom'
import {
  Breadcrumbs,
  Button as MuiButton,
  Link as MuiLink,
  List as MuiList,
  ListItem,
  ListItemIcon,
  ListItemText,
  Typography,
  makeStyles,
} from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'
import FolderIcon from '@material-ui/icons/Folder'
import MusicNoteIcon from '@material-ui/icons/MusicNote'
import { Title, useNotify, useTranslate } from 'react-admin'
import httpClient from '../dataProvider/httpClient'
import { emitDiscoveryChanged, addDiscoveryChangedListener } from './events'

const AUDIO_ACCEPT = '.mp3,.m4a,.flac,.wav,.ogg'

const useStyles = makeStyles((theme) => ({
  container: {
    paddingBottom: theme.spacing(2),
  },
  breadcrumbs: {
    marginBottom: theme.spacing(2),
    '& a': {
      cursor: 'pointer',
      color: theme.palette.text.secondary,
    },
  },
  actions: {
    display: 'flex',
    alignItems: 'center',
    marginBottom: theme.spacing(2),
    '& > *:not(:first-child)': {
      marginLeft: theme.spacing(1),
    },
  },
  section: {
    marginTop: theme.spacing(2),
  },
  empty: {
    color: theme.palette.text.secondary,
  },
  uploadInput: {
    display: 'none',
  },
}))

const DiscoveryBrowser = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const history = useHistory()
  const location = useLocation()
  const fileInputRef = useRef(null)

  const [currentPath, setCurrentPath] = useState('')
  const [folders, setFolders] = useState([])
  const [files, setFiles] = useState([])
  const [loading, setLoading] = useState(false)
  const [refreshToken, setRefreshToken] = useState(0)

  const locationPath = useMemo(() => {
    const params = new URLSearchParams(location.search)
    const raw = params.get('path') || ''
    return raw.replace(/^\/+/, '')
  }, [location.search])

  useEffect(() => {
    setCurrentPath((prev) => (prev !== locationPath ? locationPath : prev))
  }, [locationPath])

  const currentTitle = useMemo(
    () => translate('menu.discovery', { _: 'Discovery' }),
    [translate],
  )

  const pathSegments = useMemo(
    () => (currentPath ? currentPath.split('/') : []),
    [currentPath],
  )

  const fetchItems = useCallback(async () => {
    setLoading(true)
    try {
      const query = currentPath
        ? `?path=${encodeURIComponent(currentPath)}`
        : ''
      const { json } = await httpClient(`/api/discoveryfs/list${query}`)
      const nextFolders = Array.isArray(json?.folders) ? json.folders : []
      const nextFiles = Array.isArray(json?.files) ? json.files : []
      setFolders(nextFolders)
      setFiles(nextFiles)
    } catch (error) {
      setFolders([])
      setFiles([])
      const message =
        error?.body?.error ||
        error?.message ||
        translate('resources.discovery.notifications.load_error', {
          _: 'Unable to load discovery items',
        })
      notify(message, 'warning')
      if (error?.status === 404) {
        history.replace('/discovery')
        setCurrentPath('')
      }
    } finally {
      setLoading(false)
    }
  }, [currentPath, history, notify, translate])

  useEffect(() => {
    fetchItems()
  }, [fetchItems, refreshToken])

  useEffect(() => {
    return addDiscoveryChangedListener(() =>
      setRefreshToken((value) => value + 1),
    )
  }, [])

  const navigateTo = useCallback(
    (nextPath) => {
      const normalized = nextPath ? nextPath.replace(/^\/+/, '') : ''
      const query = normalized
        ? `?path=${encodeURIComponent(normalized)}`
        : ''
      history.push(`/discovery${query}`)
      setCurrentPath(normalized)
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

  const handleFolderClick = useCallback(
    (folderPath) => {
      navigateTo(folderPath)
    },
    [navigateTo],
  )

  const handleCreate = useCallback(() => {
    const value = window.prompt(
      translate('resources.discovery.new_folder', { _: 'Folder name' }),
    )
    const trimmed = value ? value.trim() : ''
    if (!trimmed) {
      return
    }

    httpClient('/api/discoveryfs/folder', {
      method: 'POST',
      body: JSON.stringify({ path: currentPath, name: trimmed }),
      headers: new Headers({
        Accept: 'application/json',
        'Content-Type': 'application/json',
      }),
    })
      .then(() => {
        setRefreshToken((value) => value + 1)
        emitDiscoveryChanged({ path: currentPath })
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
  }, [currentPath, notify, translate])

  const handleUploadClick = useCallback(() => {
    fileInputRef.current?.click()
  }, [])

  const handleFileChange = useCallback(
    (event) => {
      const input = event.target
      const { files: selectedFiles } = input
      if (!selectedFiles || selectedFiles.length === 0) {
        return
      }

      const formData = new FormData()
      Array.from(selectedFiles).forEach((file) => {
        formData.append('files', file)
      })

      const query = currentPath
        ? `?path=${encodeURIComponent(currentPath)}`
        : ''
      httpClient(`/api/discoveryfs/upload${query}`, {
        method: 'POST',
        body: formData,
      })
        .then(() => {
          setRefreshToken((value) => value + 1)
          emitDiscoveryChanged({ path: currentPath })
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
    [currentPath, notify, translate],
  )

  return (
    <div className={classes.container}>
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
      <div className={classes.actions}>
        <MuiButton
          color="primary"
          variant="contained"
          onClick={handleCreate}
          startIcon={<AddIcon />}
          size="small"
        >
          {`+ ${translate('ra.action.create', { _: 'Create' }).toUpperCase()}`}
        </MuiButton>
        <input
          ref={fileInputRef}
          type="file"
          accept={AUDIO_ACCEPT}
          multiple
          className={classes.uploadInput}
          onChange={handleFileChange}
        />
        <MuiButton
          color="default"
          variant="contained"
          onClick={handleUploadClick}
          startIcon={<CloudUploadIcon />}
          size="small"
        >
          {translate('resources.discovery.actions.upload', {
            _: 'Upload',
          }).toUpperCase()}
        </MuiButton>
      </div>
      <div className={classes.section}>
        <Typography variant="subtitle1">
          {translate('resources.folder.name', { smart_count: 2, _: 'Folders' })}
        </Typography>
        {loading ? (
          <Typography variant="body2" color="textSecondary">
            {translate('ra.page.loading', { _: 'Loading' })}
          </Typography>
        ) : folders.length === 0 ? (
          <Typography variant="body2" className={classes.empty}>
            {translate('resources.discovery.empty', { _: 'This folder is empty.' })}
          </Typography>
        ) : (
          <MuiList dense>
            {folders.map((folder) => (
              <ListItem
                button
                onClick={() => handleFolderClick(folder.path)}
                key={folder.path}
              >
                <ListItemIcon>
                  <FolderIcon />
                </ListItemIcon>
                <ListItemText primary={folder.name} />
              </ListItem>
            ))}
          </MuiList>
        )}
      </div>
      <div className={classes.section}>
        <Typography variant="subtitle1">
          {translate('resources.discovery.files', { _: 'Files' })}
        </Typography>
        {loading ? (
          <Typography variant="body2" color="textSecondary">
            {translate('ra.page.loading', { _: 'Loading' })}
          </Typography>
        ) : files.length === 0 ? (
          <Typography variant="body2" className={classes.empty}>
            {translate('resources.discovery.files_empty', {
              _: 'No files in this folder.',
            })}
          </Typography>
        ) : (
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
        )}
      </div>
    </div>
  )
}

export default DiscoveryBrowser
