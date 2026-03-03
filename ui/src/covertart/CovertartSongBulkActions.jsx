import React, { useMemo, useState } from 'react'
import { Button } from '@material-ui/core'
import { BiDownload } from 'react-icons/bi'
import { BiEdit } from 'react-icons/bi'
import { useListContext, useNotify, useTranslate } from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

const CovertartSongBulkActions = ({ onUnselectItems, onSpotifyCoverUpdated }) => {
  const classes = useStyles()
  const notify = useNotify()
  const translate = useTranslate()
  const { selectedIds = [] } = useListContext()
  const [isLoadingMusicBrainz, setIsLoadingMusicBrainz] = useState(false)
  const [isLoadingSpotify, setIsLoadingSpotify] = useState(false)

  const payload = useMemo(
    () => ({ songIds: selectedIds.map((id) => String(id)) }),
    [selectedIds],
  )

  const startFetch = (url, setLoading, successKey) => {
    setLoading(true)
    httpClient(url, {
      method: 'POST',
      body: JSON.stringify(payload),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    })
      .then(({ status }) => {
        if (status === 202) {
          notify(successKey, 'info')
          onUnselectItems()
          return
        }

        if (status === 409) {
          notify('activity.musicbrainz.alreadyRunning', 'warning')
          return
        }

        notify('activity.musicbrainz.failed', 'warning')
      })
      .catch(() => notify('activity.musicbrainz.failed', 'warning'))
      .finally(() => setLoading(false))
  }

  const editSpotifyUrl = () => {
    if (selectedIds.length === 0) {
      return
    }

    const spotifyUrl = window.prompt(
      translate('activity.musicbrainz.spotifyEditPrompt'),
      '',
    )
    if (!spotifyUrl || !spotifyUrl.trim()) {
      return
    }

    setIsLoadingSpotify(true)
    httpClient('/api/metadata/musicbrainz/spotify/cover', {
      method: 'POST',
      body: JSON.stringify({
        songIds: selectedIds.map((id) => String(id)),
        spotifyUrl: spotifyUrl.trim(),
      }),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    })
      .then(({ status }) => {
        if (status === 200) {
          notify('activity.musicbrainz.spotifyCoverUpdated', 'info')
          onSpotifyCoverUpdated?.()
          onUnselectItems()
          return
        }
        notify('activity.musicbrainz.spotifyCoverUpdateFailed', 'warning')
      })
      .catch(() => notify('activity.musicbrainz.spotifyCoverUpdateFailed', 'warning'))
      .finally(() => setIsLoadingSpotify(false))
  }

  return (
    <>
      <Button
        className={classes.button}
        variant="outlined"
        color="primary"
        startIcon={<BiEdit />}
        disabled={selectedIds.length === 0 || isLoadingMusicBrainz || isLoadingSpotify}
        onClick={editSpotifyUrl}
      >
        {translate('activity.musicbrainz.editSpotifyUrl')}
      </Button>
      <Button
        className={classes.button}
        startIcon={<BiDownload />}
        disabled={selectedIds.length === 0 || isLoadingMusicBrainz || isLoadingSpotify}
        onClick={() =>
          startFetch(
            '/api/metadata/musicbrainz/fetch',
            setIsLoadingMusicBrainz,
            'activity.musicbrainz.started',
          )
        }
      >
        {translate('activity.musicbrainz.fetch')}
      </Button>
      <Button
        className={classes.button}
        startIcon={<BiDownload />}
        disabled={selectedIds.length === 0 || isLoadingMusicBrainz || isLoadingSpotify}
        onClick={() =>
          startFetch(
            '/api/metadata/musicbrainz/spotify/fetch',
            setIsLoadingSpotify,
            'activity.musicbrainz.spotifyStarted',
          )
        }
      >
        {translate('activity.musicbrainz.fetchSpotify')}
      </Button>
    </>
  )
}

export default CovertartSongBulkActions
