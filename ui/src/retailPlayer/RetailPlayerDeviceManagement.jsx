import React, { useCallback, useEffect, useMemo, useState, memo } from 'react'
import {
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormHelperText,
  IconButton,
  InputAdornment,
  InputLabel,
  ListItemText,
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
import LockIcon from '@material-ui/icons/Lock'
import LockOpenIcon from '@material-ui/icons/LockOpen'
import FolderIcon from '@material-ui/icons/Folder'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import SearchIcon from '@material-ui/icons/Search'
import DeleteOutlineIcon from '@material-ui/icons/DeleteOutline'
import Breadcrumbs from '@material-ui/core/Breadcrumbs'
import Link from '@material-ui/core/Link'
import clsx from 'clsx'
import PropTypes from 'prop-types'
import { useHistory } from 'react-router-dom'
import httpClient from '../dataProvider/httpClient'
import { useRetailPlayerDeviceStore } from './RetailPlayerDeviceStoreContext'
import AddToFolderDialog from './AddToFolderDialog'
import useAssignRetailPlayerDeviceToFolder from './useAssignRetailPlayerDeviceToFolder'
import {
  useRetailPlayerDeviceDrag,
  useRetailPlayerFolderDrop,
} from './useRetailPlayerDnD'
import buildRetailPlayerDnDStyles from './retailPlayerDnDStyles'
import useRetailPlayerChannelCounts from './useRetailPlayerChannelCounts'
import { isDeviceLocked } from './deviceLockState'

const useStyles = makeStyles((theme) => {
  const dndStyles = buildRetailPlayerDnDStyles(theme)
  return {
    root: {
      padding: theme.spacing(1, 5, 5, 5),
      maxWidth: 1200,
      margin: '0 auto',
      width: '100%',
      boxSizing: 'border-box',
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(2),
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
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  },
  titleBlock: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
  },
  headerControls: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
    justifyContent: 'flex-end',
  },
  actions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  selectionRibbon: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    padding: theme.spacing(1, 2.5),
    margin: theme.spacing(2, 2, 2, 2),
    borderRadius: theme.shape.borderRadius,
    background: `linear-gradient(135deg, ${fade(theme.palette.primary.dark, 0.9)}, ${fade(
      theme.palette.primary.main,
      0.9,
    )})`,
    color: theme.palette.primary.contrastText,
    boxShadow: `0 6px 8px ${fade(theme.palette.primary.main, 0.35)}`,
    flexWrap: 'wrap',
    [theme.breakpoints.down('xs')]: {
      flexDirection: 'column',
      alignItems: 'flex-start',
      gap: theme.spacing(1.25),
    },
  },
  selectionSummary: {
    fontWeight: theme.typography.fontWeightBold,
    letterSpacing: 1,
    textTransform: 'uppercase',
    fontSize: theme.typography.pxToRem(12),
  },
  selectionActions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    flexWrap: 'wrap',
    justifyContent: 'flex-end',
  },

  selectionActionButton: {
    fontSize: theme.typography.pxToRem(12),
    padding: theme.spacing(0.5, 1.25),
    minHeight: 32,
    letterSpacing: 0.8,
    textTransform: 'uppercase',
  },

  selectionPrimaryButton: {
  
    color: fade(theme.palette.common.white, 0.92),
    backgroundColor: 'transparent',
    border: 'none',
    '&:hover': {
      backgroundColor: fade(theme.palette.error.main, 0.16),


    },
  },
  selectionDeleteButton: {
    
  color: fade(theme.palette.common.white, 0.92),
  backgroundColor: 'transparent',
  border: 'none',
  '&:hover': {
    backgroundColor: fade(theme.palette.error.main, 0.16),

    },
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
      '64px minmax(220px, 2fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(200px, 1.2fr) minmax(96px, 0.8fr)',
    paddingTop: theme.spacing(0),
    paddingBottom: theme.spacing(0),
    paddingLeft: theme.spacing(1.7),
    paddingRight: theme.spacing(2),
    backgroundColor: theme.palette.action.hover,
    color: theme.palette.text.secondary,
    fontSize: theme.typography.pxToRem(14),
    textTransform: 'uppercase',
    letterSpacing: 0.8,
    fontWeight: theme.typography.fontWeightMedium,
    alignItems: 'center',
    gap: theme.spacing(1),
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns:
        '56px minmax(180px, 2fr) minmax(120px, 1fr) minmax(120px, 1fr) minmax(180px, 1.1fr) 72px',
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
      '64px minmax(220px, 2fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(200px, 1.2fr) minmax(96px, 0.8fr)',
    alignItems: 'center',
    padding: '1px 2px',
    borderTop: `1px solid ${theme.palette.divider}`,
    minHeight: 28, // 🔥 ensures consistent compact row height
    '& .MuiTypography-body1': {
      fontSize: '0.8rem', // reduce font size inside cell
      lineHeight: 1.2,
    },
    '& .MuiIconButton-root': {
      padding: 2, // shrink edit icon area
    },
    '& .MuiCheckbox-root': {
      padding: 2, // shrink checkbox hit area
    },
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns:
        '56px minmax(180px, 2fr) minmax(120px, 1fr) minmax(120px, 1fr) minmax(180px, 1.1fr) 72px',
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
    fontSize: theme.typography.pxToRem(16.5),
  },
  onlineIcon: {
    color: theme.palette.success.main,
  },
  nameLabel: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  },
  searchField: {
    minWidth: 220,
    '& .MuiOutlinedInput-root': {
      backgroundColor: theme.palette.background.default,
    },
  },
  nameTitle: {
    color: theme.palette.common.white,
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
  remoteControlCell: {
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
  lockIconActive: {
    color: theme.palette.secondary.main,
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
  dropTarget: dndStyles.dropTarget,
  dropTargetCanDrop: dndStyles.dropTargetCanDrop,
  dropTargetActive: dndStyles.dropTargetActive,
  dragging: dndStyles.dragItem,
  }
})

