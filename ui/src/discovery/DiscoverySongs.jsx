import React, { useCallback, useEffect, useMemo } from 'react'
import {
  BulkActionsToolbar,
  ListContextProvider,
  ListToolbar,
  TextField,
  useListContext,
  useVersion,
} from 'react-admin'
import { useDispatch } from 'react-redux'
import { Card, useMediaQuery } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import clsx from 'clsx'
import {
  DurationField,
  PathField,
  SizeField,
  SongDatagrid,
  SongTitleField,
  SongContextMenu,
  SongInfo,
  useSelectedFields,
  useResourceRefresh,
} from '../common'
import ExpandInfoDialog from '../dialogs/ExpandInfoDialog'
import { playTracks } from '../actions'
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
    toolbar: {
      justifyContent: 'flex-start',
    },
    bulkActionsDisplayed: {
      marginTop: -theme.spacing(8),
      transition: theme.transitions.create('margin-top'),
    },
    row: {
      '&:hover': {
        '& $contextMenu': {
          visibility: 'visible',
        },
      },
    },
    contextMenu: (props) => ({
      visibility: props?.isDesktop ? 'hidden' : 'visible',
    }),
  }),
  { name: 'NDDiscoverySongs' },
)

const DiscoverySongs = ({ actions, pagination, discoveryId, searchTerm }) => {
  const listContext = useListContext()
  const version = useVersion()
  const dispatch = useDispatch()
  const { setPage, selectedIds, data, ids, onUnselectItems } = listContext
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const classes = useStyles({ isDesktop })
  useResourceRefresh('discoveryTrack', 'discovery')

  useEffect(() => {
    setPage(1)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [discoveryId, searchTerm, setPage])

  const toggleableFields = useMemo(
    () => ({
      title: <SongTitleField source="title" showTrackNumbers={false} />,
      artist: isDesktop && (
        <TextField source="artist" label="resources.song.fields.artist" />
      ),
      album: isDesktop && (
        <TextField source="album" label="resources.song.fields.album" />
      ),
      duration: <DurationField source="duration" />, 
      size: isDesktop && <SizeField source="size" />, 
      path: <PathField source="path" label="resources.song.fields.path" />,
    }),
    [isDesktop],
  )

  const columns = useSelectedFields({
    resource: 'discoveryTrack',
    columns: toggleableFields,
    defaultOff: ['path'],
  })

  const handleRowClick = useCallback(() => {
    if (Array.isArray(ids) && ids.length > 0) {
      dispatch(playTracks(data, ids))
      return
    }

    dispatch(playTracks(data))
  }, [dispatch, data, ids])

  return (
    <ListContextProvider value={listContext}>
      <ListToolbar classes={{ toolbar: classes.toolbar }} actions={actions} />
      <div className={classes.main}>
        <Card
          className={clsx(classes.content, {
            [classes.bulkActionsDisplayed]: selectedIds?.length > 0,
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
            {...listContext}
            rowClick={handleRowClick}
            hasBulkActions
            contextAlwaysVisible={!isDesktop}
            classes={{ row: classes.row }}
          >
            {columns}
            <SongContextMenu
              onAddToPlaylist={() => null}
              showLove={false}
              className={classes.contextMenu}
            />
          </SongDatagrid>
        </Card>
      </div>
      <ExpandInfoDialog content={<SongInfo />} />
      {pagination && React.cloneElement(pagination, listContext)}
    </ListContextProvider>
  )
}

export default DiscoverySongs
