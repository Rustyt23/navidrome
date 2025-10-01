import React, { useCallback, useEffect, useRef, useState } from 'react'
import {
  Box,
  Breadcrumbs,
  Button,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Typography,
  makeStyles,
} from '@material-ui/core'
import FolderIcon from '@material-ui/icons/Folder'
import MusicNoteIcon from '@material-ui/icons/MusicNote'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(2),
  },
  header: {
    marginBottom: theme.spacing(2),
  },
  actions: {
    display: 'flex',
    gap: theme.spacing(1),
    marginTop: theme.spacing(1),
    marginBottom: theme.spacing(2),
  },
  list: {
    backgroundColor: theme.palette.background.paper,
  },
}))

const DiscoveryBrowser = () => {
  const classes = useStyles()
  const [path, setPath] = useState('')
  const [folders, setFolders] = useState([])
  const [files, setFiles] = useState([])
  const inputRef = useRef(null)

  const fetchData = useCallback(async () => {
    const params = path ? `?path=${encodeURIComponent(path)}` : ''
    const response = await fetch(`/api/discoveryfs${params}`)
    if (!response.ok) {
      setFolders([])
      setFiles([])
      return
    }
    const data = await response.json()
    const newPath = data.path || ''
    if (newPath !== path) {
      setPath(newPath)
    }
    setFolders(data.folders || [])
    setFiles(data.files || [])
  }, [path])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const navigateTo = (nextPath) => {
    setPath(nextPath)
  }

  const parentPath = () => {
    if (!path) return ''
    const parts = path.split('/')
    parts.pop()
    return parts.join('/')
  }

  const handleFolderClick = (folder) => {
    navigateTo(folder.path)
  }

  const handleBreadcrumbClick = (index) => {
    if (index === -1) {
      navigateTo('')
    } else {
      const segments = path.split('/').slice(0, index + 1)
      navigateTo(segments.join('/'))
    }
  }

  const handleNewFolder = async () => {
    const name = window.prompt('Folder name:')
    if (!name) {
      return
    }
    const params = path ? `?path=${encodeURIComponent(path)}` : ''
    const response = await fetch(`/api/discoveryfs/folder${params}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    })
    if (response.ok) {
      fetchData()
    }
  }

  const handleUploadClick = () => {
    inputRef.current?.click()
  }

  const handleUpload = async (event) => {
    const filesToUpload = event.target.files
    if (!filesToUpload?.length) {
      return
    }
    const params = path ? `?path=${encodeURIComponent(path)}` : ''
    const formData = new FormData()
    Array.from(filesToUpload).forEach((file) => {
      formData.append('files', file)
    })
    const response = await fetch(`/api/discoveryfs/upload${params}`, {
      method: 'POST',
      body: formData,
    })
    event.target.value = ''
    if (response.ok) {
      fetchData()
    }
  }

  const crumbs = path ? path.split('/') : []

  return (
    <Box className={classes.root}>
      <Box className={classes.header}>
        <Typography variant="h5">Discovery</Typography>
        <Breadcrumbs aria-label="breadcrumb">
          <Button color="primary" onClick={() => handleBreadcrumbClick(-1)}>
            Root
          </Button>
          {crumbs.map((crumb, index) => (
            <Button
              color="primary"
              key={`${crumb}-${index}`}
              onClick={() => handleBreadcrumbClick(index)}
            >
              {crumb}
            </Button>
          ))}
        </Breadcrumbs>
      </Box>
      <Box className={classes.actions}>
        <Button variant="contained" color="primary" onClick={handleNewFolder}>
          New Folder
        </Button>
        <input
          type="file"
          accept="audio/*"
          multiple
          hidden
          ref={inputRef}
          onChange={handleUpload}
        />
        <Button
          variant="contained"
          color="default"
          startIcon={<CloudUploadIcon />}
          onClick={handleUploadClick}
        >
          Upload
        </Button>
      </Box>
      <List className={classes.list}>
        {path && (
          <ListItem button onClick={() => navigateTo(parentPath())}>
            <ListItemText primary=".." secondary="Parent Folder" />
          </ListItem>
        )}
        {folders.map((folder) => (
          <ListItem button key={folder.path} onClick={() => handleFolderClick(folder)}>
            <ListItemIcon>
              <FolderIcon />
            </ListItemIcon>
            <ListItemText primary={folder.name} secondary="Folder" />
          </ListItem>
        ))}
        {files.map((file) => (
          <ListItem key={file.path}>
            <ListItemIcon>
              <MusicNoteIcon />
            </ListItemIcon>
            <ListItemText primary={file.name} secondary="Audio" />
          </ListItem>
        ))}
        {!folders.length && !files.length && (
          <ListItem>
            <ListItemText primary="This folder is empty." />
          </ListItem>
        )}
      </List>
    </Box>
  )
}

export default DiscoveryBrowser
