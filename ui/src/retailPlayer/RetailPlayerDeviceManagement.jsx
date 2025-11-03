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
import { Title } from 'react-admin'
import AddIcon from '@material-ui/icons/Add'
import EditIcon from '@material-ui/icons/Edit'
import FolderIcon from '@material-ui/icons/Folder'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import clsx from 'clsx'
import PropTypes from 'prop-types'
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
      'minmax(220px, 2fr) minmax(120px, 1fr) minmax(160px, 1fr) minmax(160px, 1fr) minmax(100px, 0.8fr)',
    padding: theme.spacing(1.5, 2),
    backgroundColor: theme.palette.action.hover,
    color: theme.palette.text.secondary,
    fontSize: theme.typography.pxToRem(12),
    textTransform: 'uppercase',
    letterSpacing: 0.8,
    fontWeight: theme.typography.fontWeightMedium,
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns: 'minmax(200px, 2fr) minmax(120px, 1fr) minmax(140px, 1fr)',
      gridTemplateAreas: "'name type actions' 'channel channelList actions'",
      rowGap: theme.spacing(1),
    },
  },
  headerName: {
    [theme.breakpoints.down('sm')]: {
      gridArea: 'name',
    },
  },
  headerType: {
    [theme.breakpoints.down('sm')]: {
      gridArea: 'type',
    },
  },
  headerChannel: {
    [theme.breakpoints.down('sm')]: {
      gridArea: 'channel',
    },
  },
  headerChannelList: {
    [theme.breakpoints.down('sm')]: {
      gridArea: 'channelList',
    },
  },
  headerActions: {
    [theme.breakpoints.down('sm')]: {
      gridArea: 'actions',
      justifySelf: 'flex-end',
    },
  },
  row: {
    display: 'grid',
    gridTemplateColumns:
      'minmax(220px, 2fr) minmax(120px, 1fr) minmax(160px, 1fr) minmax(160px, 1fr) minmax(100px, 0.8fr)',
    alignItems: 'center',
    padding: theme.spacing(1.5, 2),
    borderTop: `1px solid ${theme.palette.divider}`,
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns: 'minmax(200px, 2fr) minmax(120px, 1fr) minmax(140px, 1fr)',
      gridTemplateAreas: "'name type actions' 'channel channelList actions'",
      rowGap: theme.spacing(1),
    },
  },
  folderRow: {
    backgroundColor: theme.palette.action.selected,
  },
  nameCell: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    fontWeight: theme.typography.fontWeightMedium,
    [theme.breakpoints.down('sm')]: {
      gridArea: 'name',
    },
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
    [theme.breakpoints.down('sm')]: {
      gridArea: 'type',
    },
  },
  metaCell: {
    fontSize: theme.typography.pxToRem(14),
    color: theme.palette.text.secondary,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    [theme.breakpoints.down('sm')]: {
      gridArea: 'channel',
    },
  },
  metaSecondary: {
    [theme.breakpoints.down('sm')]: {
      gridArea: 'channelList',
    },
  },
  actionsCell: {
    display: 'flex',
    gap: theme.spacing(1),
    justifyContent: 'flex-end',
    [theme.breakpoints.down('sm')]: {
      gridArea: 'actions',
    },
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
  const {
    state: { tree, folders, devices, loading, error },
    actions: { createFolder, updateFolder, createDevice, updateDevice },
  } = useRetailPlayerDeviceStore()
  const [menuAnchor, setMenuAnchor] = useState(null)
  const [folderDialog, setFolderDialog] = useState({ open: false, target: null })
  const [deviceDialog, setDeviceDialog] = useState({ open: false, target: null })

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
        return [
          <div
            key={`folder-row-${node.id}`}
            className={clsx(classes.row, classes.folderRow)}
          >
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
            <div className={classes.typeCell}>Folder</div>
            <div className={classes.metaCell}>—</div>
            <div className={clsx(classes.metaCell, classes.metaSecondary)}>—</div>
            <div className={classes.actionsCell}>
              <Tooltip title="Edit folder">
                <IconButton
                  size="small"
                  onClick={() => handleEditFolder(node.id)}
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
      return [
        <div key={`device-row-${node.id}`} className={classes.row}>
          <div className={classes.nameCell} style={indentStyle}>
            <SpeakerGroupIcon className={classes.nameIcon} />
            <div className={classes.nameLabel}>
              <Typography variant="body1" color="textPrimary">
                {node.name}
              </Typography>
              {node.organization ? (
                <Typography variant="caption" color="textSecondary">
                  {node.organization}
                </Typography>
              ) : null}
            </div>
          </div>
          <div className={classes.typeCell}>Device</div>
          <div className={classes.metaCell}>{node.channel || '—'}</div>
          <div className={clsx(classes.metaCell, classes.metaSecondary)}>
            {node.channelList || '—'}
          </div>
          <div className={classes.actionsCell}>
            <Tooltip title="Edit device">
              <IconButton
                size="small"
                onClick={() => handleEditDevice(node.id)}
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
        <div className={classes.listHeader}>
          <span className={classes.headerName}>Name</span>
          <span className={classes.headerType}>Type</span>
          <span className={classes.headerChannel}>Channel</span>
          <span className={classes.headerChannelList}>Channel List</span>
          <span className={classes.headerActions}>Actions</span>
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
        ) : tree.length ? (
          renderRows(tree)
        ) : (
          <div className={classes.emptyState}>
            <Typography variant="body2">
              No devices found yet. Use the Create menu to add folders or local
              devices.
            </Typography>
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
