import PropTypes from 'prop-types'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
} from '@material-ui/core'
import { useEffect, useMemo, useState } from 'react'

const formatLabel = (folder, currentId) => {
  const prefix = folder.depth > 0 ? `${'\u00A0'.repeat(folder.depth * 2)}• ` : ''
  const name = folder.name || 'Untitled folder'
  if (folder.id === currentId) {
    return `${prefix}${name} (current)`
  }
  return `${prefix}${name}`
}

const MoveItemDialog = ({
  open,
  title,
  folders,
  currentId,
  initialParentId,
  forbiddenIds = [],
  destinationLabel = 'Destination folder',
  rootLabel = 'Root',
  onClose,
  onSubmit,
}) => {
  const [selected, setSelected] = useState(initialParentId ?? '')

  useEffect(() => {
    if (open) {
      setSelected(initialParentId ?? '')
    }
  }, [open, initialParentId])

  const availableFolders = useMemo(() => {
    const forbidden = new Set(forbiddenIds)
    if (currentId) {
      forbidden.add(currentId)
    }
    return folders.filter((folder) => !forbidden.has(folder.id))
  }, [folders, forbiddenIds, currentId])

  const handleSubmit = (event) => {
    event.preventDefault()
    const payload = selected === '' ? null : selected
    onSubmit(payload)
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <form onSubmit={handleSubmit}>
        <DialogTitle>{title}</DialogTitle>
        <DialogContent>
          <FormControl variant="outlined" fullWidth margin="dense">
            <InputLabel id="retail-player-move-folder-label">
              {destinationLabel}
            </InputLabel>
            <Select
              labelId="retail-player-move-folder-label"
              value={selected}
              onChange={(event) => setSelected(event.target.value)}
              label={destinationLabel}
            >
              <MenuItem value="">{rootLabel}</MenuItem>
              {availableFolders.map((folder) => (
                <MenuItem key={folder.id} value={folder.id}>
                  {formatLabel(folder, currentId)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose} color="primary">
            Cancel
          </Button>
          <Button type="submit" color="primary" variant="contained">
            Move
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

MoveItemDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  title: PropTypes.string.isRequired,
  folders: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
      parentId: PropTypes.string,
      depth: PropTypes.number,
    }),
  ).isRequired,
  currentId: PropTypes.string,
  initialParentId: PropTypes.string,
  forbiddenIds: PropTypes.arrayOf(PropTypes.string),
  destinationLabel: PropTypes.string,
  rootLabel: PropTypes.string,
  onClose: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
}

export default MoveItemDialog
