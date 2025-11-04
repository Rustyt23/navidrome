import { useCallback, useMemo, useState } from 'react'
import {
  Box,
  Breadcrumbs,
  Button,
  Card,
  CardContent,
  CircularProgress,
  IconButton,
  Link,
  makeStyles,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
  TextField,
  Tooltip,
  Typography,
} from '@material-ui/core'
import FolderIcon from '@material-ui/icons/Folder'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import EditIcon from '@material-ui/icons/Edit'
import DeleteIcon from '@material-ui/icons/Delete'
import OpenInNewIcon from '@material-ui/icons/OpenInNew'
import MoveToInboxIcon from '@material-ui/icons/MoveToInbox'
import Switch from '@material-ui/core/Switch'
import { useHistory } from 'react-router-dom'
import { Title, useTranslate } from 'react-admin'
import useRetailPlayerDevices from '../retailPlayer/useRetailPlayerDevices'
import FolderFormDialog from './FolderFormDialog'
import MoveItemDialog from './MoveItemDialog'
import ConfirmDialog from './ConfirmDialog'
import useRetailPlayerFolderState from './useRetailPlayerFolderState'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(3),
    [theme.breakpoints.down('sm')]: {
      padding: theme.spacing(2),
    },
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing(3),
    flexWrap: 'wrap',
    gap: theme.spacing(2),
  },
  breadcrumbs: {
    '& a': {
      color: theme.palette.primary.main,
    },
  },
  actionsRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(2),
  },
  tableHead: {
    backgroundColor: theme.palette.background.paper,
  },
  typeCell: {
    width: theme.spacing(6),
  },
  nameCell: {
    minWidth: 220,
  },
  ownerCell: {
    minWidth: 160,
  },
  updatedCell: {
    minWidth: 160,
  },
  publicCell: {
    width: theme.spacing(10),
  },
  actionsCell: {
    width: theme.spacing(18),
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  emptyState: {
    padding: theme.spacing(4),
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  loadingWrapper: {
    display: 'flex',
    justifyContent: 'center',
    padding: theme.spacing(4),
  },
}))

const HEADERS = [
  { id: 'type', labelKey: 'retailPlayer.folders.columns.type', defaultLabel: 'Type', sortable: false },
  { id: 'name', labelKey: 'retailPlayer.folders.columns.name', defaultLabel: 'Name', sortable: true },
  {
    id: 'ownerName',
    labelKey: 'retailPlayer.folders.columns.ownerName',
    defaultLabel: 'Owner Name',
    sortable: true,
  },
  {
    id: 'updatedAt',
    labelKey: 'retailPlayer.folders.columns.updatedAt',
    defaultLabel: 'Updated',
    sortable: true,
  },
  {
    id: 'public',
    labelKey: 'retailPlayer.folders.columns.public',
    defaultLabel: 'Public',
    sortable: true,
  },
  {
    id: 'actions',
    labelKey: 'retailPlayer.folders.columns.actions',
    defaultLabel: 'Actions',
    sortable: false,
  },
]

const formatDate = (value) => {
  if (!value) {
    return '—'
  }
  try {
    return new Date(value).toLocaleString()
  } catch (err) {
    return value
  }
}