const FolderDialog = ({ open, onClose, onSubmit, initialValues }) => {
  const classes = useStyles()
  const [name, setName] = useState(initialValues?.name || '')

  useEffect(() => {
    setName(initialValues?.name || '')
  }, [initialValues, open])

  const handleSubmit = () => {
    if (!name.trim()) {
      return
    }
    onSubmit({ name: name.trim() })
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
  initialValues: PropTypes.shape({
    id: PropTypes.string,
    name: PropTypes.string,
  }),
}

FolderDialog.defaultProps = {
  initialValues: null,
}

const DeviceDialog = ({
  open,
  onClose,
  onSubmit,
  initialValues,
}) => {
  const [form, setForm] = useState(() => ({
    name: initialValues?.name || '',
    remoteControlId: initialValues?.remoteControlId || '',
  }))

  useEffect(() => {
    setForm({
      name: initialValues?.name || '',
      remoteControlId: initialValues?.remoteControlId || '',
    })
  }, [initialValues, open])

  const isRemote = initialValues?.source !== 'local'

  const handleNameChange = (event) => {
    setForm((prev) => ({ ...prev, name: event.target.value }))
  }

  const handleRemoteControlChange = (event) => {
    setForm((prev) => ({ ...prev, remoteControlId: event.target.value }))
  }

  const handleSubmit = () => {
    if (!form.name.trim() || !form.remoteControlId.trim()) {
      return
    }
    onSubmit({
      name: form.name.trim(),
      remoteControlId: form.remoteControlId.trim(),
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
            onChange={handleNameChange}
            disabled={isRemote}
            helperText={
              isRemote
                ? 'Name is managed by the device integration.'
                : 'Give the device a friendly label for identification.'
            }
          />
          <TextField
            label="QR ID"
            fullWidth
            variant="outlined"
            value={form.remoteControlId}
            onChange={handleRemoteControlChange}
            helperText="Paste the QR identifier used for remote control."
          />
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
  initialValues: PropTypes.shape({
    id: PropTypes.string,
    name: PropTypes.string,
    folderIds: PropTypes.arrayOf(PropTypes.string),
    folderId: PropTypes.string,
    source: PropTypes.string,
    remoteControlId: PropTypes.string,
  }),
}

DeviceDialog.defaultProps = {
  initialValues: null,
}

const RetailPlayerFolderRow = memo(
  ({
    node,
    deviceCount,
    isSelected,
    classes,
    onEnterFolder,
    onToggleSelection,
    onKeyDown,
    onEdit,
    onDeviceDrop,
  }) => {
    const { dropRef, isOver, canDrop } = useRetailPlayerFolderDrop({
      folderId: node.id,
      onDrop: (deviceId, folderId) => onDeviceDrop(deviceId, folderId),
    })

    return (
      <div
        ref={dropRef}
        className={clsx(
          classes.row,
          classes.folderRow,
          classes.interactiveRow,
          classes.dropTarget,
          isSelected && classes.selectedRow,
          canDrop && classes.dropTargetCanDrop,
          canDrop && isOver && classes.dropTargetActive,
        )}
        role="button"
        tabIndex={0}
        onClick={() => onEnterFolder(node.id)}
        onKeyDown={(event) => onKeyDown(event, () => onEnterFolder(node.id))}
        aria-label={`Open folder ${node.name}`}
      >
        <div className={classes.selectCell}>
          <Checkbox
            color="primary"
            checked={isSelected}
            onChange={(event) => {
              event.stopPropagation()
              onToggleSelection(node.id, event)
            }}
            onClick={(event) => event.stopPropagation()}
            inputProps={{ 'aria-label': `Select folder ${node.name}` }}
            style={{ transform: 'scale(0.8)' }}
          />
        </div>
        <div className={classes.nameCell}>
          <FolderIcon className={classes.nameIcon} />
          <div className={classes.nameLabel}>
            <Typography variant="body1" className={classes.nameTitle}>
              {node.name}
            </Typography>
          </div>
        </div>
        <div className={classes.typeCell}>Folder</div>
        <div className={classes.countCell}>{deviceCount}</div>
        <div className={classes.remoteControlCell}>—</div>
        <div className={classes.actionsCell}>
          <Tooltip title="Edit folder">
            <IconButton
              size="small"
              onClick={(event) => {
                event.stopPropagation()
                onEdit(node.id)
              }}
              aria-label={`Edit folder ${node.name}`}
            >
              <EditIcon style={{ fontSize: 15 }} />
            </IconButton>
          </Tooltip>
        </div>
      </div>
    )
  },
)

RetailPlayerFolderRow.propTypes = {
  node: PropTypes.shape({
    id: PropTypes.string.isRequired,
    name: PropTypes.string.isRequired,
    remoteControlId: PropTypes.string,
  }).isRequired,
  deviceCount: PropTypes.number.isRequired,
  isSelected: PropTypes.bool.isRequired,
  classes: PropTypes.object.isRequired,
  onEnterFolder: PropTypes.func.isRequired,
  onToggleSelection: PropTypes.func.isRequired,
  onKeyDown: PropTypes.func.isRequired,
  onEdit: PropTypes.func.isRequired,
  onDeviceDrop: PropTypes.func.isRequired,
}

RetailPlayerFolderRow.displayName = 'RetailPlayerFolderRow'

const RetailPlayerDeviceRow = memo(
  ({
    node,
    isSelected,
    classes,
    channelCount,
    onNavigate,
    onToggleSelection,
    onKeyDown,
    onEdit,
    onToggleLock,
    isLocked,
    isOnline,
  }) => {
    const { dragRef, isDragging } = useRetailPlayerDeviceDrag({
      deviceId: node.id,
      deviceName: node.name,
      origin: 'management-list',
    })

    return (
      <div
        ref={dragRef}
        className={clsx(
          classes.row,
          classes.interactiveRow,
          isSelected && classes.selectedRow,
          isDragging && classes.dragging,
        )}
        role="button"
        tabIndex={0}
        onClick={() => onNavigate(node)}
        onKeyDown={(event) => onKeyDown(event, () => onNavigate(node))}
        aria-label={`Open device ${node.name}`}
      >
        <div className={classes.selectCell}>
          <Checkbox
            color="primary"
            checked={isSelected}
            onChange={(event) => {
              event.stopPropagation()
              onToggleSelection(node.id, event)
            }}
            onClick={(event) => event.stopPropagation()}
            inputProps={{ 'aria-label': `Select device ${node.name}` }}
            style={{ transform: 'scale(0.8)' }}
          />
        </div>
        <div className={classes.nameCell}>
          <SpeakerGroupIcon
            className={clsx(classes.nameIcon, isOnline && classes.onlineIcon)}
          />
          <div className={classes.nameLabel}>
            <Typography variant="body1" className={classes.nameTitle}>
              {node.name}
            </Typography>
          </div>
        </div>
        <div className={classes.typeCell}>Device</div>
        <div className={classes.countCell}>
          {typeof channelCount === 'number' ? channelCount : '—'}
        </div>
        <div className={classes.remoteControlCell}>
          {node.remoteControlId ? node.remoteControlId : '—'}
        </div>
        <div className={classes.actionsCell}>
          <Tooltip title={isLocked ? 'Unlock device' : 'Lock device'}>
            <IconButton
              size="small"
              onClick={(event) => {
                event.stopPropagation()
                onToggleLock(node)
              }}
              aria-label={`${isLocked ? 'Unlock' : 'Lock'} device ${node.name}`}
            >
              {isLocked ? (
                <LockIcon style={{ fontSize: 15 }} className={classes.lockIconActive} />
              ) : (
                <LockOpenIcon style={{ fontSize: 15 }} />
              )}
            </IconButton>
          </Tooltip>
          <Tooltip title="Edit device">
            <IconButton
              size="small"
              onClick={(event) => {
                event.stopPropagation()
                onEdit(node.id)
              }}
              aria-label={`Edit device ${node.name}`}
            >
              <EditIcon style={{ fontSize: 15 }} />
            </IconButton>
          </Tooltip>
        </div>
      </div>
    )
  },
)

RetailPlayerDeviceRow.propTypes = {
  node: PropTypes.shape({
    id: PropTypes.string.isRequired,
    name: PropTypes.string.isRequired,
  }).isRequired,
  isSelected: PropTypes.bool.isRequired,
  classes: PropTypes.object.isRequired,
  channelCount: PropTypes.number,
  onNavigate: PropTypes.func.isRequired,
  onToggleSelection: PropTypes.func.isRequired,
  onKeyDown: PropTypes.func.isRequired,
  onEdit: PropTypes.func.isRequired,
  onToggleLock: PropTypes.func.isRequired,
  isLocked: PropTypes.bool,
  isOnline: PropTypes.bool,
}

RetailPlayerDeviceRow.defaultProps = {
  channelCount: null,
  isLocked: false,
  isOnline: null,
}

RetailPlayerDeviceRow.displayName = 'RetailPlayerDeviceRow'

const RetailPlayerDeviceManagement = () => {
  const classes = useStyles()
  const theme = useTheme()
  const history = useHistory()
  const {
    state: { tree, folders, devices, loading, error, isApiEnabled },
    actions: { createFolder, updateFolder, createDevice, updateDevice, deleteNodes },
  } = useRetailPlayerDeviceStore()
  const [folderDialog, setFolderDialog] = useState({
    open: false,
    target: null,
    parentId: null,
  })
  const [deviceDialog, setDeviceDialog] = useState({ open: false, target: null })
  const [activeFolderId, setActiveFolderId] = useState(null)
  const [selectedIds, setSelectedIds] = useState(() => new Set())
  const [lastSelectedId, setLastSelectedId] = useState(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [addToFolderDialogOpen, setAddToFolderDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [deviceStatusMap, setDeviceStatusMap] = useState(() => new Map())
  const assignDeviceToFolder = useAssignRetailPlayerDeviceToFolder()
  const { countsByDeviceId: channelCountsByDeviceId } =
    useRetailPlayerChannelCounts(devices, isApiEnabled)

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

  useEffect(() => {
    if (!devices?.length) {
      setDeviceStatusMap(new Map())
      return undefined
    }

    try {
      const rawCache = sessionStorage.getItem('retailPlayerDeviceStatusMap')
      if (rawCache) {
        const parsedCache = JSON.parse(rawCache)
        if (parsedCache && typeof parsedCache === 'object') {
          const hydrated = new Map()
          devices.forEach((device) => {
            const cachedValue = parsedCache[device.id]
            if (typeof cachedValue === 'boolean') {
              hydrated.set(device.id, cachedValue)
            }
          })
          if (hydrated.size) {
            setDeviceStatusMap(hydrated)
          }
        }
      }
    } catch (err) {
      console.warn('Unable to hydrate retail player device statuses', err)
    }

    if (!isApiEnabled) {
      return undefined
    }

    return undefined
  }, [devices, isApiEnabled])

  const folderChildrenMap = useMemo(() => {
    const map = new Map()
    folders.forEach((folder) => {
      if (!folder.parentId) {
        return
      }
      if (!map.has(folder.parentId)) {
        map.set(folder.parentId, [])
      }
      map.get(folder.parentId).push(folder.id)
    })
    return map
  }, [folders])

  const collectDescendantFolderIds = useCallback(
    (rootId) => {
      const descendants = new Set()
      if (!rootId) {
        return descendants
      }
      const queue = [...(folderChildrenMap.get(rootId) || [])]
      while (queue.length) {
        const current = queue.shift()
        if (!current || descendants.has(current)) {
          continue
        }
        descendants.add(current)
        const children = folderChildrenMap.get(current)
        if (Array.isArray(children) && children.length) {
          queue.push(...children)
        }
      }
      return descendants
    },
    [folderChildrenMap],
  )

  const selectedFolderIds = useMemo(
    () =>
      Array.from(selectedIds).filter((id) => id && folderMap.has(id)),
    [selectedIds, folderMap],
  )

  const selectedDeviceIds = useMemo(
    () =>
      Array.from(selectedIds).filter((id) => id && deviceMap.has(id)),
    [selectedIds, deviceMap],
  )

  const selectedCount = selectedFolderIds.length + selectedDeviceIds.length

  const handleDeviceDropOnFolder = useCallback(
    async (deviceId, folderId) => {
      if (!deviceId || !folderId) {
        return
      }
      try {
        await assignDeviceToFolder(deviceId, folderId)
      } catch (err) {
        console.error('Failed to assign retail player device to folder', err)
      }
    },
    [assignDeviceToFolder],
  )

  const excludedFolderIds = useMemo(() => {
    const excluded = new Set(selectedFolderIds)
    selectedFolderIds.forEach((folderId) => {
      collectDescendantFolderIds(folderId).forEach((descendantId) => {
        excluded.add(descendantId)
      })
    })
    return Array.from(excluded)
  }, [selectedFolderIds, collectDescendantFolderIds])

  const showSelectionRibbon = selectedCount > 0

  useEffect(() => {
    setSelectedIds((prev) => {
      if (!prev || prev.size === 0) {
        return prev
      }
      const validFolderIds = new Set(folders.map((folder) => folder.id))
      const validDeviceIds = new Set(devices.map((device) => device.id))
      const next = new Set()
      let changed = false
      prev.forEach((id) => {
        if (validFolderIds.has(id) || validDeviceIds.has(id)) {
          next.add(id)
        } else {
          changed = true
        }
      })
      return changed ? next : prev
    })
  }, [folders, devices])

  const handleAddToFolderDialogClose = useCallback(() => {
    setAddToFolderDialogOpen(false)
  }, [])

  const handleAddToFolderConfirm = useCallback(
    async ({ folderIds: incomingFolderIds, newFolderName }) => {
      const folderIdSet = new Set(
        Array.isArray(incomingFolderIds)
          ? incomingFolderIds
              .map((value) => (typeof value === 'string' ? value : null))
              .filter(Boolean)
          : [],
      )

      const trimmedNewFolderName =
        typeof newFolderName === 'string' ? newFolderName.trim() : ''

      if (trimmedNewFolderName) {
        const parentForNewFolder =
          folderIdSet.size > 0
            ? Array.from(folderIdSet)[0]
            : activeFolderId || null
        try {
          const newFolder = await createFolder({
            name: trimmedNewFolderName,
            parentId: parentForNewFolder,
          })
          if (newFolder && newFolder.id) {
            folderIdSet.add(newFolder.id)
          }
        } catch (err) {
          console.error('Failed to create retail player folder', err)
        }
      }

      const targetFolderIds = Array.from(folderIdSet)
      if (!targetFolderIds.length) {
        setAddToFolderDialogOpen(false)
        return
      }

      for (const deviceId of selectedDeviceIds) {
        const device = deviceMap.get(deviceId)
        if (!device) {
          continue
        }
        const existingIds = Array.isArray(device.folderIds)
          ? device.folderIds
          : []
        const mergedIds = Array.from(new Set([...existingIds, ...targetFolderIds]))
        const changed =
          mergedIds.length !== existingIds.length ||
          mergedIds.some((id, index) => id !== existingIds[index])
        if (changed) {
          try {
            await updateDevice({ id: deviceId, folderIds: mergedIds })
          } catch (err) {
            console.error('Failed to update retail player device folders', err)
          }
        }
      }

      const parentFolderId = targetFolderIds[0] || null
      if (parentFolderId) {
        for (const folderId of selectedFolderIds) {
          if (folderId === parentFolderId) {
            continue
          }
          const folder = folderMap.get(folderId)
          if (folder && folder.parentId === parentFolderId) {
            continue
          }
          try {
            await updateFolder({ id: folderId, parentId: parentFolderId })
          } catch (err) {
            console.error('Failed to move retail player folder', err)
          }
        }
      }

      setAddToFolderDialogOpen(false)
      setSelectedIds(new Set())
    },
    [
      activeFolderId,
      createFolder,
      deviceMap,
      folderMap,
      selectedDeviceIds,
      selectedFolderIds,
      updateDevice,
      updateFolder,
    ],
  )

  const handleDeleteDialogClose = useCallback(() => {
    setDeleteDialogOpen(false)
  }, [])

  const handleDeleteConfirm = useCallback(async () => {
    if (!selectedFolderIds.length && !selectedDeviceIds.length) {
      setDeleteDialogOpen(false)
      return
    }

    try {
      await deleteNodes({
        folderIds: selectedFolderIds,
        deviceIds: selectedDeviceIds,
      })
    } catch (err) {
      console.error('Failed to delete retail player items', err)
      return
    }

    const deletedFolderSet = new Set(selectedFolderIds)
    selectedFolderIds.forEach((folderId) => {
      collectDescendantFolderIds(folderId).forEach((descendantId) => {
        deletedFolderSet.add(descendantId)
      })
    })

    setActiveFolderId((previous) => {
      if (previous && deletedFolderSet.has(previous)) {
        return null
      }
      return previous
    })

    setDeleteDialogOpen(false)
    setSelectedIds(new Set())
  }, [
    collectDescendantFolderIds,
    deleteNodes,
    selectedDeviceIds,
    selectedFolderIds,
  ])

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

  const baseVisibleNodes = useMemo(
    () => (activeFolderNode ? activeFolderNode.children || [] : tree),
    [activeFolderNode, tree],
  )

  const normalizedSearchTerm = useMemo(
    () => searchTerm.trim().toLowerCase(),
    [searchTerm],
  )

  const visibleNodes = useMemo(() => {
    if (!normalizedSearchTerm) {
      return baseVisibleNodes
    }

    const matches = []
    const visit = (nodes) => {
      if (!Array.isArray(nodes) || !nodes.length) {
        return
      }
      nodes.forEach((node) => {
        if (!node) {
          return
        }
        const label = (node.name || '').toLowerCase()
        if (label.includes(normalizedSearchTerm)) {
          matches.push(node)
        }
        if (node.type === 'folder' && Array.isArray(node.children)) {
          visit(node.children)
        }
      })
    }

    visit(tree)
    return matches
  }, [baseVisibleNodes, normalizedSearchTerm, tree])

  const showingSearchResults = normalizedSearchTerm.length > 0

  const handleCreateFolder = () => {
    setFolderDialog({ open: true, target: null, parentId: activeFolderId })
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
    setFolderDialog({ open: false, target: null, parentId: null })
  }

  const handleDeviceDialogClose = () => {
    setDeviceDialog({ open: false, target: null })
  }

  const handleFolderSubmit = async (values) => {
    try {
      if (folderDialog.target) {
        await updateFolder({ id: folderDialog.target.id, name: values.name })
      } else {
        await createFolder({ ...values, parentId: folderDialog.parentId || null })
      }
      handleFolderDialogClose()
    } catch (err) {
      console.error('Failed to save retail player folder', err)
    }
  }

  const handleDeviceSubmit = async (values) => {
    try {
      if (deviceDialog.target) {
        await updateDevice({ id: deviceDialog.target.id, ...values })
      } else {
        createDevice(values)
      }
      handleDeviceDialogClose()
    } catch (err) {
      console.error('Failed to save retail player device', err)
    }
  }

  const handleToggleDeviceLock = useCallback(async (device) => {
    if (!device) {
      return
    }

    try {
      const nextLockedValue = !isDeviceLocked(device)
      await updateDevice({ id: device.id, isLocked: nextLockedValue })
      setSelectedIds((previous) => new Set(previous))
    } catch (err) {
      console.error('Failed to update retail player device lock state', err)
    }
  }, [updateDevice])

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

  const handleEnterFolder = useCallback(
    (folderId) => {
      setSearchTerm('')
      if (!folderId) {
        setActiveFolderId(null)
        return
      }
      setActiveFolderId(folderId)
    },
    [setActiveFolderId, setSearchTerm],
  )

  const visibleNodeIds = useMemo(
    () =>
      (visibleNodes || [])
        .map((node) => node?.id)
        .filter((id, index, arr) => id && arr.indexOf(id) === index),
    [visibleNodes],
  )

  useEffect(() => {
    if (lastSelectedId && !visibleNodeIds.includes(lastSelectedId)) {
      setLastSelectedId(null)
    }
  }, [lastSelectedId, visibleNodeIds])

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

  const toggleNodeSelection = useCallback(
    (nodeId, event) => {
      if (!nodeId) {
        return
      }

      const isShiftPressed = Boolean(
        event?.shiftKey || event?.nativeEvent?.shiftKey,
      )

      setSelectedIds((prev) => {
        const next = new Set(prev)
        const nodeIsSelected = next.has(nodeId)

        if (isShiftPressed && visibleNodeIds.length) {
          const anchorId =
            lastSelectedId && visibleNodeIds.includes(lastSelectedId)
              ? lastSelectedId
              : null
          const currentIndex = visibleNodeIds.indexOf(nodeId)

          if (anchorId && currentIndex !== -1) {
            const anchorIndex = visibleNodeIds.indexOf(anchorId)

            if (anchorIndex !== -1) {
              const start = Math.min(anchorIndex, currentIndex)
              const end = Math.max(anchorIndex, currentIndex)
              const shouldSelect = !nodeIsSelected

              for (let index = start; index <= end; index += 1) {
                const rangeId = visibleNodeIds[index]
                if (!rangeId) {
                  continue
                }
                if (shouldSelect) {
                  next.add(rangeId)
                } else {
                  next.delete(rangeId)
                }
              }

              return next
            }
          }
        }

        if (nodeIsSelected) {
          next.delete(nodeId)
        } else {
          next.add(nodeId)
        }
        return next
      })

      setLastSelectedId(nodeId)
    },
    [lastSelectedId, visibleNodeIds],
  )

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

  const isLoading = loading

  const renderRows = (nodes) =>
    nodes.map((node) => {
      if (node.type === 'folder') {
        const deviceCount = countDevices(node)
        const isSelected = selectedIds.has(node.id)
        return (
          <RetailPlayerFolderRow
            key={`folder-row-${node.id}`}
            node={node}
            deviceCount={deviceCount}
            isSelected={isSelected}
            classes={classes}
            onEnterFolder={handleEnterFolder}
            onToggleSelection={toggleNodeSelection}
            onKeyDown={handleRowKeyDown}
            onEdit={handleEditFolder}
            onDeviceDrop={handleDeviceDropOnFolder}
          />
        )
      }
      const isSelected = selectedIds.has(node.id)
      const rowKey = node.treeKey || node.id
      const channelCount = channelCountsByDeviceId?.[node.id]
      const isOnline =
        typeof node.online === 'boolean' ? node.online : deviceStatusMap.get(node.id) ?? null
      return (
        <RetailPlayerDeviceRow
          key={`device-row-${rowKey}`}
          node={node}
          isSelected={isSelected}
          classes={classes}
          channelCount={channelCount}
          onNavigate={handleNavigateToDevice}
          onToggleSelection={toggleNodeSelection}
          onKeyDown={handleRowKeyDown}
          onEdit={handleEditDevice}
          onToggleLock={handleToggleDeviceLock}
          isLocked={isDeviceLocked(node)}
          isOnline={isOnline}
        />
      )
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
        <div className={classes.headerControls}>
          <TextField
            className={classes.searchField}
            variant="outlined"
            size="small"
            placeholder="Search"
            value={searchTerm}
            onChange={(event) => setSearchTerm(event.target.value)}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon fontSize="small" />
                </InputAdornment>
              ),
            }}
            inputProps={{ 'aria-label': 'Search retail player items' }}
          />
          <div className={classes.actions}>
            <Button
              color="primary"
              variant="contained"
              startIcon={<AddIcon />}
              onClick={handleCreateFolder}
            >
              Create Folder
            </Button>
          </div>
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
                onClick={() => handleEnterFolder(null)}
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
                    onClick={() => handleEnterFolder(folder.id)}
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
        {showSelectionRibbon ? (
          <div
            className={classes.selectionRibbon}
            role="status"
            aria-live="polite"
          >
            <Typography
              variant="subtitle2"
              component="p"
              className={classes.selectionSummary}
            >
              {`${selectedCount} item${selectedCount === 1 ? '' : 's'} selected`}
            </Typography>
            <div className={classes.selectionActions}>
              <Button
                variant="contained"
                color="secondary"
                className={classes.selectionPrimaryButton}
                startIcon={<FolderIcon />}
                onClick={() => setAddToFolderDialogOpen(true)}
              >
                Add to Folder
              </Button>
              <Button
                variant="outlined"
                color="inherit"
                className={classes.selectionDeleteButton}
                startIcon={<DeleteOutlineIcon />}
                onClick={() => setDeleteDialogOpen(true)}
              >
                Delete
              </Button>
            </div>
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
              style={{ transform: 'scale(0.8)' }} 
            />
          </div>
          <span>Name</span>
          <span>Type</span>
          <span>Devices / Channels</span>
          <span>QR ID</span>
          <span className={classes.headerActions}>Edit</span>
        </div>
        {isLoading ? (
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
        ) : showingSearchResults ? (
          <div className={classes.emptyState}>
            <Typography variant="body2">
              {`No results found for “${searchTerm.trim()}”.`}
            </Typography>
          </div>
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

      <AddToFolderDialog
        open={addToFolderDialogOpen}
        folders={folders}
        excludeFolderIds={excludedFolderIds}
        selectedCount={selectedCount}
        onClose={handleAddToFolderDialogClose}
        onConfirm={handleAddToFolderConfirm}
      />

      <Dialog
        open={deleteDialogOpen}
        onClose={handleDeleteDialogClose}
        maxWidth="xs"
        fullWidth
        aria-labelledby="retail-player-delete-dialog-title"
      >
        <DialogTitle id="retail-player-delete-dialog-title">
          Delete Selected Items
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {selectedCount === 1
              ? 'Are you sure you want to delete this item? This action cannot be undone.'
              : `Are you sure you want to delete these ${selectedCount} items? This action cannot be undone.`}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteDialogClose}>Cancel</Button>
          <Button
            onClick={handleDeleteConfirm}
            color="secondary"
            variant="contained"
            startIcon={<DeleteOutlineIcon />}
          >
            Delete
          </Button>
        </DialogActions>
      </Dialog>

      <FolderDialog
        open={folderDialog.open}
        onClose={handleFolderDialogClose}
        onSubmit={handleFolderSubmit}
        initialValues={folderDialog.target}
      />
      <DeviceDialog
        open={deviceDialog.open}
        onClose={handleDeviceDialogClose}
        onSubmit={handleDeviceSubmit}
        initialValues={deviceDialog.target}
      />
    </div>
  )
}

export default RetailPlayerDeviceManagement
