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
import { httpClient } from '../dataProvider'
import { REST_URL } from '../consts'

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

  const getTrackTitle = (track) =>
    track?.title ||
    track?.MediaFile?.title ||
    track?.mediaFile?.title ||
    track?.MediaFile?.name ||
    track?.mediaFile?.name ||
    translate('resources.song.fields.title')

  const notifyPlaylistResult = ({
    playlistName,
    addedCount,
    skippedCount,
    skippedSampleTitles,
  }) => {
    const lines = []

    if (addedCount > 0 && skippedCount > 0) {
      lines.push(
        translate('resources.playlist.message.addedWithSkips', {
          addedCount,
          skippedCount,
          playlistName,
        }),
      )
    } else if (addedCount > 0) {
      lines.push(
        translate('resources.playlist.message.added', {
          addedCount,
          playlistName,
        }),
      )
    } else {
      lines.push(
        translate('resources.playlist.message.allDuplicates', {
          playlistName,
        }),
      )
    }

    if (skippedCount > 0) {
      const sampleTitles = skippedSampleTitles.slice(0, 3)
      sampleTitles.forEach((title) =>
        lines.push(
          translate('resources.playlist.message.duplicateLine', {
            title,
          }),
        ),
      )
      const remaining = skippedCount - sampleTitles.length
      if (remaining > 0) {
        lines.push(
          translate('resources.playlist.message.duplicateLineMore', {
            remaining,
          }),
        )
      }
    }

    notify(lines.join('\n'), {
      type: 'info',
      autoHideDuration: 3000,
      multiLine: true,
    })
  }

  const addTracksToPlaylist = async (
    playlistId,
    playlistName,
    existingTracks = [],
  ) => {
    const existingIds = new Set(
      existingTracks.map((track) => track?.mediaFileId).filter(Boolean),
    )
    const newTrackIds = uniqueSelectedIds.filter(
      (id) => !existingIds.has(id),
    )
    const skippedIds = uniqueSelectedIds.filter((id) => existingIds.has(id))
    const skippedCount = skippedIds.length
    const skippedIdSet = new Set(skippedIds)
    const skippedSampleTitles = []

    if (skippedCount > 0) {
      existingTracks.forEach((track) => {
        if (
          skippedIdSet.has(track?.mediaFileId) &&
          skippedSampleTitles.length < 3
        ) {
          const title = getTrackTitle(track)
          if (!skippedSampleTitles.includes(title)) {
            skippedSampleTitles.push(title)
          }
        }
      })
    }

    if (newTrackIds.length > 0) {
      await dataProvider.create('playlistTrack', {
        data: { ids: newTrackIds },
        filter: { playlist_id: playlistId },
      })
    }

    notifyPlaylistResult({
      playlistName,
      addedCount: newTrackIds.length,
      skippedCount,
      skippedSampleTitles,
    })

    return { addedCount: newTrackIds.length, skippedCount }
  }

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
      return await addTracksToPlaylist(playlistId, playlistName, [])
    } catch (error) {
      notify(`Error: ${error.message}`, { type: 'warning' })
      return null
    }
  }

  const addToExistingPlaylist = async (playlistObject) => {
    try {
      const res = await httpClient(
        `${REST_URL}/playlist/${playlistObject.id}/tracks`,
      )
      const tracks = res?.json || []
      return await addTracksToPlaylist(
        playlistObject.id,
        playlistObject.name,
        tracks,
      )
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
