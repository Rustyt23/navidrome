import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Box,
  Button,
  Container,
  LinearProgress,
  Snackbar,
  Typography,
} from '@material-ui/core'
import CircularProgress from '@material-ui/core/CircularProgress'
import Alert from '@material-ui/lab/Alert'
import { makeStyles } from '@material-ui/core/styles'
import { FunctionField, ListContextProvider, TextField } from 'react-admin'
import { SongDatagrid } from '../common/SongDatagrid'
import { SongTitleField } from '../common/SongTitleField'
import { ArtistLinkField } from '../common/ArtistLinkField'
import { AlbumLinkField } from '../song/AlbumLinkField'
import subsonic from '../subsonic'
import { baseUrl } from '../utils'
import {
  fetchMissingMetadata,
  getAllSongsMetadata,
} from '../services/metadata'

const MERGEABLE_FIELDS = ['title', 'artist', 'album', 'genre', 'year', 'coverArt', 'artworkUrl']

const mergeUpdatedMetadata = (oldSongs = [], updatedSongs = []) => {
  if (!Array.isArray(updatedSongs) || updatedSongs.length === 0) {
    return oldSongs
  }

  const updates = new Map()
  updatedSongs.forEach((song) => {
    if (song?.id) {
      updates.set(song.id, song)
    }
  })

  if (updates.size === 0) {
    return oldSongs
  }

  let didUpdate = false

  const merged = oldSongs.map((song) => {
    if (!song?.id || !updates.has(song.id)) {
      return song
    }

    const update = updates.get(song.id)
    let changed = false
    const nextSong = { ...song }

    MERGEABLE_FIELDS.forEach((field) => {
      if (!Object.prototype.hasOwnProperty.call(update, field)) {
        return
      }
      const nextValue = update[field]
      if (nextSong[field] !== nextValue) {
        nextSong[field] = nextValue
        changed = true
      }
    })

    if (changed) {
      didUpdate = true
      return nextSong
    }

    return song
  })

  return didUpdate ? merged : oldSongs
}

const parseYearValue = (year) => {
  if (year === null || year === undefined) {
    return null
  }
  if (typeof year === 'number') {
    return Number.isNaN(year) ? null : year
  }
  if (typeof year === 'string') {
    const trimmed = year.trim()
    if (!trimmed) {
      return null
    }
    const parsed = Number.parseInt(trimmed, 10)
    return Number.isNaN(parsed) ? null : parsed
  }
  return null
}

const findMissingFields = (song) => {
  if (!song) {
    return false
  }

  const normalize = (value) => {
    if (typeof value === 'string') {
      return value.trim()
    }
    if (typeof value === 'number') {
      return Number.isNaN(value) ? '' : `${value}`.trim()
    }
    return value || ''
  }

  const album = normalize(song.album)
  const missingAlbum = !album || album.toLowerCase() === '[unknown album]'
  const missingArtist = !normalize(song.artist)
  const missingGenre = !normalize(song.genre)
  const missingYear = parseYearValue(song.year) === null
  const missingArtwork = !song.coverArt && !song.artworkUrl

  return missingAlbum || missingArtist || missingGenre || missingYear || missingArtwork
}

const useStyles = makeStyles((theme) => ({
  pageWrapper: {
    paddingTop: theme.spacing(4),
    paddingBottom: theme.spacing(6),
  },
  header: {
    marginBottom: theme.spacing(3),
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
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
  buttonWrapper: {
    whiteSpace: 'nowrap',
  },
}))

const MetadataPage = () => {
  const classes = useStyles()
  const [songs, setSongs] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState(null)
  const [fetchSuccessCount, setFetchSuccessCount] = useState(0)

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

  const songsMissingMetadata = useMemo(() => songs.filter(findMissingFields), [songs])
  const hasMissingSongs = songsMissingMetadata.length > 0

  const handleFetchMissingMetadata = useCallback(async () => {
    if (!hasMissingSongs || fetching) {
      return
    }

    setFetchError(null)
    setFetchSuccessCount(0)
    setFetching(true)
    try {
      const updates = await fetchMissingMetadata(
        songsMissingMetadata.filter((song) => song?.id),
      )
      if (!Array.isArray(updates) || updates.length === 0) {
        return
      }
      const persistedCount = updates.reduce(
        (count, song) => (song?.persisted ? count + 1 : count),
        0,
      )
      let updated = false
      setSongs((prevSongs) => {
        const merged = mergeUpdatedMetadata(prevSongs, updates)
        if (merged !== prevSongs) {
          updated = true
        }
        return merged
      })
      if (updated && persistedCount > 0) {
        setFetchSuccessCount(persistedCount)
      }
    } catch (err) {
      setFetchError(err)
    } finally {
      setFetching(false)
    }
  }, [fetching, hasMissingSongs, songsMissingMetadata])

  const handleCloseToast = useCallback((_, reason) => {
    if (reason === 'clickaway') {
      return
    }
    setFetchSuccessCount(0)
  }, [])

  const renderArtwork = (record) => {
    if (!record) {
      return null
    }

    const artworkUrl = record.artworkUrl
      || (record.coverArt
        ? baseUrl(subsonic.url('getCoverArt', record.coverArt || record.id, { size: 120 }))
        : '')

    if (!artworkUrl) {
      return <div className={classes.artworkPlaceholder} />
    }

    return (
      <img
        src={artworkUrl}
        alt={`${record.title || 'Song'} artwork`}
        className={classes.artwork}
      />
    )
  }

  return (
    <Container maxWidth="lg" className={classes.pageWrapper}>
      <div className={classes.header}>
        <div>
          <Typography variant="h3" component="h1" gutterBottom>
            MetaData
          </Typography>
          <Typography variant="subtitle1" color="textSecondary">
            Manage and enhance song metadata
          </Typography>
        </div>
        <div className={classes.buttonWrapper}>
          <Button
            color="primary"
            variant="contained"
            onClick={handleFetchMissingMetadata}
            disabled={!hasMissingSongs || fetching}
            startIcon={
              fetching ? <CircularProgress size={18} color="inherit" /> : null
            }
          >
            Fetch Missing Metadata
          </Button>
        </div>
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
        {fetchError && (
          <div className={classes.alertWrapper}>
            <Alert severity="warning">
              {fetchError.message || 'Unable to fetch missing metadata.'}
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
      <Snackbar
        open={fetchSuccessCount > 0}
        autoHideDuration={4000}
        onClose={handleCloseToast}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert elevation={6} variant="filled" severity="success" onClose={handleCloseToast}>
          {`Metadata permanently updated for ${fetchSuccessCount} track${fetchSuccessCount === 1 ? '' : 's'}`}
        </Alert>
      </Snackbar>
    </Container>
  )
}

export default MetadataPage
