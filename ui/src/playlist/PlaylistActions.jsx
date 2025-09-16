import React from 'react'
import { useDispatch } from 'react-redux'
import {
  Button,
  sanitizeListRestProps,
  TopToolbar,
  useTranslate,
  useDataProvider,
  useNotify,
} from 'react-admin'
import {
  useMediaQuery,
  makeStyles,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
} from '@material-ui/core'
import MuiButton from '@material-ui/core/Button'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import ShuffleIcon from '@material-ui/icons/Shuffle'
import CloudDownloadOutlinedIcon from '@material-ui/icons/CloudDownloadOutlined'
import { RiPlayListAddFill, RiPlayList2Fill } from 'react-icons/ri'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import ShareIcon from '@material-ui/icons/Share'
import PublishIcon from '@material-ui/icons/Publish'
import { httpClient } from '../dataProvider'
import {
  playNext,
  addTracks,
  playTracks,
  shuffleTracks,
  openDownloadMenu,
  DOWNLOAD_MENU_PLAY,
  openShareMenu,
} from '../actions'
import { M3U_MIME_TYPE, REST_URL } from '../consts'
import PropTypes from 'prop-types'
import { formatBytes } from '../utils'
import config from '../config'
import { ToggleFieldsMenu } from '../common'

const useStyles = makeStyles((theme) => ({
  toolbar: { display: 'flex', justifyContent: 'space-between', width: '100%' },
  publishDialogActions: {
    padding: theme.spacing(2, 3),
  },
  publishConfirmButton: {
    color: theme.palette.common.white,
    '&:hover, &:focus, &:focus-visible': {
      backgroundColor: 'rgba(128, 128, 128, 0.3)',
      color: theme.palette.common.white,
    },
  },
  publishCancelButton: {
    color: theme.palette.common.white,
    '&:hover, &:focus, &:focus-visible': {
      color: theme.palette.common.white,
    },
  },
}))

