import PropTypes from 'prop-types'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
} from '@material-ui/core'
import { useState, useEffect } from 'react'

const FolderFormDialog = ({
  open,
  title,
  label,
  initialName = '',
  onClose,
  onSubmit,
}) => {
  const [name, setName] = useState(initialName)
  const [error, setError] = useState('')

  useEffect(() => {
    if (open) {
      setName(initialName)
      setError('')
    }
  }, [open, initialName])

  const handleSubmit = (event) => {
    event.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) {
      setError('Name is required')
      return
    }
    onSubmit(trimmed)
  }

  const handleClose = () => {
    setName(initialName)
    setError('')
    onClose()
  }

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="xs">
      <form onSubmit={handleSubmit}>
        <DialogTitle>{title}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            label={label}
            value={name}
            onChange={(event) => {
              setName(event.target.value)
              if (error) {
                setError('')
              }
            }}
            error={Boolean(error)}
            helperText={error}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose} color="primary">
            Cancel
          </Button>
          <Button type="submit" color="primary" variant="contained">
            Save
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

FolderFormDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  title: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  initialName: PropTypes.string,
  onClose: PropTypes.func.isRequired,
  onSubmit: PropTypes.func.isRequired,
}

export default FolderFormDialog
