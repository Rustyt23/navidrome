import React from 'react'
import {
  Button,
  sanitizeListRestProps,
  TopToolbar,
  useTranslate,
  useDataProvider,
  useNotify,
} from 'react-admin'
import { useDispatch } from 'react-redux'
import { useMediaQuery, makeStyles } from '@material-ui/core'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import ShuffleIcon from '@material-ui/icons/Shuffle'
import CloudDownloadOutlinedIcon from '@material-ui/icons/CloudDownloadOutlined'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import { RiPlayListAddFill, RiPlayList2Fill } from 'react-icons/ri'
import { ToggleFieldsMenu } from '../common'
import {
  addTracks,
  playNext,
  playTracks,
  shuffleTracks,
  openDownloadMenu,
  DOWNLOAD_MENU_PLAY,
} from '../actions'
import { httpClient } from '../dataProvider'
import { REST_URL } from '../consts'
import config from '../config'
import { formatBytes } from '../utils'
import PublishDiscoveryButton from './PublishDiscoveryButton'

const useStyles = makeStyles({
  toolbar: { display: 'flex', justifyContent: 'space-between', width: '100%' },
  columnPicker: {
    marginLeft: 'auto',
    display: 'flex',
    alignItems: 'center',
  },
})

const DiscoveryActions = ({ className, ids, data, record, ...rest }) => {
  const dispatch = useDispatch()
  const translate = useTranslate()
  const classes = useStyles()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))

  const getAllTracks = React.useCallback(
    (action) => {
      if (!record) {
        return
      }
      if (ids?.length && record.songCount && ids.length >= record.songCount) {
        dispatch(action(data, ids))
        return
      }
      dataProvider
        .getList('discoveryTrack', {
          pagination: { page: 1, perPage: 0 },
          sort: { field: 'id', order: 'ASC' },
          filter: { discovery_id: record.id },
        })
        .then((response) => {
          const trackMap = response.data.reduce(
            (acc, track) => ({ ...acc, [track.id]: track }),
            {},
          )
          dispatch(action(trackMap))
        })
        .catch(() => {
          notify('ra.page.error', 'warning')
        })
    },
    [record, ids, data, dataProvider, dispatch, notify],
  )

  const handlePlay = React.useCallback(() => {
    getAllTracks(playTracks)
  }, [getAllTracks])

  const handlePlayNext = React.useCallback(() => {
    getAllTracks(playNext)
  }, [getAllTracks])

  const handlePlayLater = React.useCallback(() => {
    getAllTracks(addTracks)
  }, [getAllTracks])

  const handleShuffle = React.useCallback(() => {
    getAllTracks(shuffleTracks)
  }, [getAllTracks])

  const handleDownload = React.useCallback(() => {
    if (!record) {
      return
    }
    dispatch(openDownloadMenu(record, DOWNLOAD_MENU_PLAY))
  }, [dispatch, record])

  const handleExport = React.useCallback(() => {
    if (!record) {
      return
    }
    httpClient(`${REST_URL}/discovery/${record.id}/tracks`, {
      headers: new Headers({ Accept: 'audio/x-mpegurl' }),
    })
      .then((res) => {
        const blob = new Blob([res.body], { type: 'audio/x-mpegurl' })
        const url = window.URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `${record.name}.m3u`
        document.body.appendChild(link)
        link.click()
        link.parentNode.removeChild(link)
      })
      .catch(() => {
        notify('ra.page.error', 'warning')
      })
  }, [notify, record])

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
          {config.enableDownloads && (
            <Button
              onClick={handleDownload}
              label={
                translate('ra.action.download') +
                (isNotSmall ? ` (${formatBytes(record?.size)})` : '')
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
          <PublishDiscoveryButton record={record} />
        </div>
        <div className={classes.columnPicker}>
          {isNotSmall && <ToggleFieldsMenu resource="discoveryTrack" />}
        </div>
      </div>
    </TopToolbar>
  )
}

export default DiscoveryActions