const PlaylistActions = ({ className, ids, data, record, ...rest }) => {
  const dispatch = useDispatch()
  const translate = useTranslate()
  const classes = useStyles()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  const [isPublishDialogOpen, setPublishDialogOpen] = React.useState(false)
  const [isPublishing, setPublishing] = React.useState(false)

  const getAllSongsAndDispatch = React.useCallback(
    (action) => {
      if (ids?.length === record.songCount) {
        return dispatch(action(data, ids))
      }

      dataProvider
        .getList('playlistTrack', {
          pagination: { page: 1, perPage: 0 },
          sort: { field: 'id', order: 'ASC' },
          filter: { playlist_id: record.id },
        })
        .then((res) => {
          const data = res.data.reduce(
            (acc, curr) => ({ ...acc, [curr.id]: curr }),
            {},
          )
          dispatch(action(data))
        })
        .catch(() => {
          notify('ra.page.error', 'warning')
        })
    },
    [dataProvider, dispatch, record, data, ids, notify],
  )

  const handlePlay = React.useCallback(() => {
    getAllSongsAndDispatch(playTracks)
  }, [getAllSongsAndDispatch])

  const handlePlayNext = React.useCallback(() => {
    getAllSongsAndDispatch(playNext)
  }, [getAllSongsAndDispatch])

  const handlePlayLater = React.useCallback(() => {
    getAllSongsAndDispatch(addTracks)
  }, [getAllSongsAndDispatch])

  const handleShuffle = React.useCallback(() => {
    getAllSongsAndDispatch(shuffleTracks)
  }, [getAllSongsAndDispatch])

  const handleShare = React.useCallback(() => {
    dispatch(openShareMenu([record.id], 'playlist', record.name))
  }, [dispatch, record])

  const handleDownload = React.useCallback(() => {
    dispatch(openDownloadMenu(record, DOWNLOAD_MENU_PLAY))
  }, [dispatch, record])

  const handleExport = React.useCallback(
    () =>
      httpClient(`${REST_URL}/playlist/${record.id}/tracks`, {
        headers: new Headers({ Accept: M3U_MIME_TYPE }),
      }).then((res) => {
        const blob = new Blob([res.body], { type: M3U_MIME_TYPE })
        const url = window.URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `${record.name}.m3u`
        document.body.appendChild(link)
        link.click()
        link.parentNode.removeChild(link)
      }),
    [record],
  )

  const handlePublishClick = React.useCallback(() => {
    setPublishDialogOpen(true)
  }, [])

  const handlePublishClose = React.useCallback(() => {
    setPublishDialogOpen(false)
  }, [])

  const handlePublishConfirm = React.useCallback(() => {
    setPublishDialogOpen(false)

    if (!record?.id) {
      return
    }

    const parsedCount = Number(record?.songCount ?? 0)
    const trackCount = Number.isFinite(parsedCount) ? parsedCount : 0

    const rawName = record?.name ?? ''
    const normalizedParts = rawName
      .replace(/\\/g, '/')
      .split('/')
      .map((part) => part.trim())
      .filter((part) => part.length > 0)

    let playlistDisplayName = normalizedParts.length
      ? normalizedParts[normalizedParts.length - 1]
      : rawName.trim()

    if (!playlistDisplayName) {
      playlistDisplayName = translate('resources.playlist.name', { smart_count: 1 })
    }

    setPublishing(true)

    httpClient(`${REST_URL}/playlist/${record.id}/publish`, {
      method: 'POST',
    })
      .then(() => {
        const message = translate(
          'resources.playlist.notifications.published_to_sync',
          {
            smart_count: trackCount,
            playlist: playlistDisplayName,
          },
        )
        notify(message, 'info', { autoHideDuration: 3000 })
      })
      .catch(() => {
        notify(
          translate('resources.playlist.notifications.publish_error'),
          'warning',
        )
      })
      .finally(() => {
        setPublishing(false)
      })
  }, [record, notify, translate])

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      <div className={classes.toolbar}>
        <div>
          <Button
            onClick={handlePlay}
            label={translate('resources.album.actions.playAll')}
          >
            <PlayArrowIcon />
          </Button>
          <Button
            onClick={handleShuffle}
            label={translate('resources.album.actions.shuffle')}
          >
            <ShuffleIcon />
          </Button>
          <Button
            onClick={handlePlayNext}
            label={translate('resources.album.actions.playNext')}
          >
            <RiPlayList2Fill />
          </Button>
          <Button
            onClick={handlePlayLater}
            label={translate('resources.album.actions.addToQueue')}
          >
            <RiPlayListAddFill />
          </Button>
          {config.enableSharing && (
            <Button onClick={handleShare} label={translate('ra.action.share')}>
              <ShareIcon />
            </Button>
          )}
          {config.enableDownloads && (
            <Button
              onClick={handleDownload}
              label={
                translate('ra.action.download') +
                (isDesktop ? ` (${formatBytes(record.size)})` : '')
              }
            >
              <CloudDownloadOutlinedIcon />
            </Button>
          )}
          <Button
            onClick={handleExport}
            label={translate('resources.playlist.actions.export')}
          >
            <QueueMusicIcon />
          </Button>
          <Button
            onClick={handlePublishClick}
            label={translate('resources.playlist.actions.publish')}
            disabled={isPublishing}
          >
            <PublishIcon />
          </Button>
        </div>
        <div>{isNotSmall && <ToggleFieldsMenu resource="playlistTrack" />}</div>
      </div>
      <Dialog
        open={isPublishDialogOpen}
        onClose={handlePublishClose}
        aria-labelledby="publish-playlist-title"
        aria-describedby="publish-playlist-description"
      >
        <DialogTitle id="publish-playlist-title">
          {translate('resources.playlist.actions.publish')}
        </DialogTitle>
        <DialogContent>
          <DialogContentText id="publish-playlist-description">
            {translate('resources.playlist.message.publishConfirm')}
          </DialogContentText>
        </DialogContent>
        <DialogActions className={classes.publishDialogActions}>
          <MuiButton
            onClick={handlePublishClose}
            disabled={isPublishing}
            className={classes.publishCancelButton}
          >
            {translate('ra.message.no')}
          </MuiButton>
          <MuiButton
            onClick={handlePublishConfirm}
            disabled={isPublishing}
            color="primary"
            variant="contained"
            className={classes.publishConfirmButton}
          >
            {translate('ra.message.yes')}
          </MuiButton>
        </DialogActions>
      </Dialog>
    </TopToolbar>
  )
}

PlaylistActions.propTypes = {
  record: PropTypes.object.isRequired,
  selectedIds: PropTypes.arrayOf(PropTypes.number),
}

PlaylistActions.defaultProps = {
  record: {},
  selectedIds: [],
  onUnselectItems: () => null,
}

export default PlaylistActions
