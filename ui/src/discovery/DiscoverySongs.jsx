import React, { useEffect, useMemo } from 'react'
import {
  Datagrid,
  ListContextProvider,
  ListToolbar,
  TextField,
  useListContext,
  useVersion,
} from 'react-admin'
import { Card, useMediaQuery } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import clsx from 'clsx'
import {
  DurationField,
  PathField,
  SizeField,
  useSelectedFields,
  useResourceRefresh,
} from '../common'

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
  }),
  { name: 'NDDiscoverySongs' },
)

const DiscoverySongs = ({ actions, pagination, discoveryId, searchTerm }) => {
  const listContext = useListContext()
  const version = useVersion()
  const { setPage, selectedIds } = listContext
  const classes = useStyles()
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('discoveryTrack', 'discovery')

  useEffect(() => {
    setPage(1)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [discoveryId, searchTerm, setPage])

  const toggleableFields = useMemo(
    () => ({
      title: <TextField source="title" label="resources.song.fields.title" />,
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
          <Datagrid {...listContext} rowClick={false} bulkActionButtons={false}>
            {columns}
          </Datagrid>
        </Card>
      </div>
      {pagination && React.cloneElement(pagination, listContext)}
    </ListContextProvider>
  )
}

export default DiscoverySongs
