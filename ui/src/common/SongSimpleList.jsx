import React from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemIcon from '@material-ui/core/ListItemIcon'
import ListItemSecondaryAction from '@material-ui/core/ListItemSecondaryAction'
import ListItemText from '@material-ui/core/ListItemText'
import { makeStyles } from '@material-ui/core/styles'
import { sanitizeListRestProps, useListContext } from 'react-admin'
import { DurationField, SongContextMenu, RatingField } from './index'
import { playTracks } from '../actions'
import { useDispatch } from 'react-redux'
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
  const classes = useStyles({ classes: classesOverride })
  const {
    ids: contextIds,
    data: contextData,
    currentSort,
    filterValues,
    page: currentPage,
    perPage: currentPerPage,
    total: contextTotal,
    resource: listResource,
  } = useListContext() || {}

  const handlePlayFromList = (id) => {
    const effectiveIds = contextIds || ids
    const effectiveData = contextData || data

    if (!effectiveIds || !effectiveIds.length || !effectiveData) {
      return
    }

    if (!effectiveIds.includes(id) || !effectiveData[id]) {
      return
    }

    const orderedIds = effectiveIds.filter(
      (songId) => effectiveData[songId] && !effectiveData[songId].missing,
    )

    if (!orderedIds.length) {
      return
    }

    const queueData = orderedIds.reduce((acc, songId) => {
      const record = effectiveData[songId]
      if (record) {
        acc[songId] = { ...record }
      }
      return acc
    }, {})

    let normalizedFilter
    if (filterValues) {
      try {
        normalizedFilter = JSON.parse(JSON.stringify(filterValues))
      } catch (error) {
        normalizedFilter = { ...filterValues }
      }
    }

    const page = currentPage || 1
    const perPage = currentPerPage || orderedIds.length
    const resolvedTotal =
      typeof contextTotal === 'number' ? contextTotal : total

    const queueMeta = {
      context: 'songsList',
      resource: listResource || 'song',
      sort: currentSort ? { ...currentSort } : undefined,
      filter: normalizedFilter,
      pagination: {
        page,
        perPage,
        loadedPages: [page],
        total: typeof resolvedTotal === 'number' ? resolvedTotal : null,
      },
      endReached:
        typeof resolvedTotal === 'number'
          ? page * perPage >= resolvedTotal
          : false,
    }

    dispatch(playTracks(queueData, orderedIds, id, queueMeta))
  }

  return (
    (loading || total > 0) && (
      <List className={className} {...sanitizeListRestProps(rest)}>
        {ids.map(
          (id) =>
            data[id] && (
              <span key={id} onClick={() => handlePlayFromList(id)}>
                <ListItem className={classes.listItem} button={true}>
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
