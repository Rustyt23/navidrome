import React from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemIcon from '@material-ui/core/ListItemIcon'
import ListItemSecondaryAction from '@material-ui/core/ListItemSecondaryAction'
import ListItemText from '@material-ui/core/ListItemText'
import { makeStyles } from '@material-ui/core/styles'
import { sanitizeListRestProps } from 'react-admin'
import { DurationField, SongContextMenu, RatingField } from './index'
import { setTrack } from '../actions'
import { useDispatch, useSelector } from 'react-redux'
import clsx from 'clsx'
import config from '../config'

const useStyles = makeStyles(
  (theme) => ({
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
    currentListItem: {
      backgroundColor: theme.palette.action.hover,
      '& $title, & $secondary, & $artist': {
        color: '#ff007f',
      },
      '& $timeStamp': {
        color: '#ff007f',
        opacity: 1,
      },
      '& svg': {
        fill: '#ff007f',
        color: '#ff007f',
      },
      '& .MuiRating-iconFilled, & .MuiRating-iconHover': {
        color: '#ff007f',
      },
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
  onItemClick,
  highlightCurrentTrack,
  ...rest
}) => {
  const dispatch = useDispatch()
  const currentTrack = useSelector((state) => state?.player?.current || {})
  const classes = useStyles({ classes: classesOverride })
  const highlight = Boolean(highlightCurrentTrack)
  return (
    (loading || total > 0) && (
      <List className={className} {...sanitizeListRestProps(rest)}>
        {ids.map(
          (id) =>
            data[id] && (
              <span
                key={id}
                onClick={() =>
                  onItemClick
                    ? onItemClick({
                        id,
                        record: data[id],
                        data,
                        ids,
                      })
                    : dispatch(setTrack(data[id]))
                }
              >
                <ListItem
                  className={clsx(
                    classes.listItem,
                    highlight &&
                      currentTrack?.trackId != null &&
                      (currentTrack.trackId?.toString() === id?.toString() ||
                        (data[id]?.id &&
                          currentTrack.trackId?.toString() ===
                            data[id]?.id?.toString()) ||
                        (data[id]?.mediaFileId &&
                          currentTrack.trackId?.toString() ===
                            data[id]?.mediaFileId?.toString())) &&
                      classes.currentListItem,
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
  onItemClick: PropTypes.func,
  highlightCurrentTrack: PropTypes.bool,
}

SongSimpleList.defaultProps = {
  hasBulkActions: false,
  selectedIds: [],
  highlightCurrentTrack: false,
}