const RetailPlayerFolderPage = () => {
  const classes = useStyles()
  const history = useHistory()
  const [currentFolderId, setCurrentFolderId] = useState(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [orderBy, setOrderBy] = useState('name')
  const [orderDirection, setOrderDirection] = useState('asc')
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editFolderId, setEditFolderId] = useState(null)
  const [moveContext, setMoveContext] = useState(null)
  const [confirmContext, setConfirmContext] = useState(null)

  const { devices, isLoading, error } = useRetailPlayerDevices()

  const folderState = useRetailPlayerFolderState(devices)
  const translate = useTranslate()

  const items = useMemo(
    () =>
      folderState.getItems(currentFolderId, searchTerm, orderBy, orderDirection),
    [folderState, currentFolderId, searchTerm, orderBy, orderDirection],
  )

  const breadcrumbTrail = useMemo(() => folderState.getBreadcrumb(currentFolderId), [folderState, currentFolderId])

  const handleSortChange = (columnId) => {
    if (orderBy === columnId) {
      setOrderDirection((prev) => (prev === 'asc' ? 'desc' : 'asc'))
    } else {
      setOrderBy(columnId)
      setOrderDirection('asc')
    }
  }

  const handleCreateFolder = useCallback(
    (name) => {
      folderState.createFolder(name, currentFolderId)
      setIsCreateOpen(false)
    },
    [folderState, currentFolderId],
  )

  const handleRenameFolder = useCallback(
    (name) => {
      if (!editFolderId) return
      folderState.updateFolder(editFolderId, { name })
      setEditFolderId(null)
    },
    [folderState, editFolderId],
  )

  const handleDeleteFolder = useCallback(() => {
    if (!confirmContext) return
    folderState.deleteFolder(confirmContext.id)
    setConfirmContext(null)
    if (confirmContext.id === currentFolderId) {
      setCurrentFolderId(null)
    }
  }, [folderState, confirmContext, currentFolderId])

  const handleMove = useCallback(
    (targetParentId) => {
      if (!moveContext) return
      if (moveContext.type === 'folder') {
        folderState.moveFolder(moveContext.id, targetParentId)
      } else if (moveContext.type === 'device') {
        folderState.moveDevice(moveContext.id, targetParentId)
      }
      setMoveContext(null)
    },
    [folderState, moveContext],
  )

  const openFolder = (folderId) => {
    setCurrentFolderId(folderId)
    setSearchTerm('')
  }

  const moveDialogInitialParent = useMemo(() => {
    if (!moveContext) return null
    if (moveContext.type === 'folder') {
      return folderState.getFolderById(moveContext.id)?.parentId ?? null
    }
    return folderState.assignments[moveContext.id] ?? null
  }, [moveContext, folderState])

  const forbiddenFolderTargets = useMemo(() => {
    if (!moveContext || moveContext.type !== 'folder') {
      return []
    }
    return Array.from(folderState.getDescendantIds(moveContext.id))
  }, [moveContext, folderState])

  return (
    <Box className={classes.root}>
      <Title title={translate('retailPlayer.folders.title', { _: 'Retail Player Device Folders' })} />
      <div className={classes.header}>
        <Typography variant="h4" component="h1">
          {translate('retailPlayer.folders.title', { _: 'Retail Player Device Folders' })}
        </Typography>
        <Button color="primary" variant="contained" onClick={() => setIsCreateOpen(true)}>
          {translate('retailPlayer.folders.create', { _: 'Create folder' })}
        </Button>
      </div>
      <Card>
        <CardContent>
          <div className={classes.actionsRow}>
            <Breadcrumbs aria-label="breadcrumb" className={classes.breadcrumbs}>
              <Link
                component="button"
                onClick={() => {
                  setCurrentFolderId(null)
                  setSearchTerm('')
                }}
              >
                {translate('retailPlayer.folders.root', { _: 'Root' })}
              </Link>
              {breadcrumbTrail.map((folder) => (
                <Link
                  key={folder.id}
                  component="button"
                  onClick={() => openFolder(folder.id)}
                >
                  {folder.name}
                </Link>
              ))}
            </Breadcrumbs>
            <TextField
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
              variant="outlined"
              size="small"
              placeholder={translate('retailPlayer.folders.searchPlaceholder', {
                _: 'Search folders or devices',
              })}
              InputProps={{
                'aria-label': translate('retailPlayer.folders.searchPlaceholder', {
                  _: 'Search folders or devices',
                }),
              }}
            />
          </div>
          {isLoading ? (
            <div className={classes.loadingWrapper}>
              <CircularProgress size={24} />
            </div>
          ) : error ? (
            <Typography color="error">
              {translate('retailPlayer.folders.loadError', {
                _: 'Unable to load retail player devices. Please try again later.',
              })}
            </Typography>
          ) : (
            <TableContainer component={Paper}>
              <Table size="small">
                <TableHead className={classes.tableHead}>
                  <TableRow>
                    {HEADERS.map((column) => (
                      <TableCell
                        key={column.id}
                        className={
                          column.id === 'type'
                            ? classes.typeCell
                            : column.id === 'name'
                            ? classes.nameCell
                            : column.id === 'ownerName'
                            ? classes.ownerCell
                            : column.id === 'updatedAt'
                            ? classes.updatedCell
                            : column.id === 'public'
                            ? classes.publicCell
                            : undefined
                        }
                      >
                        {column.sortable ? (
                          <TableSortLabel
                            active={orderBy === column.id}
                            direction={orderBy === column.id ? orderDirection : 'asc'}
                            onClick={() => handleSortChange(column.id)}
                          >
                            {translate(column.labelKey, { _: column.defaultLabel })}
                          </TableSortLabel>
                        ) : (
                          translate(column.labelKey, { _: column.defaultLabel })
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {items.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={HEADERS.length} className={classes.emptyState}>
                        {translate('retailPlayer.folders.empty', {
                          _: 'No folders or devices in this location yet.',
                        })}
                      </TableCell>
                    </TableRow>
                  ) : (
                    items.map((item) => (
                      <TableRow key={`${item.type}-${item.id}`} hover>
                        <TableCell className={classes.typeCell}>
                          {item.type === 'folder' ? (
                            <FolderIcon color="primary" />
                          ) : (
                            <SpeakerGroupIcon color="action" />
                          )}
                        </TableCell>
                        <TableCell className={classes.nameCell}>
                          {item.type === 'folder' ? (
                            <Link component="button" onClick={() => openFolder(item.id)}>
                              {item.name}
                            </Link>
                          ) : (
                            item.name
                          )}
                        </TableCell>
                        <TableCell className={classes.ownerCell}>
                          {item.ownerName || '—'}
                        </TableCell>
                        <TableCell className={classes.updatedCell}>{formatDate(item.updatedAt)}</TableCell>
                        <TableCell className={classes.publicCell}>
                          {item.type === 'folder' ? (
                            <Switch
                              color="primary"
                              size="small"
                              checked={Boolean(item.public)}
                              onClick={(event) => event.stopPropagation()}
                              onChange={() => folderState.toggleFolderPublic(item.id)}
                            />
                          ) : (
                            '—'
                          )}
                        </TableCell>
                        <TableCell>
                          <div className={classes.actionsCell}>
                            {item.type === 'folder' ? (
                              <Tooltip title="Open">
                              <IconButton size="small" onClick={() => openFolder(item.id)}>
                                <OpenInNewIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          ) : (
                              <Tooltip
                                title={translate('retailPlayer.folders.viewDevice', { _: 'View device' })}
                              >
                                <IconButton
                                  size="small"
                                  onClick={() => {
                                    const slug = item.data.slug || item.data.name || item.data.id
                                    history.push(`/retailplayer/${encodeURIComponent(slug)}`)
                                  }}
                                >
                                  <OpenInNewIcon fontSize="small" />
                                </IconButton>
                              </Tooltip>
                            )}
                            <Tooltip
                              title={
                                item.type === 'folder'
                                  ? translate('retailPlayer.folders.moveFolder', { _: 'Move folder' })
                                  : translate('retailPlayer.folders.moveDevice', { _: 'Move device' })
                              }
                            >
                              <IconButton
                                size="small"
                                onClick={() =>
                                  setMoveContext({
                                    id: item.id,
                                    type: item.type,
                                  })
                                }
                              >
                                <MoveToInboxIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                            {item.type === 'folder' ? (
                              <>
                                <Tooltip
                                  title={translate('retailPlayer.folders.rename', { _: 'Rename folder' })}
                                >
                                  <IconButton size="small" onClick={() => setEditFolderId(item.id)}>
                                    <EditIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                                <Tooltip
                                  title={translate('retailPlayer.folders.delete', { _: 'Delete folder' })}
                                >
                                  <IconButton
                                    size="small"
                                    onClick={() =>
                                      setConfirmContext({
                                        id: item.id,
                                        name: item.name,
                                      })
                                    }
                                  >
                                    <DeleteIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                              </>
                            ) : null}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </CardContent>
      </Card>
      <FolderFormDialog
        open={isCreateOpen}
        title={translate('retailPlayer.folders.create', { _: 'Create folder' })}
        label={translate('retailPlayer.folders.create', { _: 'Create folder' })}
        onClose={() => setIsCreateOpen(false)}
        onSubmit={handleCreateFolder}
      />
      <FolderFormDialog
        open={Boolean(editFolderId)}
        title={translate('retailPlayer.folders.rename', { _: 'Rename folder' })}
        label={translate('retailPlayer.folders.rename', { _: 'Rename folder' })}
        initialName={editFolderId ? folderState.getFolderById(editFolderId)?.name || '' : ''}
        onClose={() => setEditFolderId(null)}
        onSubmit={handleRenameFolder}
      />
      <MoveItemDialog
        open={Boolean(moveContext)}
        title={
          moveContext?.type === 'folder'
            ? translate('retailPlayer.folders.moveFolder', { _: 'Move folder' })
            : translate('retailPlayer.folders.moveDevice', { _: 'Move device' })
        }
        folders={folderState.folderOptions}
        currentId={moveContext?.type === 'folder' ? moveContext.id : undefined}
        initialParentId={moveDialogInitialParent || undefined}
        forbiddenIds={forbiddenFolderTargets}
        destinationLabel={translate('retailPlayer.folders.destination', { _: 'Destination folder' })}
        rootLabel={translate('retailPlayer.folders.root', { _: 'Root' })}
        onClose={() => setMoveContext(null)}
        onSubmit={handleMove}
      />
      <ConfirmDialog
        open={Boolean(confirmContext)}
        title={translate('retailPlayer.folders.delete', { _: 'Delete folder' })}
        message={
          confirmContext
            ? translate('retailPlayer.folders.deleteMessage', {
                name: confirmContext.name,
                _: `Deleting "${confirmContext.name}" will move any nested items back to the parent folder.`,
              })
            : ''
        }
        confirmLabel={translate('retailPlayer.folders.delete', { _: 'Delete folder' })}
        onClose={() => setConfirmContext(null)}
        onConfirm={handleDeleteFolder}
      />
    </Box>
  )
}

export default RetailPlayerFolderPage
