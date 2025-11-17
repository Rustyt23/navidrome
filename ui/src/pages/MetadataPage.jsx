import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Box,
  Container,
  LinearProgress,
  Typography,
} from '@material-ui/core'
import Alert from '@material-ui/lab/Alert'
import { makeStyles } from '@material-ui/core/styles'
import { FunctionField, ListContextProvider, TextField } from 'react-admin'
import { SongDatagrid } from '../common/SongDatagrid'
import { SongTitleField } from '../common/SongTitleField'
import { ArtistLinkField } from '../common/ArtistLinkField'
import { AlbumLinkField } from '../song/AlbumLinkField'
import subsonic from '../subsonic'
import { baseUrl } from '../utils'
import { getAllSongsMetadata } from '../services/metadata'

const useStyles = makeStyles((theme) => ({
  pageWrapper: {
    paddingTop: theme.spacing(4),
    paddingBottom: theme.spacing(6),
  },
  header: {
    marginBottom: theme.spacing(3),
  },
  tableWrapper: {
    width: '100%',
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
    boxShadow: theme.shadows[1],
    overflow: 'hidden',
  },
  datagridContainer: {
    width: '100%',
  },
  artwork: {
    width: 56,
    height: 56,
    borderRadius: theme.shape.borderRadius,
    objectFit: 'cover',
    boxShadow: theme.shadows[2],
  },
  artworkPlaceholder: {
    width: 56,
    height: 56,
    borderRadius: theme.shape.borderRadius,
    backgroundColor: theme.palette.action.hover,
  },
  alertWrapper: {
    padding: theme.spacing(2),
  },
}))

const MetadataPage = () => {
  const classes = useStyles()
  const [songs, setSongs] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    let isMounted = true

    const loadMetadata = async () => {
      setLoading(true)
      setError(null)
      try {
        const response = await getAllSongsMetadata()
        if (!isMounted) {
          return
        }
        setSongs(Array.isArray(response) ? response : [])
      } catch (err) {
        if (isMounted) {
          setError(err)
        }
      } finally {
        if (isMounted) {
          setLoading(false)
        }
      }
    }

    loadMetadata()

    return () => {
      isMounted = false
    }
  }, [])

  const { songIds, songsById } = useMemo(() => {
    const ids = []
    const map = {}

    songs.forEach((song) => {
      if (!song?.id) {
        return
      }
      ids.push(song.id)
      map[song.id] = song
    })

    return { songIds: ids, songsById: map }
  }, [songs])

  const noop = useCallback(() => {}, [])

  const listContextValue = useMemo(
    () => ({
      data: songsById,
      ids: songIds,
      total: songIds.length,
      resource: 'metadataSongs',
      basePath: '/metadata',
      currentSort: { field: 'title', order: 'ASC' },
      sort: { field: 'title', order: 'ASC' },
      filterValues: {},
      displayedFilters: {},
      selectedIds: [],
      onSelect: noop,
      onToggleItem: noop,
      onUnselectItems: noop,
      setFilters: noop,
      setPage: noop,
      setPerPage: noop,
      setSort: noop,
      showFilter: noop,
      hideFilter: noop,
      loading,
      loaded: !loading,
      page: 1,
      perPage: songIds.length || 25,
      version: songIds.length,
    }),
    [songsById, songIds, loading, noop],
  )

  const renderArtwork = (record) => {
    if (!record) {
      return null
    }

    if (!record.coverArt && !record.id) {
      return <div className={classes.artworkPlaceholder} />
    }

    const artId = record.coverArt || record.id
    const imageUrl = baseUrl(subsonic.url('getCoverArt', artId, { size: 120 }))

    return (
      <img
        src={imageUrl}
        alt={`${record.title || 'Song'} artwork`}
        className={classes.artwork}
      />
    )
  }

  return (
    <Container maxWidth="lg" className={classes.pageWrapper}>
      <div className={classes.header}>
        <Typography variant="h3" component="h1" gutterBottom>
          MetaData
        </Typography>
        <Typography variant="subtitle1" color="textSecondary">
          Manage and enhance song metadata
        </Typography>
      </div>
      <Box className={classes.tableWrapper}>
        {loading && <LinearProgress />}
        {error && (
          <div className={classes.alertWrapper}>
            <Alert severity="error">
              {error.message || 'Unable to load song metadata.'}
            </Alert>
          </div>
        )}
        <Box className={classes.datagridContainer}>
          <ListContextProvider value={listContextValue}>
            <SongDatagrid
              {...listContextValue}
              rowClick={false}
              hasBulkActions={false}
            >
              <FunctionField
                label="Artwork"
                render={renderArtwork}
                sortable={false}
              />
              <SongTitleField
                label="Title"
                source="title"
                showTrackNumbers={false}
                sortable={false}
              />
              <ArtistLinkField
                label="Artist"
                source="artist"
                sortable={false}
              />
              <AlbumLinkField
                label="Album"
                source="album"
                sortable={false}
              />
              <TextField
                label="Genre"
                source="genre"
                sortable={false}
              />
              <FunctionField
                label="Year"
                render={(record) => record?.year || ''}
                sortable={false}
              />
            </SongDatagrid>
          </ListContextProvider>
        </Box>
      </Box>
    </Container>
  )
}

export default MetadataPage
