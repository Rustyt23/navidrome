import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Breadcrumbs,
  Button,
  Card,
  CardContent,
  Link,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  makeStyles,
  TextField,
  Typography,
} from '@material-ui/core'
import FolderIcon from '@material-ui/icons/Folder'
import MusicNoteIcon from '@material-ui/icons/MusicNote'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'
import { Title, useNotify, useTranslate } from 'react-admin'
import httpClient from '../dataProvider/httpClient'

const useStyles = makeStyles((theme) => ({
  card: {
    marginTop: theme.spacing(2),
  },
  breadcrumbs: {
    marginBottom: theme.spacing(2),
  },
  actions: {
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    marginBottom: theme.spacing(2),
    '& > *': {
      marginRight: theme.spacing(2),
      marginBottom: theme.spacing(1),
    },
  },
  newFolderForm: {
    display: 'flex',
    alignItems: 'center',
    '& > *': {
      marginRight: theme.spacing(1),
    },
  },
  fileInput: {
    display: 'none',
  },
}))

const DiscoveryBrowser = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const classes = useStyles()
  const [path, setPath] = useState('')
  const [refreshToken, setRefreshToken] = useState(0)
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [folderName, setFolderName] = useState('')
  const fileInputRef = useRef(null)

  const currentTitle = useMemo(
    () => translate('menu.discovery', { _: 'Discovery' }),
    [translate],
  )

  useEffect(() => {
    let active = true
    setLoading(true)
    const query = path ? `?path=${encodeURIComponent(path)}` : ''
    httpClient(`/api/discoveryfs/list${query}`)
      .then(({ json }) => {
        if (!active) {
          return
        }
        setItems(json?.items || [])
      })
      .catch((error) => {
        if (!active) {
          return
        }
        const message =
          error?.body?.error ||
          error?.message ||
          translate('resources.discovery.notifications.load_error', {
            _: 'Unable to load discovery items',
          })
        notify(message, 'warning')
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })
    return () => {
      active = false
    }
  }, [path, refreshToken, notify, translate])

  const pathSegments = path ? path.split('/') : []

  const handleNavigate = (index) => {
    if (index < 0) {
      setPath('')
      return
    }
    const nextPath = pathSegments.slice(0, index + 1).join('/')
    setPath(nextPath)
  }

  const handleFolderClick = (name) => {
    setPath((current) => (current ? `${current}/${name}` : name))
  }

  const handleCreateFolder = useCallback(
    (event) => {
      event.preventDefault()
      const trimmed = folderName.trim()
      if (!trimmed) {
        return
      }
      httpClient('/api/discoveryfs/folder', {
        method: 'POST',
        body: JSON.stringify({ path, name: trimmed }),
        headers: new Headers({
          Accept: 'application/json',
          'Content-Type': 'application/json',
        }),
      })
        .then(() => {
          setFolderName('')
          setRefreshToken((value) => value + 1)
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
    },
    [folderName, path, notify, translate],
  )

  const handleUploadClick = useCallback(() => {
    if (fileInputRef.current) {
      fileInputRef.current.click()
    }
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
          setRefreshToken((value) => value + 1)
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
    [path, notify, translate],
  )

  return (
    <Card className={classes.card}>
      <Title title={`MusicMatters - ${currentTitle}`} />
      <CardContent>
        <Typography variant="h5" gutterBottom>
          {currentTitle}
        </Typography>
        <Breadcrumbs aria-label="breadcrumb" className={classes.breadcrumbs}>
          <Link color="inherit" component="button" onClick={() => handleNavigate(-1)}>
            {currentTitle}
          </Link>
          {pathSegments.map((segment, index) => (
            <Link
              key={`${segment}-${index}`}
              color="inherit"
              component="button"
              onClick={() => handleNavigate(index)}
            >
              {segment}
            </Link>
          ))}
        </Breadcrumbs>
        <div className={classes.actions}>
          <form className={classes.newFolderForm} onSubmit={handleCreateFolder}>
            <TextField
              label={translate('resources.discovery.new_folder', { _: 'Folder name' })}
              value={folderName}
              onChange={(event) => setFolderName(event.target.value)}
              size="small"
              variant="outlined"
            />
            <Button type="submit" variant="contained" color="primary">
              {translate('ra.action.create', { _: 'Create' })}
            </Button>
          </form>
          <div>
            <input
              accept=".mp3,.m4a,.flac,.wav,.ogg"
              className={classes.fileInput}
              multiple
              onChange={handleFileChange}
              ref={fileInputRef}
              type="file"
            />
            <Button
              variant="contained"
              color="default"
              startIcon={<CloudUploadIcon />}
              onClick={handleUploadClick}
            >
              {translate('ra.action.upload', { _: 'Upload' })}
            </Button>
          </div>
        </div>
        {loading ? (
          <Typography variant="body2">
            {translate('ra.page.loading', { _: 'Loading' })}
          </Typography>
        ) : items.length === 0 ? (
          <Typography variant="body2">
            {translate('resources.discovery.empty', { _: 'This folder is empty.' })}
          </Typography>
        ) : (
          <List dense>
            {items.map((item) => (
              <ListItem
                key={`${item.type}-${item.name}`}
                button={item.type === 'folder'}
                onClick={
                  item.type === 'folder'
                    ? () => handleFolderClick(item.name)
                    : undefined
                }
              >
                <ListItemIcon>
                  {item.type === 'folder' ? <FolderIcon /> : <MusicNoteIcon />}
                </ListItemIcon>
                <ListItemText primary={item.name} />
              </ListItem>
            ))}
          </List>
        )}
      </CardContent>
    </Card>
  )
}

export default DiscoveryBrowser

