import React, { useEffect, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  TextField,
  Typography,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import SearchIcon from '@material-ui/icons/Search'
import AddIcon from '@material-ui/icons/Add'
import ClearIcon from '@material-ui/icons/Clear'
import FolderIcon from '@material-ui/icons/Folder'

const useStyles = makeStyles((theme) => ({
  dialogContent: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
    minWidth: 420,
    [theme.breakpoints.down('xs')]: {
      minWidth: 'auto',
    },
  },
  introText: {
    color: theme.palette.text.secondary,
  },
  searchField: {
    '& .MuiOutlinedInput-root': {
      backgroundColor: theme.palette.background.default,
    },
  },
  list: {
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: theme.shape.borderRadius,
    maxHeight: 260,
    overflowY: 'auto',
    backgroundColor: theme.palette.background.paper,
  },
  listItem: {
    paddingTop: theme.spacing(1),
    paddingBottom: theme.spacing(1),
  },
  createItemIcon: {
    color: theme.palette.primary.light,
  },
  emptyState: {
    padding: theme.spacing(2.5),
    textAlign: 'center',
    color: theme.palette.text.secondary,
    width: '100%',
  },
  newFolderBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(0.75, 1.25),
    borderRadius: theme.shape.borderRadius,
    backgroundColor: theme.palette.primary.dark,
    color: theme.palette.primary.contrastText,
    alignSelf: 'flex-start',
  },
  removeNewButton: {
    color: theme.palette.primary.contrastText,
    padding: theme.spacing(0.5),
  },
  footerHelper: {
    color: theme.palette.text.secondary,
  },
}))

