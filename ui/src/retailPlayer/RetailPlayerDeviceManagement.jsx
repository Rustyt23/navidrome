import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormHelperText,
  IconButton,
  InputLabel,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Select,
  TextField,
  Tooltip,
  Typography,
} from '@material-ui/core'
import { makeStyles, useTheme } from '@material-ui/core/styles'
import { fade } from '@material-ui/core/styles/colorManipulator'
import { Title } from 'react-admin'
import AddIcon from '@material-ui/icons/Add'
import EditIcon from '@material-ui/icons/Edit'
import FolderIcon from '@material-ui/icons/Folder'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import Breadcrumbs from '@material-ui/core/Breadcrumbs'
import Link from '@material-ui/core/Link'
import clsx from 'clsx'
import PropTypes from 'prop-types'
import { useHistory } from 'react-router-dom'
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
    overflow: 'hidden',
    backgroundColor: theme.palette.background.paper,
  },
  listHeader: {
    display: 'grid',
    gridTemplateColumns:
      '64px minmax(220px, 2fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(96px, 0.8fr)',
    padding: theme.spacing(1.5, 2),
    backgroundColor: theme.palette.action.hover,
    color: theme.palette.text.secondary,
    fontSize: theme.typography.pxToRem(12),
    textTransform: 'uppercase',
    letterSpacing: 0.8,
    fontWeight: theme.typography.fontWeightMedium,
    alignItems: 'center',
    gap: theme.spacing(1),
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns: '56px minmax(180px, 2fr) minmax(120px, 1fr) minmax(120px, 1fr) 72px',
      fontSize: theme.typography.pxToRem(11),
      letterSpacing: 0.6,
    },
  },
  headerSelect: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
  },
  headerLabel: {
    textTransform: 'uppercase',
  },
  headerActions: {
    justifySelf: 'flex-end',
  },
  row: {
    display: 'grid',
    gridTemplateColumns:
      '64px minmax(220px, 2fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(96px, 0.8fr)',
    alignItems: 'center',
    padding: theme.spacing(1.5, 2),
    borderTop: `1px solid ${theme.palette.divider}`,
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns: '56px minmax(180px, 2fr) minmax(120px, 1fr) minmax(120px, 1fr) 72px',
      rowGap: theme.spacing(1),
    },
  },
  folderRow: {
    backgroundColor: fade(theme.palette.primary.main, 0.04),
  },
  interactiveRow: {
    cursor: 'pointer',
    '&:hover': {
      backgroundColor: fade(theme.palette.primary.main, 0.08),
    },
    '&:focus': {
      outline: `2px solid ${theme.palette.primary.main}`,
      outlineOffset: -2,
    },
  },
  selectedRow: {
    backgroundColor: fade(theme.palette.primary.main, 0.12),
  },
  selectCell: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  nameCell: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    fontWeight: theme.typography.fontWeightMedium,
  },
  nameIcon: {
    color: theme.palette.primary.main,
    fontSize: theme.typography.pxToRem(18),
  },
  nameLabel: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  },
  nameTitle: {
    color: theme.palette.primary.main,
    fontWeight: theme.typography.fontWeightMedium,
  },
  typeCell: {
    fontSize: theme.typography.pxToRem(14),
    color: theme.palette.text.secondary,
  },
  countCell: {
    fontSize: theme.typography.pxToRem(14),
    color: theme.palette.text.secondary,
    textAlign: 'center',
  },
  actionsCell: {
    display: 'flex',
    gap: theme.spacing(1),
    justifyContent: 'flex-end',
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
  const {
    state: { tree, folders, devices, loading, error },
    actions: { createFolder, updateFolder, createDevice, updateDevice },
  } = useRetailPlayerDeviceStore()
  const [menuAnchor, setMenuAnchor] = useState(null)
  const [folderDialog, setFolderDialog] = useState({ open: false, target: null })
  const [deviceDialog, setDeviceDialog] = useState({ open: false, target: null })
  const [activeFolderId, setActiveFolderId] = useState(null)
  const [selectedIds, setSelectedIds] = useState(() => new Set())

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

  const visibleNodes = useMemo(
    () => (activeFolderNode ? activeFolderNode.children || [] : tree),
    [activeFolderNode, tree],
  )

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

  const visibleNodeIds = useMemo(() => {
    const ids = []
    const collect = (nodes) => {
      nodes.forEach((node) => {
        if (!node || !node.id) {
          return
        }
        ids.push(node.id)
        if (Array.isArray(node.children) && node.children.length) {
          collect(node.children)
        }
      })
    }
    collect(visibleNodes)
    return ids
  }, [visibleNodes])

  const allSelected =
    visibleNodeIds.length > 0 && visibleNodeIds.every((id) => selectedIds.has(id))
  const someSelected = !allSelected && visibleNodeIds.some((id) => selectedIds.has(id))

  const handleSelectAllChange = (event) => {
    const { checked } = event.target
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (checked) {
        visibleNodeIds.forEach((id) => next.add(id))
      } else {
        visibleNodeIds.forEach((id) => next.delete(id))
      }
      return next
    })
  }

  const toggleNodeSelection = useCallback((nodeId) => {
    if (!nodeId) {
      return
    }
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(nodeId)) {
        next.delete(nodeId)
      } else {
        next.add(nodeId)
      }
      return next
    })
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

  const renderRows = (nodes, depth = 0) =>
    nodes.flatMap((node) => {
      if (node.type === 'folder') {
        const indentStyle = { paddingLeft: theme.spacing(depth * 2) }
        const folderChildren = renderRows(node.children || [], depth + 1)
        const deviceCount = countDevices(node)
        const isSelected = selectedIds.has(node.id)
        return [
          <div
            key={`folder-row-${node.id}`}
            className={clsx(
              classes.row,
              classes.folderRow,
              classes.interactiveRow,
              isSelected && classes.selectedRow,
            )}
            role="button"
            tabIndex={0}
            onClick={() => handleEnterFolder(node.id)}
            onKeyDown={(event) => handleRowKeyDown(event, () => handleEnterFolder(node.id))}
            aria-label={`Open folder ${node.name}`}
          >
            <div className={classes.selectCell}>
              <Checkbox
                color="primary"
                checked={isSelected}
                onChange={(event) => {
                  event.stopPropagation()
                  toggleNodeSelection(node.id)
                }}
                onClick={(event) => event.stopPropagation()}
                inputProps={{ 'aria-label': `Select folder ${node.name}` }}
              />
            </div>
            <div className={classes.nameCell} style={indentStyle}>
              <FolderIcon className={classes.nameIcon} />
              <div className={classes.nameLabel}>
                <Typography variant="body1" className={classes.nameTitle}>
                  {node.name}
                </Typography>
              </div>
            </div>
            <div className={classes.typeCell}>Folder</div>
            <div className={classes.countCell}>{deviceCount}</div>
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
      const indentStyle = { paddingLeft: theme.spacing(depth * 2) }
      const isSelected = selectedIds.has(node.id)
      return [
        <div
          key={`device-row-${node.id}`}
          className={clsx(
            classes.row,
            classes.interactiveRow,
            isSelected && classes.selectedRow,
          )}
          role="button"
          tabIndex={0}
          onClick={() => handleNavigateToDevice(node)}
          onKeyDown={(event) => handleRowKeyDown(event, () => handleNavigateToDevice(node))}
          aria-label={`Open device ${node.name}`}
        >
          <div className={classes.selectCell}>
            <Checkbox
              color="primary"
              checked={isSelected}
              onChange={(event) => {
                event.stopPropagation()
                toggleNodeSelection(node.id)
              }}
              onClick={(event) => event.stopPropagation()}
              inputProps={{ 'aria-label': `Select device ${node.name}` }}
            />
          </div>
          <div className={classes.nameCell} style={indentStyle}>
            <SpeakerGroupIcon className={classes.nameIcon} />
            <div className={classes.nameLabel}>
              <Typography variant="body1" className={classes.nameTitle}>
                {node.name}
              </Typography>
            </div>
          </div>
          <div className={classes.typeCell}>Device</div>
          <div className={classes.countCell}>—</div>
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
        </div>
        <div className={classes.actions}>
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
            <MenuItem onClick={handleCreateFolder}>
              <ListItemIcon>
                <FolderIcon fontSize="small" className={classes.nameIcon} />
              </ListItemIcon>
              <ListItemText primary="Create Folder" />
            </MenuItem>
            <MenuItem onClick={handleCreateDevice}>
              <ListItemIcon>
                <SpeakerGroupIcon fontSize="small" className={classes.nameIcon} />
              </ListItemIcon>
              <ListItemText primary="Create Device" />
            </MenuItem>
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
        <div className={classes.listHeader}>
          <div className={classes.headerSelect}>
            <Checkbox
              color="primary"
              checked={allSelected}
              indeterminate={someSelected}
              onChange={handleSelectAllChange}
              inputProps={{ 'aria-label': 'Select all retail player items' }}
            />
          </div>
          <span>Name</span>
          <span>Type</span>
          <span>No. of Devices</span>
          <span className={classes.headerActions}>Edit</span>
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
