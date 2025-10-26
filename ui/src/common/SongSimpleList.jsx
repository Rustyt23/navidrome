import React, { useCallback, useMemo } from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemIcon from '@material-ui/core/ListItemIcon'
import ListItemSecondaryAction from '@material-ui/core/ListItemSecondaryAction'
import ListItemText from '@material-ui/core/ListItemText'
import { makeStyles, useTheme } from '@material-ui/core/styles'
import { sanitizeListRestProps } from 'react-admin'
import clsx from 'clsx'
import { DurationField, SongContextMenu, RatingField } from './index'
import { useDispatch, useSelector } from 'react-redux'
import { playTracks, setTrack } from '../actions'
import config from '../config'
import PlayingLight from '../icons/playing-light.gif'
import PlayingDark from '../icons/playing-dark.gif'
import PausedLight from '../icons/paused-light.png'
import PausedDark from '../icons/paused-dark.png'

const useStyles = makeStyles(
  (theme) => ({
    link: {
      textDecoration: 'none',
      color: 'inherit',
    },
    listItem: {
      paddingTop: 6,
      paddingBottom: 6,
      paddingLeft: theme.spacing(2),
      paddingRight: theme.spacing(7),
    },
    currentRowMobile: {
      backgroundColor: theme.palette.action.hover,
      '& $title, & $secondary, & $artist, & $timeStamp': {
        color: 'var(--accent)',
      },
      '& svg': {
        fill: 'var(--accent)',
        color: 'var(--accent)',
      },
      '& $rightIcon': {
        visibility: 'visible',
      },
    },
    mobilePrimaryContainer: {
      display: 'flex',
      alignItems: 'center',
      width: '100%',
    },
    title: {
      paddingRight: '10px',
      width: '80%',
    },
    mobilePrimaryRow: {
      display: 'grid',
      gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
      alignItems: 'center',
      width: '100%',
      boxSizing: 'border-box',
      flex: 1,
    },
    mobileTitle: {
      minWidth: 0,
      overflow: 'hidden',
      paddingRight: theme.spacing(1),
      textAlign: 'left',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    mobileArtist: {
      minWidth: 0,
      overflow: 'hidden',
      textAlign: 'left',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
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
    playingIndicator: {
      width: 24,
      height: 24,
      marginRight: theme.spacing(1),
      flexShrink: 0,
    },
    playingIndicatorActive: {
      filter:
        'invert(72%) sepia(34%) saturate(6113%) hue-rotate(305deg) brightness(103%) contrast(102%)',
    },
  }),
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
  contextMenuProps = {},
  ...rest
}) => {
  const dispatch = useDispatch()
  const currentTrack = useSelector((state) => state?.player?.current || {})
  const currentTrackId = currentTrack?.trackId
  const isPaused = Boolean(currentTrack?.paused)
  const theme = useTheme()
  const themeType = theme.palette.type
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

  const contextMenuPropsForRender = isMobile
    ? { ...contextMenuProps, showLove: false }
    : contextMenuProps
  const playingIcon = useMemo(() => {
    if (!currentTrackId) {
      return null
    }
    if (isPaused) {
      return themeType === 'light' ? PausedLight : PausedDark
    }
    return themeType === 'light' ? PlayingLight : PlayingDark
  }, [currentTrackId, isPaused, themeType])
  return (
    (loading || total > 0) && (
      <List className={className} {...sanitizeListRestProps(rest)}>
        {ids.map((id) => {
          const record = data[id]
          if (!record) {
            return null
          }
          const isCurrent = isCurrentSong(record)
          return (
            <span key={id} onClick={handlePlay(id)}>
              <ListItem
                className={clsx(
                  classes.listItem,
                  isMobile && isCurrent && classes.currentRowMobile,
                )}
                selected={isMobile && isCurrent}
                button={true}
              >
                <ListItemText
                  primary={
                    isMobile ? (
                      <div className={classes.mobilePrimaryContainer}>
                        {isCurrent && playingIcon && (
                          <img
                            src={playingIcon}
                            alt={isPaused ? 'paused' : 'playing'}
                            className={clsx(
                              classes.playingIndicator,
                              !isPaused && classes.playingIndicatorActive,
                            )}
                          />
                        )}
                        <div className={classes.mobilePrimaryRow}>
                          <span className={classes.mobileTitle}>
                            {record.title}
                          </span>
                          <span className={classes.mobileArtist}>
                            {record.artist}
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className={classes.title}>{record.title}</div>
                    )
                  }
                  secondary={
                    isMobile ? (
                      null
                    ) : (
                      <>
                        <span className={classes.secondary}>
                          <span className={classes.artist}>{record.artist}</span>
                          <span className={classes.timeStamp}>
                            <DurationField
                              record={record}
                              source={'duration'}
                            />
                          </span>
                        </span>
                        {config.enableStarRating && (
                          <RatingField
                            record={record}
                            source={'rating'}
                            resource={'song'}
                            size={'small'}
                          />
                        )}
                      </>
                    )
                  }
                />
                <ListItemSecondaryAction className={classes.rightIcon}>
                  <ListItemIcon>
                    <SongContextMenu
                      record={record}
                      visible={true}
                      {...contextMenuPropsForRender}
                    />
                  </ListItemIcon>
                </ListItemSecondaryAction>
              </ListItem>
            </span>
          )
        })}
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
  contextMenuProps: PropTypes.object,
}

SongSimpleList.defaultProps = {
  hasBulkActions: false,
  selectedIds: [],
  contextMenuProps: {},
}
