import React, { useCallback, useEffect, useMemo } from 'react'
import {
  BulkActionsToolbar,
  ListToolbar,
  NumberField,
  TextField,
  useListContext,
  useVersion,
} from 'react-admin'
import clsx from 'clsx'
import { useDispatch } from 'react-redux'
import { Card, useMediaQuery } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import {
  DurationField,
  SongInfo,
  SongContextMenu,
  SongDatagrid,
  SongTitleField,
  QualityInfo,
  useSelectedFields,
  useResourceRefresh,
  DateField,
  ArtistLinkField,
  PathField,
  RatingField,
} from '../common'
import { AlbumLinkField } from '../song/AlbumLinkField'
import { playTracks } from '../actions'
import ExpandInfoDialog from '../dialogs/ExpandInfoDialog'
import config from '../config'
import DiscoverySongBulkActions from './DiscoverySongBulkActions'

const useStyles = makeStyles(
  (theme) => ({
    main: {
      display: 'flex',
    },
    content: {
      marginTop: 0,
      transition: theme.transitions.create('margin-top'),
      position: 'relative',
      flex: '1 1 auto',
      [theme.breakpoints.down('xs')]: {
        boxShadow: 'none',
      },
    },
    bulkActionsDisplayed: {
      marginTop: -theme.spacing(8),
      transition: theme.transitions.create('margin-top'),
    },
    actions: {
      zIndex: 2,
      display: 'flex',
      justifyContent: 'flex-end',
      flexWrap: 'wrap',
    },
    toolbar: {
      justifyContent: 'flex-start',
    },
    row: {
      '&:hover': {
        '& $contextMenu': {
          visibility: 'visible',
        },
        '& $ratingField': {
          visibility: 'visible',
        },
      },
    },
    contextMenu: {
      visibility: (props) => (props.isDesktop ? 'hidden' : 'visible'),
    },
    ratingField: {
      visibility: 'hidden',
    },
  }),
  { name: 'NDDiscoverySongs' },
)

const DiscoverySongs = ({ actions, pagination, filters, discoveryId }) => {
  const listContext = useListContext()
  const { data, ids, selectedIds, setPage, onUnselectItems } = listContext
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const classes = useStyles({ isDesktop })
  const dispatch = useDispatch()
  const version = useVersion()
  useResourceRefresh('song')

  useEffect(() => {
    setPage(1)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [discoveryId, setPage])

  const handleRowClick = useCallback(
    (id) => {
      if (!ids || ids.length === 0) {
        dispatch(playTracks(data, ids, id))
        return
      }

      const startIndex = ids.indexOf(id)
      if (startIndex === -1) {
        dispatch(playTracks(data, ids, id))
        return
      }

      const orderedIds = [
        ...ids.slice(startIndex),
        ...ids.slice(0, startIndex),
      ]

      dispatch(playTracks(data, orderedIds, id))
    },
    [dispatch, data, ids],
  )

  const toggleableFields = useMemo(() => {
    return {
      position: isDesktop && <TextField source="position" label={'#'} />,
      title: <SongTitleField source="title" showTrackNumbers={false} />,
      album: isDesktop && <AlbumLinkField source="album" />,
      artist: isDesktop && <ArtistLinkField source="artist" />,
      albumArtist: isDesktop && <ArtistLinkField source="albumArtist" />,
      duration: <DurationField source="duration" />, 
      year: isDesktop && (
        <NumberField source="year" sortByOrder={'DESC'} />
      ),
      playCount: isDesktop && (
        <NumberField source="playCount" sortByOrder={'DESC'} />
      ),
      playDate: isDesktop && (
        <DateField source="playDate" sortByOrder={'DESC'} showTime />
      ),
      createdAt: (
        <DateField source="createdAt" showTime sortable={false} />
      ),
      quality: isDesktop && <QualityInfo source="quality" sortable={false} />,
      channels: isDesktop && <NumberField source="channels" />,
      bpm: isDesktop && <NumberField source="bpm" />,
      genre: <TextField source="genre" />,
      comment: <TextField source="comment" />,
      path: <PathField source="path" />,
      rating: config.enableStarRating && (
        <RatingField
          source="rating"
          sortByOrder={'DESC'}
          resource={'song'}
          className={classes.ratingField}
        />
      ),
    }
  }, [isDesktop, classes.ratingField])

  const columns = useSelectedFields({
    resource: 'discoveryTrack',
    columns: toggleableFields,
    defaultOff: [
      'channels',
      'bpm',
      'year',
      'playCount',
      'comment',
      'playDate',
      'createdAt',
      'albumArtist',
      'rating',
    ],
  })

  const toolbarActions = useMemo(() => {
    if (!actions) {
      return null
    }
    return React.cloneElement(actions, {
      ids,
      data,
      selectedIds,
      onUnselectItems,
    })
  }, [actions, ids, data, selectedIds, onUnselectItems])

  return (
    <>
      <ListToolbar
        classes={{ toolbar: classes.toolbar }}
        filters={filters}
        actions={toolbarActions}
      />
      <div className={classes.main}>
        <Card
          className={clsx(classes.content, {
            [classes.bulkActionsDisplayed]: selectedIds.length > 0,
          })}
          key={version}
        >
          <BulkActionsToolbar>
            <DiscoverySongBulkActions
              discoveryId={discoveryId}
              onUnselectItems={onUnselectItems}
            />
          </BulkActionsToolbar>
          <SongDatagrid
            rowClick={handleRowClick}
            {...listContext}
            hasBulkActions={true}
            contextAlwaysVisible={!isDesktop}
            classes={{ row: classes.row }}
          >
            {columns}
            <SongContextMenu
              showLove={true}
              className={classes.contextMenu}
            />
          </SongDatagrid>
        </Card>
      </div>
      <ExpandInfoDialog content={<SongInfo />} />
      {pagination && React.cloneElement(pagination, listContext)}
    </>
  )
}

export default DiscoverySongs
