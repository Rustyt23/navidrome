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
import { RiPlayListAddFill, RiPlayList2Fill } from 'react-icons/ri'
import { ToggleFieldsMenu } from '../common'
import { addTracks, playNext, playTracks, shuffleTracks } from '../actions'

const useStyles = makeStyles({
  toolbar: { display: 'flex', justifyContent: 'space-between', width: '100%' },
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
        </div>
        <div>{isNotSmall && <ToggleFieldsMenu resource="discoveryTrack" />}</div>
      </div>
    </TopToolbar>
  )
}

export default DiscoveryActions
