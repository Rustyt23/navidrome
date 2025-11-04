import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormHelperText,
  IconButton,
  InputLabel,
  Menu,
  MenuItem,
  Paper,
  Select,
  TextField,
  Tooltip,
  Typography,
} from '@material-ui/core'
import { makeStyles, useTheme } from '@material-ui/core/styles'
import { Title, useTranslate } from 'react-admin'
import AddIcon from '@material-ui/icons/Add'
import EditIcon from '@material-ui/icons/Edit'
import FolderIcon from '@material-ui/icons/Folder'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import Breadcrumbs from '@material-ui/core/Breadcrumbs'
import Link from '@material-ui/core/Link'
import clsx from 'clsx'
import PropTypes from 'prop-types'
import { useHistory } from 'react-router-dom'
import { useSelector } from 'react-redux'
import { ToggleFieldsMenu, useSelectedFields } from '../common'
import { useRetailPlayerDeviceStore } from './RetailPlayerDeviceStoreContext'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(5),
    maxWidth: 1200,
    margin: '0 auto',
    width: '100%',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
    [theme.breakpoints.down('md')]: {
      padding: theme.spacing(4),
    },
    [theme.breakpoints.down('sm')]: {
      padding: theme.spacing(2.5),
      gap: theme.spacing(2),
    },
  },
  header: {
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  },
  titleBlock: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
  },
  actions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  panel: {
    borderRadius: theme.shape.borderRadius,
    border: `1px solid ${theme.palette.divider}`,
    overflowX: 'auto',
    overflowY: 'hidden',
    backgroundColor: theme.palette.background.paper,
  },
  listHeader: {
    display: 'grid',
    padding: theme.spacing(1.5, 2),
    backgroundColor: theme.palette.action.hover,
    color: theme.palette.text.secondary,
    fontSize: theme.typography.pxToRem(12),
    textTransform: 'uppercase',
    letterSpacing: 0.8,
    fontWeight: theme.typography.fontWeightMedium,
    alignItems: 'center',
  },
  headerSerial: {
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  headerName: {
    display: 'flex',
    alignItems: 'center',
  },
  headerType: {
    display: 'flex',
    alignItems: 'center',
  },
  headerChannel: {
    display: 'flex',
    alignItems: 'center',
  },
  headerChannelList: {
    display: 'flex',
    alignItems: 'center',
  },
  headerActions: {
    display: 'flex',
    justifyContent: 'flex-end',
  },
  row: {
    display: 'grid',
    alignItems: 'center',
    padding: theme.spacing(1.5, 2),
    borderTop: `1px solid ${theme.palette.divider}`,
  },
  folderRow: {
    backgroundColor: theme.palette.action.selected,
  },
  interactiveRow: {
    cursor: 'pointer',
    '&:hover': {
      backgroundColor: theme.palette.action.hover,
    },
    '&:focus': {
      outline: `2px solid ${theme.palette.primary.main}`,
      outlineOffset: -2,
    },
  },
  nameCell: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    fontWeight: theme.typography.fontWeightMedium,
  },
  nameIcon: {
    color: theme.palette.text.secondary,
    fontSize: theme.typography.pxToRem(18),
  },
  nameLabel: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  },
  typeCell: {
    fontSize: theme.typography.pxToRem(14),
    color: theme.palette.text.secondary,
  },
  metaCell: {
    fontSize: theme.typography.pxToRem(14),
    color: theme.palette.text.secondary,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  actionsCell: {
    display: 'flex',
    gap: theme.spacing(1),
    justifyContent: 'flex-end',
  },
  serialCell: {
    fontVariantNumeric: 'tabular-nums',
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  breadcrumbBar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    padding: theme.spacing(1.5, 2),
    borderBottom: `1px solid ${theme.palette.divider}`,
    backgroundColor: theme.palette.background.default,
  },
  breadcrumbs: {
    '& .MuiBreadcrumbs-separator': {
      color: theme.palette.text.secondary,
    },
  },
  breadcrumbLink: {
    color: theme.palette.primary.main,
    cursor: 'pointer',
    fontWeight: theme.typography.fontWeightMedium,
  },
  emptyState: {
    padding: theme.spacing(4),
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  loaderState: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(4),
    gap: theme.spacing(2),
    color: theme.palette.text.secondary,
  },
  errorState: {
    padding: theme.spacing(4),
    textAlign: 'center',
    color: theme.palette.error.main,
  },
  dialogFields: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2.5),
    minWidth: 360,
    [theme.breakpoints.down('xs')]: {
      minWidth: 'auto',
    },
  },
}))

