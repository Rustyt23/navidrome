import React, { useCallback, useMemo } from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemIcon from '@material-ui/core/ListItemIcon'
import ListItemSecondaryAction from '@material-ui/core/ListItemSecondaryAction'
import ListItemText from '@material-ui/core/ListItemText'
import { makeStyles } from '@material-ui/core/styles'
import { sanitizeListRestProps } from 'react-admin'
import { DurationField, SongContextMenu, RatingField } from './index'
import clsx from 'clsx'
import { useDispatch, useSelector } from 'react-redux'
import { playTracks, setTrack } from '../actions'
import config from '../config'

const useStyles = makeStyles(
  {
    link: {
      textDecoration: 'none',
      color: 'inherit',
    },
    listItem: {
      padding: '10px',
    },
    title: {
      paddingRight: '10px',
      width: '80%',
    },
    secondary: {
      marginTop: '-3px',
      width: '96%',
      display: 'flex',
      alignItems: 'flex-start',
      justifyContent: 'space-between',
    },
    artist: {
      paddingRight: '30px',
    },
    timeStamp: {
      float: 'right',
      color: '#fff',
      fontWeight: '200',
      opacity: 0.6,
      fontSize: '12px',
      padding: '2px',
    },
    rightIcon: {
      top: '26px',
    },
  },
  { name: 'RaSongSimpleList' },
)

export const SongSimpleList = ({
  basePath,
  className,
  classes: classesOverride,
  data,
  hasBulkActions,
  ids,
  loading,
  onToggleItem,
  selectedIds,
  total,
  ...rest
}) => {
  const dispatch = useDispatch()
  const currentTrackId = useSelector(
    (state) => state?.player?.current?.trackId,
  )
  const isMobile = useMemo(() => {
    if (typeof window === 'undefined' || !window.matchMedia) {
      return false
    }
    return window.matchMedia('(max-width: 768px)').matches
  }, [])
  const classes = useStyles({ classes: classesOverride })
  const getTrackId = useCallback(
    (song) => song?.mediaFileId || song?.id,
    [],
  )

  const handlePlay = useCallback(
    (songId) => () => {
      const record = data?.[songId]
      if (!record) {
        return
      }

      if (isMobile && Array.isArray(ids) && ids.length > 0) {
        const visibleSongs = ids
          .map((id) => data?.[id])
          .filter((song) => Boolean(song) && !song?.missing)

        if (visibleSongs.length > 0) {
          const startIndex = visibleSongs.findIndex(
            (song) => getTrackId(song) === getTrackId(record),
          )
          if (startIndex === -1) {
            dispatch(setTrack(record))
            return
          }
          const queue = visibleSongs.reduce((acc, song, idx) => {
            acc[idx] = song
            return acc
          }, {})
          dispatch(playTracks(queue, undefined, String(startIndex)))
          return
        }
      }

      dispatch(setTrack(record))
    },
    [data, dispatch, getTrackId, ids, isMobile],
  )

  const isCurrentSong = useCallback(
    (song) => {
      const trackId = getTrackId(song)
      return Boolean(trackId) && trackId === currentTrackId
    },
    [currentTrackId, getTrackId],
  )
  return (
    (loading || total > 0) && (
      <List className={className} {...sanitizeListRestProps(rest)}>
        {ids.map(
          (id) =>
            data[id] && (
              <span key={id} onClick={handlePlay(id)}>
                <ListItem
                  className={clsx(
                    classes.listItem,
                    isMobile && isCurrentSong(data[id]) && 'row--playing-mobile',
                  )}
                  button={true}
                >
                  <ListItemText
                    primary={
                      <div className={classes.title}>{data[id].title}</div>
                    }
                    secondary={
                      <>
                        <span className={classes.secondary}>
                          <span className={classes.artist}>
                            {data[id].artist}
                          </span>
                          <span className={classes.timeStamp}>
                            <DurationField
                              record={data[id]}
                              source={'duration'}
                            />
                          </span>
                        </span>
                        {config.enableStarRating && (
                          <RatingField
                            record={data[id]}
                            source={'rating'}
                            resource={'song'}
                            size={'small'}
                          />
                        )}
                      </>
                    }
                  />
                  <ListItemSecondaryAction className={classes.rightIcon}>
                    <ListItemIcon>
                      <SongContextMenu record={data[id]} visible={true} />
                    </ListItemIcon>
                  </ListItemSecondaryAction>
                </ListItem>
              </span>
            ),
        )}
      </List>
    )
  )
}

SongSimpleList.propTypes = {
  basePath: PropTypes.string,
  className: PropTypes.string,
  classes: PropTypes.object,
  data: PropTypes.object,
  hasBulkActions: PropTypes.bool.isRequired,
  ids: PropTypes.array,
  onToggleItem: PropTypes.func,
  selectedIds: PropTypes.arrayOf(PropTypes.any).isRequired,
}

SongSimpleList.defaultProps = {
  hasBulkActions: false,
  selectedIds: [],
}
