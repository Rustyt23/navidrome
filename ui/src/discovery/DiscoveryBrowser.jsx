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
  Typography,
  makeStyles,
} from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'
import {
  FunctionField,
  TextField,
  Title,
  TopToolbar,
  sanitizeListRestProps,
  useNotify,
  useTranslate,
} from 'react-admin'
import httpClient from '../dataProvider/httpClient'
import { List } from '../common'
import DiscoveryDataGrid from './DiscoveryDataGrid'
import DiscoveryTypeIconField from './DiscoveryTypeIconField'
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
  uploadInput: {
    display: 'none',
  },
  actions: {
    display: 'flex',
    alignItems: 'center',
    marginBottom: theme.spacing(2),
    '& > *:not(:first-child)': {
      marginLeft: theme.spacing(1),
    },
  },
}))

const DiscoveryListActions = ({
  className,
  onCreate,
  onUploadClick,
  onFileChange,
  fileInputRef,
  uploadInputClassName,
  ...rest
}) => {
  const translate = useTranslate()

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      <MuiButton
        color="primary"
        variant="contained"
        onClick={onCreate}
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
        className={uploadInputClassName}
        style={{ display: 'none' }}
        onChange={onFileChange}
      />
      <MuiButton
        color="default"
        variant="contained"
        onClick={onUploadClick}
        startIcon={<CloudUploadIcon />}
        size="small"
      >
        {translate('resources.discovery.actions.upload', {
          _: 'Upload',
        }).toUpperCase()}
      </MuiButton>
    </TopToolbar>
  )
}

const DiscoveryBrowser = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const history = useHistory()
  const location = useLocation()
  const fileInputRef = useRef(null)

  const [currentPath, setCurrentPath] = useState('')
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

  const listFilter = useMemo(
    () => ({ path: currentPath }),
    [currentPath],
  )

  const listKey = useMemo(
    () => `${currentPath || 'root'}-${refreshToken}`,
    [currentPath, refreshToken],
  )

  const rowClick = useCallback((id, record) => {
    if (!record || record.type !== 'folder') return null
    const targetPath = record.path || ''
    const query = targetPath ? `?path=${encodeURIComponent(targetPath)}` : ''
    return `/discovery${query}`
  }, [])

  return (
    <div className={classes.container}>
      <Title title={`Navidrome - ${currentTitle}`} />
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
        key={listKey}
        basePath="/discovery"
        resource="discoveryFolder"
        filter={listFilter}
        sort={{ field: 'order', order: 'ASC' }}
        exporter={false}
        bulkActionButtons={false}
        actions={
          <DiscoveryListActions
            className={classes.actions}
            onCreate={handleCreate}
            onUploadClick={handleUploadClick}
            onFileChange={handleFileChange}
            fileInputRef={fileInputRef}
            uploadInputClassName={classes.uploadInput}
          />
        }
      >
        <DiscoveryDataGrid rowClick={rowClick}>
          <DiscoveryTypeIconField label={false} />
          <TextField source="name" />
          <TextField source="ownerName" />
          <FunctionField
            label="resources.discoveryFolder.fields.updatedAt"
            render={(record) => record?.updatedAt || '—'}
          />
          <FunctionField
            label="resources.discoveryFolder.fields.public"
            render={(record) => record?.public || '—'}
          />
          <FunctionField label="ra.action.edit" render={() => '—'} />
        </DiscoveryDataGrid>
      </List>
    </div>
  )
}

export default DiscoveryBrowser