const RETAIL_DEVICE_RESOURCE = 'retailDevice'
const RETAIL_DEVICE_COLUMNS = [
  'type',
  'channel',
  'channelList',
  'organization',
  'timeZone',
  'source',
]
const RETAIL_DEVICE_DEFAULT_OFF = ['timeZone', 'source']

const FolderDialog = ({
  open,
  onClose,
  onSubmit,
  parentOptions,
  initialValues,
  disableParent,
}) => {
  const classes = useStyles()
  const [name, setName] = useState(initialValues?.name || '')
  const [parentId, setParentId] = useState(initialValues?.parentId || '')

  useEffect(() => {
    setName(initialValues?.name || '')
    setParentId(initialValues?.parentId || '')
  }, [initialValues, open])

  const handleSubmit = () => {
    if (!name.trim()) {
      return
    }
    onSubmit({ name: name.trim(), parentId: parentId || null })
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle>{initialValues?.id ? 'Edit Folder' : 'Create Folder'}</DialogTitle>
      <DialogContent>
        <div className={classes.dialogFields}>
          <TextField
            autoFocus
            label="Folder Name"
            fullWidth
            variant="outlined"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
          <FormControl variant="outlined" fullWidth disabled={disableParent}>
            <InputLabel id="folder-parent-label">Parent Folder</InputLabel>
            <Select
              labelId="folder-parent-label"
              value={parentId}
              onChange={(event) => setParentId(event.target.value)}
              label="Parent Folder"
            >
              <MenuItem value="">
                <em>None</em>
              </MenuItem>
              {parentOptions.map((option) => (
                <MenuItem key={option.id} value={option.id}>
                  {option.name}
                </MenuItem>
              ))}
            </Select>
            {disableParent && (
              <FormHelperText>
                Parent folders cannot be changed for existing groups yet.
              </FormHelperText>
            )}
          </FormControl>
        </div>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button color="primary" variant="contained" onClick={handleSubmit}>
          {initialValues?.id ? 'Save Changes' : 'Create Folder'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

FolderDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
  parentOptions: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
    }),
  ).isRequired,
  initialValues: PropTypes.shape({
    id: PropTypes.string,
    name: PropTypes.string,
    parentId: PropTypes.string,
  }),
  disableParent: PropTypes.bool,
}

FolderDialog.defaultProps = {
  initialValues: null,
  disableParent: false,
}

const DeviceDialog = ({
  open,
  onClose,
  onSubmit,
  parentOptions,
  initialValues,
}) => {
  const [form, setForm] = useState(() => ({
    name: initialValues?.name || '',
    channel: initialValues?.channel || '',
    channelList: initialValues?.channelList || '',
    organization: initialValues?.organization || '',
    folderId: initialValues?.folderId || '',
  }))

  useEffect(() => {
    setForm({
      name: initialValues?.name || '',
      channel: initialValues?.channel || '',
      channelList: initialValues?.channelList || '',
      organization: initialValues?.organization || '',
      folderId: initialValues?.folderId || '',
    })
  }, [initialValues, open])

  const isRemote = initialValues?.source !== 'local'

  const handleChange = (field) => (event) => {
    setForm((prev) => ({ ...prev, [field]: event.target.value }))
  }

  const handleSubmit = () => {
    if (!form.name.trim()) {
      return
    }
    onSubmit({
      ...form,
      name: form.name.trim(),
      folderId: form.folderId || null,
    })
  }

  const classes = useStyles()

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>
        {initialValues?.id ? 'Edit Device' : 'Create Device'}
      </DialogTitle>
      <DialogContent>
        <div className={classes.dialogFields}>
          <TextField
            autoFocus
            label="Device Name"
            fullWidth
            variant="outlined"
            value={form.name}
            onChange={handleChange('name')}
            disabled={isRemote}
            helperText={
              isRemote
                ? 'Name is managed by the device integration.'
                : 'Give the device a friendly label for identification.'
            }
          />
          <TextField
            label="Channel"
            fullWidth
            variant="outlined"
            value={form.channel}
            onChange={handleChange('channel')}
          />
          <TextField
            label="Channel List"
            fullWidth
            variant="outlined"
            value={form.channelList}
            onChange={handleChange('channelList')}
          />
          <TextField
            label="Organization"
            fullWidth
            variant="outlined"
            value={form.organization}
            onChange={handleChange('organization')}
          />
          <FormControl variant="outlined" fullWidth>
            <InputLabel id="device-folder-label">Folder</InputLabel>
            <Select
              labelId="device-folder-label"
              value={form.folderId}
              onChange={handleChange('folderId')}
              label="Folder"
            >
              <MenuItem value="">
                <em>None</em>
              </MenuItem>
              {parentOptions.map((option) => (
                <MenuItem key={option.id} value={option.id}>
                  {option.name}
                </MenuItem>
              ))}
            </Select>
            <FormHelperText>
              Use folders to keep devices grouped by location or usage.
            </FormHelperText>
          </FormControl>
        </div>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button color="primary" variant="contained" onClick={handleSubmit}>
          {initialValues?.id ? 'Save Changes' : 'Create Device'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

DeviceDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
  parentOptions: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
    }),
  ).isRequired,
  initialValues: PropTypes.shape({
    id: PropTypes.string,
    name: PropTypes.string,
    channel: PropTypes.string,
    channelList: PropTypes.string,
    organization: PropTypes.string,
    folderId: PropTypes.string,
    source: PropTypes.string,
  }),
}

