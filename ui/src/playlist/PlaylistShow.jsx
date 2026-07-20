import React, { useState, useCallback } from 'react'
import {
  Filter,
  Pagination,
  ReferenceManyField,
  SearchInput,
  ShowContextProvider,
  Title as RaTitle,
  useShowContext,
  useShowController,
  useTranslate,
} from 'react-admin'
import { Switch, Tooltip } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import PlaylistDetails from './PlaylistDetails'
import PlaylistSongs from './PlaylistSongs'
import PlaylistActions from './PlaylistActions'
import { Title, canChangeTracks, useResourceRefresh } from '../common'

const useStyles = makeStyles(
  (theme) => ({
    playlistActions: {
      width: '100%',
    },
    filterBar: {
      display: 'flex',
      alignItems: 'center',
      flexWrap: 'wrap',
    },
    filterToggle: {
      marginLeft: theme.spacing(1),
      marginBottom: theme.spacing(1),
    },
  }),
  {
    name: 'NDPlaylistShow',
  },
)

const PlaylistShowLayout = (props) => {
  const { loading, ...context } = useShowContext(props)
  const { record } = context
  const classes = useStyles()
  const translate = useTranslate()
  useResourceRefresh('song')

  const [searchTerm, setSearchTerm] = useState('')
  const [showDuplicatesOnly, setShowDuplicatesOnly] = useState(false)
  const [includeMissing, setIncludeMissing] = useState(true)

  const handleSearchChange = useCallback((eventOrValue) => {
    const value =
      typeof eventOrValue === 'string'
        ? eventOrValue
        : (eventOrValue?.target?.value ?? '')

    setSearchTerm(value)
  }, [])

  const handleToggleDuplicates = useCallback((event) => {
    setShowDuplicatesOnly(event.target.checked)
  }, [])

  const handleToggleMissing = useCallback((event) => {
    setIncludeMissing(event.target.checked)
  }, [])

  React.useEffect(() => {
    setShowDuplicatesOnly(false)
    setIncludeMissing(true)
  }, [record?.id])

  return (
    <>
      {record && <RaTitle title={<Title subTitle={record.name} />} />}
      {record && <PlaylistDetails {...context} />}
      {record && (
        <>
          <div className={classes.filterBar}>
            <Filter variant="outlined">
              <SearchInput
                id="search"
                source="q"
                alwaysOn
                value={searchTerm}
                onChange={handleSearchChange}
              />
            </Filter>
            <Tooltip title={translate('resources.playlist.actions.duplicates')}>
              <Switch
                className={classes.filterToggle}
                checked={showDuplicatesOnly}
                onChange={handleToggleDuplicates}
                color="secondary"
                inputProps={{
                  'aria-label': translate(
                    'resources.playlist.actions.duplicates',
                  ),
                }}
              />
            </Tooltip>
            <Tooltip
              title={translate('resources.playlist.actions.includeMissing')}
            >
              <Switch
                className={classes.filterToggle}
                checked={includeMissing}
                onChange={handleToggleMissing}
                color="secondary"
                inputProps={{
                  'aria-label': translate(
                    'resources.playlist.actions.includeMissing',
                  ),
                }}
              />
            </Tooltip>
          </div>

          <ReferenceManyField
            {...context}
            addLabel={false}
            reference="playlistTrack"
            target="playlist_id"
            sort={{ field: 'id', order: 'ASC' }}
            perPage={50}
            filter={{
              playlist_id: props.id,
              q: searchTerm,
              duplicatesOnly: showDuplicatesOnly,
              includeMissing,
            }}
          >
            <PlaylistSongs
              {...props}
              readOnly={!canChangeTracks(record)}
              title={<Title subTitle={record.name} />}
              actions={
                <PlaylistActions
                  className={classes.playlistActions}
                  record={record}
                />
              }
              resource={'playlistTrack'}
              exporter={false}
              pagination={
                <Pagination
                  rowsPerPageOptions={[50, 100, 200, 500]}
                  perPage={50}
                />
              }
              searchTerm={searchTerm}
              showDuplicatesOnly={showDuplicatesOnly}
              includeMissing={includeMissing}
            />
          </ReferenceManyField>
        </>
      )}
    </>
  )
}

const PlaylistShow = (props) => {
  const controllerProps = useShowController(props)
  return (
    <ShowContextProvider value={controllerProps}>
      <PlaylistShowLayout {...props} {...controllerProps} />
    </ShowContextProvider>
  )
}

export default PlaylistShow
