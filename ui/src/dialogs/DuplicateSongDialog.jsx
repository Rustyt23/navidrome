import React from 'react'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
} from '@material-ui/core'

import { useTranslate } from 'react-admin'

const DuplicateSongDialog = ({
  open,
  handleSkip,
  handleDuplicate,
  messageKey = 'resources.playlist.message.song_exist',
}) => {
  const translate = useTranslate()

  return (
    <Dialog open={open} aria-labelledby="form-dialog-duplicate-song">
      <DialogTitle id="form-dialog-duplicate-song">
        {translate('resources.playlist.message.duplicate_song')}
      </DialogTitle>
      <DialogContent>{translate(messageKey)}</DialogContent>
      <DialogActions>
        <Button onClick={handleSkip} color="primary">
          {translate('ra.action.skip')}
        </Button>
        <Button onClick={handleDuplicate} color="primary" autoFocus>
          {translate('ra.action.duplicate')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default DuplicateSongDialog
