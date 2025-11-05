import React, { useCallback, useEffect, useMemo, useState } from 'react'
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
import SearchIcon from '@material-ui/icons/Search'
import DeleteOutlineIcon from '@material-ui/icons/DeleteOutline'
import Breadcrumbs from '@material-ui/core/Breadcrumbs'
import Link from '@material-ui/core/Link'
import clsx from 'clsx'
import PropTypes from 'prop-types'
import { useHistory } from 'react-router-dom'
import { useRetailPlayerDeviceStore } from './RetailPlayerDeviceStoreContext'
import AddToFolderDialog from './AddToFolderDialog'

const useStyles = makeStyles((theme) => ({
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
      '64px minmax(220px, 2fr) minmax(140px, 1fr) minmax(140px, 1fr) minmax(96px, 0.8fr)',
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
    gap: theme.spacing (1),
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
      '56px minmax(180px, 2fr) minmax(120px, 1fr) minmax(120px, 1fr) 72px',
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
  parentOptions,
  initialValues,
}) => {
  const normalizeInitialFolders = useCallback((values) => {
    if (!values) {
      return []
    }
    if (Array.isArray(values.folderIds)) {
      return values.folderIds.filter(Boolean)
    }
    if (values.folderId) {
      return [values.folderId].filter(Boolean)
    }
    return []
  }, [])

  const [form, setForm] = useState(() => ({
    name: initialValues?.name || '',
    folderIds: normalizeInitialFolders(initialValues),
  }))

  useEffect(() => {
    setForm({
      name: initialValues?.name || '',
      folderIds: normalizeInitialFolders(initialValues),
    })
  }, [initialValues, open, normalizeInitialFolders])

  const isRemote = initialValues?.source !== 'local'

  const handleNameChange = (event) => {
    setForm((prev) => ({ ...prev, name: event.target.value }))
  }

  const handleFolderChange = (event) => {
    const value = event.target.value
    const nextValue = Array.isArray(value)
      ? value.filter(Boolean)
      : value
      ? [value]
      : []
    setForm((prev) => ({ ...prev, folderIds: nextValue }))
  }

  const handleSubmit = () => {
    if (!form.name.trim()) {
      return
    }
    const normalizedFolderIds = Array.from(new Set(form.folderIds.filter(Boolean)))
    const primaryFolderId = normalizedFolderIds[0] || null
    onSubmit({
      name: form.name.trim(),
      folderIds: normalizedFolderIds,
      folderId: primaryFolderId,
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
          <FormControl variant="outlined" fullWidth>
            <InputLabel id="device-folder-label">Folder</InputLabel>
            <Select
              labelId="device-folder-label"
              multiple
              value={form.folderIds}
              onChange={handleFolderChange}
              label="Folder"
              renderValue={(selected) => {
                if (!Array.isArray(selected) || !selected.length) {
                  return 'None'
                }
                const labels = parentOptions
                  .filter((option) => selected.includes(option.id))
                  .map((option) => option.name)
                return labels.join(', ')
              }}
            >
              {parentOptions.map((option) => (
                <MenuItem key={option.id} value={option.id}>
                  <Checkbox
                    color="primary"
                    checked={form.folderIds.includes(option.id)}
                  />
                  <ListItemText primary={option.name} />
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
    folderIds: PropTypes.arrayOf(PropTypes.string),
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
    actions: { createFolder, updateFolder, createDevice, updateDevice, deleteNodes },
  } = useRetailPlayerDeviceStore()
  const [menuAnchor, setMenuAnchor] = useState(null)
  const [folderDialog, setFolderDialog] = useState({
    open: false,
    target: null,
    parentId: null,
  })
  const [deviceDialog, setDeviceDialog] = useState({ open: false, target: null })
  const [activeFolderId, setActiveFolderId] = useState(null)
  const [selectedIds, setSelectedIds] = useState(() => new Set())
  const [searchTerm, setSearchTerm] = useState('')
  const [addToFolderDialogOpen, setAddToFolderDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)

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
    ({ folderIds: incomingFolderIds, newFolderName }) => {
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
        const newFolder = createFolder({
          name: trimmedNewFolderName,
          parentId: parentForNewFolder,
        })
        if (newFolder && newFolder.id) {
          folderIdSet.add(newFolder.id)
        }
      }

      const targetFolderIds = Array.from(folderIdSet)
      if (!targetFolderIds.length) {
        setAddToFolderDialogOpen(false)
        return
      }

      selectedDeviceIds.forEach((deviceId) => {
        const device = deviceMap.get(deviceId)
        if (!device) {
          return
        }
        const existingIds = Array.isArray(device.folderIds)
          ? device.folderIds
          : []
        const mergedIds = Array.from(new Set([...existingIds, ...targetFolderIds]))
        const changed =
          mergedIds.length !== existingIds.length ||
          mergedIds.some((id, index) => id !== existingIds[index])
        if (changed) {
          updateDevice({ id: deviceId, folderIds: mergedIds })
        }
      })

      const parentFolderId = targetFolderIds[0] || null
      if (parentFolderId) {
        selectedFolderIds.forEach((folderId) => {
          if (folderId === parentFolderId) {
            return
          }
          const folder = folderMap.get(folderId)
          if (folder && folder.parentId === parentFolderId) {
            return
          }
          updateFolder({ id: folderId, parentId: parentFolderId })
        })
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

  const handleDeleteConfirm = useCallback(() => {
    if (!selectedFolderIds.length && !selectedDeviceIds.length) {
      setDeleteDialogOpen(false)
      return
    }

    deleteNodes({
      folderIds: selectedFolderIds,
      deviceIds: selectedDeviceIds,
    })

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

  const openMenu = (event) => {
    setMenuAnchor(event.currentTarget)
  }

  const closeMenu = () => {
    setMenuAnchor(null)
  }

  const handleCreateFolder = () => {
    closeMenu()
    setFolderDialog({ open: true, target: null, parentId: activeFolderId })
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
    setFolderDialog({ open: false, target: null, parentId: null })
  }

  const handleDeviceDialogClose = () => {
    setDeviceDialog({ open: false, target: null })
  }

  const handleFolderSubmit = (values) => {
    if (folderDialog.target) {
      updateFolder({ id: folderDialog.target.id, name: values.name })
    } else {
      createFolder({ ...values, parentId: folderDialog.parentId || null })
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

  const renderRows = (nodes) =>
    nodes.map((node) => {
      if (node.type === 'folder') {
        const deviceCount = countDevices(node)
        const isSelected = selectedIds.has(node.id)
        return (
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
                  <EditIcon style={{ fontSize: 15 }} />
                </IconButton>
              </Tooltip>
            </div>
          </div>
        )
      }
      const isSelected = selectedIds.has(node.id)
      const rowKey = node.treeKey || node.id
      return (
        <div
          key={`device-row-${rowKey}`}
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
              style={{ transform: 'scale(0.8)' }}
            />
          </div>
          <div className={classes.nameCell}>
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
                <EditIcon style={{ fontSize: 15 }} />
              </IconButton>
            </Tooltip>
          </div>
        </div>
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
            placeholder="Search folders and devices"
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
        parentOptions={folderOptions}
        initialValues={deviceDialog.target}
      />
    </div>
  )
}

export default RetailPlayerDeviceManagement