DeviceDialog.defaultProps = {
  initialValues: null,
}

const RetailPlayerDeviceManagement = () => {
  const classes = useStyles()
  const theme = useTheme()
  const history = useHistory()
  const translate = useTranslate()
  const {
    state: { tree, folders, devices, loading, error },
    actions: { createFolder, updateFolder, createDevice, updateDevice },
  } = useRetailPlayerDeviceStore()
  const [menuAnchor, setMenuAnchor] = useState(null)
  const [folderDialog, setFolderDialog] = useState({ open: false, target: null })
  const [deviceDialog, setDeviceDialog] = useState({ open: false, target: null })
  const [activeFolderId, setActiveFolderId] = useState(null)

  const folderOptions = useMemo(
    () => folders.map((folder) => ({ id: folder.id, name: folder.name })),
    [folders],
  )

  const folderMap = useMemo(() => {
    const map = new Map()
    folders.forEach((folder) => {
      map.set(folder.id, folder)
    })
    return map
  }, [folders])

  const deviceMap = useMemo(() => {
    const map = new Map()
    devices.forEach((device) => {
      map.set(device.id, device)
    })
    return map
  }, [devices])

  const findFolderNode = useCallback((nodes, targetId) => {
    if (!targetId) {
      return null
    }
    for (let index = 0; index < nodes.length; index += 1) {
      const node = nodes[index]
      if (node.type !== 'folder') {
        // eslint-disable-next-line no-continue
        continue
      }
      if (node.id === targetId) {
        return node
      }
      const childResult = findFolderNode(node.children || [], targetId)
      if (childResult) {
        return childResult
      }
    }
    return null
  }, [])

  const activeFolderNode = useMemo(
    () => findFolderNode(tree, activeFolderId),
    [findFolderNode, tree, activeFolderId],
  )

  useEffect(() => {
    if (activeFolderId && !activeFolderNode) {
      setActiveFolderId(null)
    }
  }, [activeFolderId, activeFolderNode])

  const activeFolderPath = useMemo(() => {
    if (!activeFolderId) {
      return []
    }
    const path = []
    let currentId = activeFolderId
    const seen = new Set()
    while (currentId) {
      if (seen.has(currentId)) {
        break
      }
      seen.add(currentId)
      const folder = folderMap.get(currentId)
      if (!folder) {
        break
      }
      path.unshift(folder)
      currentId = folder.parentId || null
    }
    return path
  }, [activeFolderId, folderMap])

  const visibleNodes = activeFolderNode ? activeFolderNode.children || [] : tree

  const columnDefinitions = useMemo(() => {
    const dash = '—'
    return {
      type: {
        label: translate('resources.retailDevice.fields.type', { _: 'Type' }),
        cellClass: classes.typeCell,
        headerClass: classes.headerType,
        width: 'minmax(120px, 1fr)',
        render: (node) =>
          node.type === 'folder'
            ? translate('resources.retailDevice.values.folder', { _: 'Folder' })
            : translate('resources.retailDevice.values.device', { _: 'Device' }),
      },
      channel: {
        label: translate('resources.retailDevice.fields.channel', { _: 'Channel' }),
        cellClass: classes.metaCell,
        headerClass: classes.headerChannel,
        width: 'minmax(140px, 1fr)',
        render: (node) => (node.type === 'device' ? node.channel || dash : dash),
      },
      channelList: {
        label: translate('resources.retailDevice.fields.channelList', {
          _: 'Channel List',
        }),
        cellClass: classes.metaCell,
        headerClass: classes.headerChannelList,
        width: 'minmax(160px, 1fr)',
        render: (node) => (node.type === 'device' ? node.channelList || dash : dash),
      },
      organization: {
        label: translate('resources.retailDevice.fields.organization', {
          _: 'Organization',
        }),
        cellClass: classes.metaCell,
        headerClass: classes.headerChannelList,
        width: 'minmax(180px, 1.2fr)',
        render: (node) => (node.type === 'device' ? node.organization || dash : dash),
      },
      timeZone: {
        label: translate('resources.retailDevice.fields.timeZone', {
          _: 'Time Zone',
        }),
        cellClass: classes.metaCell,
        headerClass: classes.headerChannelList,
        width: 'minmax(160px, 1fr)',
        render: (node) => (node.type === 'device' ? node.timeZone || dash : dash),
      },
      source: {
        label: translate('resources.retailDevice.fields.source', { _: 'Source' }),
        cellClass: classes.metaCell,
        headerClass: classes.headerChannelList,
        width: 'minmax(140px, 1fr)',
        render: (node) =>
          node.type === 'device'
            ? translate(`resources.retailDevice.values.${
                node.source === 'local' ? 'local' : 'remote'
              }`, {
                _: node.source === 'local' ? 'Local' : 'Remote',
              })
            : dash,
      },
    }
  }, [classes.headerChannel, classes.headerChannelList, classes.headerType, classes.metaCell, classes.typeCell, translate])

  const placeholderColumns = useMemo(() => {
    const placeholders = {}
    RETAIL_DEVICE_COLUMNS.forEach((key) => {
      placeholders[key] = <span />
    })
    return placeholders
  }, [])

  useSelectedFields({
    resource: RETAIL_DEVICE_RESOURCE,
    columns: placeholderColumns,
    defaultOff: RETAIL_DEVICE_DEFAULT_OFF,
  })

  const toggleableFields =
    useSelector((state) => state.settings.toggleableFields?.[RETAIL_DEVICE_RESOURCE]) || {}
  const columnsOrderSetting =
    useSelector((state) => state.settings.columnsOrder?.[RETAIL_DEVICE_RESOURCE]) ||
    RETAIL_DEVICE_COLUMNS

  const visibleColumnKeys = useMemo(() => {
    const orderedKeys = Array.isArray(columnsOrderSetting)
      ? columnsOrderSetting.filter((key) => RETAIL_DEVICE_COLUMNS.includes(key))
      : RETAIL_DEVICE_COLUMNS
    return orderedKeys.filter((key) => toggleableFields[key])
  }, [columnsOrderSetting, toggleableFields])

  const gridTemplateColumns = useMemo(() => {
    const dynamicColumns = visibleColumnKeys.map((key) => {
      const definition = columnDefinitions[key]
      return definition?.width || 'minmax(140px, 1fr)'
    })
    return [
      'minmax(56px, 0.4fr)',
      'minmax(240px, 2.2fr)',
      ...dynamicColumns,
      'minmax(120px, 0.8fr)',
    ].join(' ')
  }, [columnDefinitions, visibleColumnKeys])

  const gridStyle = useMemo(
    () => ({ gridTemplateColumns, minWidth: 720 }),
    [gridTemplateColumns],
  )

  const showOrganizationCaption = useMemo(
    () => !visibleColumnKeys.includes('organization'),
    [visibleColumnKeys],
  )

  const serialLabel = translate('resources.retailDevice.fields.serial', { _: '#' })
  const nameLabel = translate('resources.retailDevice.fields.name', { _: 'Name' })
  const actionsLabel = translate('resources.retailDevice.fields.actions', {
    _: 'Actions',
  })

  const openMenu = (event) => {
    setMenuAnchor(event.currentTarget)
  }

  const closeMenu = () => {
    setMenuAnchor(null)
  }

  const handleCreateFolder = () => {
    closeMenu()
    setFolderDialog({ open: true, target: null })
  }

  const handleCreateDevice = () => {
    closeMenu()
    setDeviceDialog({ open: true, target: null })
  }

  const handleEditFolder = (folderId) => {
    const folder = folderMap.get(folderId)
    if (!folder) return
    setFolderDialog({ open: true, target: folder })
  }

  const handleEditDevice = (deviceId) => {
    const device = deviceMap.get(deviceId)
    if (!device) return
    setDeviceDialog({ open: true, target: device })
  }

  const handleFolderDialogClose = () => {
    setFolderDialog({ open: false, target: null })
  }

  const handleDeviceDialogClose = () => {
    setDeviceDialog({ open: false, target: null })
  }

  const handleFolderSubmit = (values) => {
    if (folderDialog.target) {
      updateFolder({ id: folderDialog.target.id, name: values.name })
    } else {
      createFolder(values)
    }
    handleFolderDialogClose()
  }

  const handleDeviceSubmit = (values) => {
    if (deviceDialog.target) {
      updateDevice({ id: deviceDialog.target.id, ...values })
    } else {
      createDevice(values)
    }
    handleDeviceDialogClose()
  }

  const handleNavigateToDevice = useCallback(
    (device) => {
      if (!device) {
        return
      }
      const slug = device.slug || device.name || device.id
      if (!slug) {
        return
      }
      const encodedSlug = encodeURIComponent(slug)
      history.push(`/retailplayer/${encodedSlug}`)
    },
    [history],
  )

  const handleEnterFolder = useCallback((folderId) => {
    if (!folderId) {
      setActiveFolderId(null)
      return
    }
    setActiveFolderId(folderId)
  }, [])

  const handleRowKeyDown = (event, action) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      action()
    }
  }

  const countDevices = useCallback((node) => {
    if (!node || !Array.isArray(node.children)) {
      return 0
    }
    return node.children.reduce((acc, child) => {
      if (child.type === 'device') {
        return acc + 1
      }
      return acc + countDevices(child)
    }, 0)
  }, [])

  let serialCounter = 0

  const renderColumnCells = (node) =>
    visibleColumnKeys.map((columnKey) => {
      const definition = columnDefinitions[columnKey]
      if (!definition) {
        return null
      }
      return (
        <div
          key={`${node.id}-${columnKey}`}
          className={definition.cellClass || classes.metaCell}
        >
          {definition.render(node)}
        </div>
      )
    })

  const getSerial = () => {
    serialCounter += 1
    return serialCounter
  }

  const renderRows = (nodes, depth = 0) =>
    nodes.flatMap((node) => {
      const indentStyle = { paddingLeft: theme.spacing(depth * 2) }
      const serialValue = getSerial()
      if (node.type === 'folder') {
        const folderChildren = renderRows(node.children || [], depth + 1)
        const deviceCount = countDevices(node)
        return [
          <div
            key={`folder-row-${node.id}`}
            className={clsx(classes.row, classes.folderRow, classes.interactiveRow)}
            style={gridStyle}
            role="button"
            tabIndex={0}
            onClick={() => handleEnterFolder(node.id)}
            onKeyDown={(event) => handleRowKeyDown(event, () => handleEnterFolder(node.id))}
            aria-label={`Open folder ${node.name}`}
          >
            <div className={classes.serialCell}>{serialValue}</div>
            <div className={classes.nameCell} style={indentStyle}>
              <FolderIcon className={classes.nameIcon} />
              <div className={classes.nameLabel}>
                <Typography variant="body1" color="textPrimary">
                  {node.name}
                </Typography>
                <Typography variant="caption" color="textSecondary">
                  {`${deviceCount} device${deviceCount === 1 ? '' : 's'}`}
                </Typography>
              </div>
            </div>
            {renderColumnCells(node)}
            <div className={classes.actionsCell}>
              <Tooltip title="Edit folder">
                <IconButton
                  size="small"
                  onClick={(event) => {
                    event.stopPropagation()
                    handleEditFolder(node.id)
                  }}
                  aria-label={`Edit folder ${node.name}`}
                >
                  <EditIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </div>
          </div>,
          ...folderChildren,
        ]
      }
      return [
        <div
          key={`device-row-${node.id}`}
          className={clsx(classes.row, classes.interactiveRow)}
          style={gridStyle}
          role="button"
          tabIndex={0}
          onClick={() => handleNavigateToDevice(node)}
          onKeyDown={(event) => handleRowKeyDown(event, () => handleNavigateToDevice(node))}
          aria-label={`Open device ${node.name}`}
        >
          <div className={classes.serialCell}>{serialValue}</div>
          <div className={classes.nameCell} style={indentStyle}>
            <SpeakerGroupIcon className={classes.nameIcon} />
            <div className={classes.nameLabel}>
              <Typography variant="body1" color="textPrimary">
                {node.name}
              </Typography>
              {showOrganizationCaption && node.organization ? (
                <Typography variant="caption" color="textSecondary">
                  {node.organization}
                </Typography>
              ) : null}
            </div>
          </div>
          {renderColumnCells(node)}
          <div className={classes.actionsCell}>
            <Tooltip title="Edit device">
              <IconButton
                size="small"
                onClick={(event) => {
                  event.stopPropagation()
                  handleEditDevice(node.id)
                }}
                aria-label={`Edit device ${node.name}`}
              >
                <EditIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </div>
        </div>,
      ]
    })

  return (
    <div className={classes.root}>
      <Title title="Retail Player Devices" />
      <div className={classes.header}>
        <div className={classes.titleBlock}>
          <Typography component="h1" variant="h4">
            Retail Player Devices
          </Typography>
          <Typography variant="body2" color="textSecondary">
            Organize retail player endpoints into folders for quick access and future
            device management.
          </Typography>
        </div>
        <div className={classes.actions}>
          <ToggleFieldsMenu resource={RETAIL_DEVICE_RESOURCE} />
          <Button
            color="primary"
            variant="contained"
            startIcon={<AddIcon />}
            onClick={openMenu}
            aria-haspopup="true"
            aria-controls="retail-device-create-menu"
          >
            Create
          </Button>
          <Menu
            id="retail-device-create-menu"
            anchorEl={menuAnchor}
            keepMounted
            open={Boolean(menuAnchor)}
            onClose={closeMenu}
          >
            <MenuItem onClick={handleCreateFolder}>Create Folder</MenuItem>
            <MenuItem onClick={handleCreateDevice}>Create Device</MenuItem>
          </Menu>
        </div>
      </div>
      <Paper className={classes.panel} elevation={0}>
        {activeFolderNode ? (
          <div className={classes.breadcrumbBar}>
            <Breadcrumbs
              aria-label="Folder navigation"
              className={classes.breadcrumbs}
              maxItems={4}
            >
              <Link
                color="inherit"
                onClick={() => setActiveFolderId(null)}
                className={classes.breadcrumbLink}
                component="button"
              >
                All devices
              </Link>
              {activeFolderPath.map((folder, index) => {
                const isLast = index === activeFolderPath.length - 1
                if (isLast) {
                  return (
                    <Typography key={folder.id} color="textPrimary">
                      {folder.name}
                    </Typography>
                  )
                }
                return (
                  <Link
                    key={folder.id}
                    color="inherit"
                    onClick={() => setActiveFolderId(folder.id)}
                    className={classes.breadcrumbLink}
                    component="button"
                  >
                    {folder.name}
                  </Link>
                )
              })}
            </Breadcrumbs>
          </div>
        ) : null}
        <div className={classes.listHeader} style={gridStyle}>
          <span className={classes.headerSerial}>{serialLabel}</span>
          <span className={classes.headerName}>{nameLabel}</span>
          {visibleColumnKeys.map((columnKey) => {
            const definition = columnDefinitions[columnKey]
            if (!definition) {
              return null
            }
            return (
              <span
                key={`header-${columnKey}`}
                className={definition.headerClass || classes.headerChannelList}
              >
                {definition.label}
              </span>
            )
          })}
          <span className={classes.headerActions}>{actionsLabel}</span>
        </div>
        {loading ? (
          <div className={classes.loaderState}>
            <CircularProgress size={20} />
            <Typography variant="body2">Loading devices…</Typography>
          </div>
        ) : error ? (
          <div className={classes.errorState}>
            <Typography variant="body2">
              We could not load retail player devices right now. Please try again.
            </Typography>
          </div>
        ) : visibleNodes.length ? (
          renderRows(visibleNodes)
        ) : (
          <div className={classes.emptyState}>
            {activeFolderNode ? (
              <Typography variant="body2">
                This folder does not contain any devices yet.
              </Typography>
            ) : (
              <Typography variant="body2">
                No devices found yet. Use the Create menu to add folders or local
                devices.
              </Typography>
            )}
          </div>
        )}
      </Paper>

      <FolderDialog
        open={folderDialog.open}
        onClose={handleFolderDialogClose}
        onSubmit={handleFolderSubmit}
        parentOptions={folderOptions}
        initialValues={folderDialog.target}
        disableParent={Boolean(folderDialog.target)}
      />
      <DeviceDialog
        open={deviceDialog.open}
        onClose={handleDeviceDialogClose}
        onSubmit={handleDeviceSubmit}
        parentOptions={folderOptions}
        initialValues={deviceDialog.target}
      />
    </div>
  )
}

export default RetailPlayerDeviceManagement
