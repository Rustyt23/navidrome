import React, { useMemo, useState } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  useDataProvider,
  useNotify,
  useRefresh,
  useTranslate,
} from 'react-admin'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  makeStyles,
} from '@material-ui/core'
import { closeAddToPlaylist } from '../actions'
import { SelectPlaylistInput } from './SelectPlaylistInput'
import {
  addTracksToPlaylist,
  formatPlaylistToast,
} from '../playlist/addTracksToPlaylist'

const useStyles = makeStyles({
  dialogPaper: {
    height: '26em',
    maxHeight: '26em',
  },
  dialogContent: {
    height: '17.5em',
    overflowY: 'auto',
    paddingTop: '0.5em',
    paddingBottom: '0.5em',
  },
})

export const AddToPlaylistDialog = () => {
  const classes = useStyles()
  const { open, selectedIds = [], onSuccess } = useSelector(
    (state) => state.addToPlaylistDialog,
  )
  const dispatch = useDispatch()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const [value, setValue] = useState([])
  const [isSubmitting, setIsSubmitting] = useState(false)
  const dataProvider = useDataProvider()
  const uniqueSelectedIds = useMemo(
    () => Array.from(new Set(selectedIds)),
    [selectedIds],
  )

  const createAndAddToPlaylist = async (playlistObject) => {
    try {
      const response = await dataProvider.create('playlist', {
        data: { name: playlistObject.name },
      })
      const playlistId = response?.data?.id
      const playlistName = response?.data?.name || playlistObject.name
      if (!playlistId) {
        throw new Error('Missing playlist id')
      }
      const result = await addTracksToPlaylist(
        dataProvider,
        playlistId,
        { ids: uniqueSelectedIds },
        { skipDuplicates: true, existingTrackIds: new Set() },
      )
      notify(formatPlaylistToast(result), {
        type: 'info',
        autoHideDuration: 3000,
      })
      return result
    } catch (error) {
      notify(`Error: ${error.message}`, { type: 'warning' })
      return null
    }
  }

  const addToExistingPlaylist = async (playlistObject) => {
    try {
      const result = await addTracksToPlaylist(
        dataProvider,
        playlistObject.id,
        { ids: uniqueSelectedIds },
        { skipDuplicates: true },
      )
      notify(formatPlaylistToast(result), {
        type: 'info',
        autoHideDuration: 3000,
      })
      return result
    } catch (error) {
      notify('ra.page.error', { type: 'warning' })
      return null
    }
  }

  const handleSubmit = async (e) => {
    e.stopPropagation()
    if (!value.length || isSubmitting) {
      return
    }

    setIsSubmitting(true)
    let addedAcrossPlaylists = 0

    try {
      for (const playlistObject of value) {
        let result = null
        if (playlistObject.id) {
          result = await addToExistingPlaylist(playlistObject)
        } else {
          result = await createAndAddToPlaylist(playlistObject)
        }
        if (result?.addedCount) {
          addedAcrossPlaylists += result.addedCount
        }
      }
      if (addedAcrossPlaylists > 0) {
        refresh()
      }
      onSuccess && onSuccess(value, addedAcrossPlaylists)
    } finally {
      setIsSubmitting(false)
      setValue([])
      dispatch(closeAddToPlaylist())
    }
  }

  const handleClickClose = (e) => {
    setValue([])
    dispatch(closeAddToPlaylist())
    e.stopPropagation()
  }

  const handleChange = (pls) => {
    setValue(pls)
  }

  return (
    <>
      <Dialog
        open={open}
        onClose={handleClickClose}
        aria-labelledby="form-dialog-new-playlist"
        fullWidth={true}
        maxWidth={'sm'}
        classes={{
          paper: classes.dialogPaper,
        }}
      >
        <DialogTitle id="form-dialog-new-playlist">
          {translate('resources.playlist.actions.selectPlaylist')}
        </DialogTitle>
        <DialogContent className={classes.dialogContent}>
          <SelectPlaylistInput onChange={handleChange} />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClickClose} color="primary">
            {translate('ra.action.cancel')}
          </Button>
          <Button
            onClick={handleSubmit}
            color="primary"
            disabled={isSubmitting || value.length === 0}
            data-testid="playlist-add"
          >
            {translate('ra.action.add')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