const AddToFolderDialog = ({
  open,
  folders,
  excludeFolderIds,
  selectedCount,
  onClose,
  onConfirm,
}) => {
  const classes = useStyles()
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedFolderIds, setSelectedFolderIds] = useState([])
  const [pendingFolderName, setPendingFolderName] = useState('')

  const excludeSet = useMemo(
    () => new Set((excludeFolderIds || []).filter(Boolean)),
    [excludeFolderIds],
  )

  const availableFolders = useMemo(
    () =>
      (folders || []).filter(
        (folder) => folder && !excludeSet.has(folder.id),
      ),
    [folders, excludeSet],
  )

  const normalizedSearch = searchTerm.trim().toLowerCase()

  const filteredFolders = useMemo(() => {
    if (!normalizedSearch) {
      return availableFolders
    }
    return availableFolders.filter((folder) =>
      (folder.name || '').toLowerCase().includes(normalizedSearch),
    )
  }, [availableFolders, normalizedSearch])

  const canCreateNew = useMemo(() => {
    if (!normalizedSearch) {
      return false
    }
    const existsInSelection = selectedFolderIds.some(
      (folderId) => {
        const match = availableFolders.find((folder) => folder.id === folderId)
        return match && match.name.toLowerCase() === normalizedSearch
      },
    )
    const existsInList = availableFolders.some(
      (folder) => (folder.name || '').toLowerCase() === normalizedSearch,
    )
    const matchesPending =
      pendingFolderName &&
      pendingFolderName.toLowerCase() === normalizedSearch
    return !existsInSelection && !existsInList && !matchesPending
  }, [
    availableFolders,
    normalizedSearch,
    pendingFolderName,
    selectedFolderIds,
  ])

  const canSubmit =
    selectedFolderIds.length > 0 || Boolean(pendingFolderName.trim())

  useEffect(() => {
    if (open) {
      setSearchTerm('')
      setSelectedFolderIds([])
      setPendingFolderName('')
    }
  }, [open])

  useEffect(() => {
    setSelectedFolderIds((previous) => {
      if (!previous.length) {
        return previous
      }
      const next = previous.filter(
        (folderId) => folderId && !excludeSet.has(folderId),
      )
      return next.length === previous.length ? previous : next
    })
  }, [excludeSet])

  const handleToggleFolder = (folderId) => {
    setSelectedFolderIds((previous) => {
      if (previous.includes(folderId)) {
        return previous.filter((id) => id !== folderId)
      }
      return [...previous, folderId]
    })
  }

  const handleCreateNew = () => {
    if (!canCreateNew) {
      return
    }
    const trimmed = searchTerm.trim()
    if (!trimmed) {
      return
    }
    setPendingFolderName(trimmed)
    setSearchTerm('')
  }

  const handleRemovePending = () => {
    setPendingFolderName('')
  }

  const handleSubmit = (event) => {
    event.preventDefault()
    if (!canSubmit) {
      return
    }
    onConfirm({
      folderIds: selectedFolderIds,
      newFolderName: pendingFolderName.trim(),
    })
  }

  const handleKeyDown = (event) => {
    if (event.key === 'Enter' && canCreateNew) {
      event.preventDefault()
      handleCreateNew()
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
      PaperProps={{ component: 'form', onSubmit: handleSubmit }}
    >
      <DialogTitle>Add to Folder</DialogTitle>
      <DialogContent className={classes.dialogContent}>
        <Typography variant="body2" className={classes.introText}>
          {selectedCount === 1
            ? 'Choose the folders where this item should appear.'
            : `Choose the folders where these ${selectedCount} items should appear.`}
        </Typography>
        <TextField
          autoFocus
          variant="outlined"
          label="Search folders"
          placeholder="Search folders or type to create new"
          value={searchTerm}
          onChange={(event) => setSearchTerm(event.target.value)}
          onKeyDown={handleKeyDown}
          className={classes.searchField}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
            endAdornment:
              canCreateNew && searchTerm.trim() ? (
                <InputAdornment position="end">
                  <IconButton
                    size="small"
                    onClick={handleCreateNew}
                    title={`Create folder "${searchTerm.trim()}"`}
                  >
                    <AddIcon />
                  </IconButton>
                </InputAdornment>
              ) : null,
          }}
        />

        {pendingFolderName ? (
          <div className={classes.newFolderBadge}>
            <FolderIcon fontSize="small" />
            <span>{pendingFolderName}</span>
            <IconButton
              size="small"
              className={classes.removeNewButton}
              onClick={handleRemovePending}
              aria-label="Remove pending folder"
            >
              <ClearIcon fontSize="small" />
            </IconButton>
          </div>
        ) : null}

        <List className={classes.list} disablePadding>
          {canCreateNew && searchTerm.trim() ? (
            <ListItem
              button
              onClick={handleCreateNew}
              className={classes.listItem}
            >
              <ListItemIcon className={classes.createItemIcon}>
                <AddIcon />
              </ListItemIcon>
              <ListItemText
                primary={`Create folder "${searchTerm.trim()}"`}
              />
            </ListItem>
          ) : null}

          {filteredFolders.length ? (
            filteredFolders.map((folder) => (
              <ListItem
                key={folder.id}
                button
                onClick={() => handleToggleFolder(folder.id)}
                className={classes.listItem}
              >
                <ListItemIcon>
                  <Checkbox
                    edge="start"
                    color="primary"
                    checked={selectedFolderIds.includes(folder.id)}
                    tabIndex={-1}
                  />
                </ListItemIcon>
                <ListItemText primary={folder.name} />
              </ListItem>
            ))
          ) : (
            <ListItem className={classes.listItem} disabled>
              <ListItemText
                primary={
                  <div className={classes.emptyState}>
                    <Typography variant="body2">
                      {normalizedSearch
                        ? 'No folders match this search.'
                        : 'No folders available yet.'}
                    </Typography>
                    {canCreateNew ? (
                      <Typography variant="body2" color="primary">
                        Press Enter to create this folder.
                      </Typography>
                    ) : null}
                  </div>
                }
              />
            </ListItem>
          )}
        </List>

        <Typography variant="caption" className={classes.footerHelper}>
          You can select multiple folders to share these items across locations.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          type="submit"
          color="primary"
          variant="contained"
          disabled={!canSubmit}
        >
          Add
        </Button>
      </DialogActions>
    </Dialog>
  )
}

AddToFolderDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  folders: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
    }),
  ),
  excludeFolderIds: PropTypes.arrayOf(PropTypes.string),
  selectedCount: PropTypes.number,
  onClose: PropTypes.func.isRequired,
  onConfirm: PropTypes.func.isRequired,
}

AddToFolderDialog.defaultProps = {
  folders: [],
  excludeFolderIds: [],
  selectedCount: 0,
}

export default AddToFolderDialog
