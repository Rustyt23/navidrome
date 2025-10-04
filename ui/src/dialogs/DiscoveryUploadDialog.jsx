import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  makeStyles,
  TextField,
  Typography,
} from '@material-ui/core'
import SearchIcon from '@material-ui/icons/Search'
import { useDataProvider, useNotify, useRefresh, useTranslate } from 'react-admin'
import { closeDiscoveryUpload } from '../actions'

const useStyles = makeStyles((theme) => ({
  content: {
    minWidth: 400,
    [theme.breakpoints.down('xs')]: {
      minWidth: 'auto',
    },
  },
  list: {
    maxHeight: '50vh',
    overflowY: 'auto',
    marginTop: theme.spacing(2),
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: theme.shape.borderRadius,
  },
  listItem: {
    paddingTop: 0,
    paddingBottom: 0,
  },
  searchField: {
    width: '100%',
  },
  loading: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(2),
  },
  empty: {
    padding: theme.spacing(2),
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
}))

const useDebouncedValue = (value, delay) => {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const handler = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(handler)
  }, [value, delay])

  return debounced
}

const DiscoveryUploadDialog = () => {
  const classes = useStyles()
  const dispatch = useDispatch()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const dataProvider = useDataProvider()
  const { open, discoveryId, discoveryName } = useSelector(
    (state) => state.discoveryUploadDialog,
  )

  const [search, setSearch] = useState('')
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(false)
  const [selectedIds, setSelectedIds] = useState([])

  const debouncedSearch = useDebouncedValue(search, 300)

  useEffect(() => {
    if (!open) {
      return
    }
    let isMounted = true
    setLoading(true)
    dataProvider
      .getList('song', {
        pagination: { page: 1, perPage: 25 },
        sort: { field: 'title', order: 'ASC' },
        filter: debouncedSearch ? { q: debouncedSearch } : {},
      })
      .then(({ data }) => {
        if (!isMounted) {
          return
        }
        setOptions(data)
      })
      .catch(() => {
        if (isMounted) {
          notify('ra.page.error', 'warning')
        }
      })
      .finally(() => {
        if (isMounted) {
          setLoading(false)
        }
      })

    return () => {
      isMounted = false
    }
  }, [dataProvider, debouncedSearch, notify, open])

  useEffect(() => {
    if (!open) {
      setSearch('')
      setSelectedIds([])
      setOptions([])
    }
  }, [open])

  const toggleSelection = useCallback(
    (id) => {
      setSelectedIds((prev) =>
        prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
      )
    },
    [],
  )

  const handleClose = useCallback(() => {
    dispatch(closeDiscoveryUpload())
  }, [dispatch])

  const handleSubmit = useCallback(() => {
    if (!discoveryId || selectedIds.length === 0) {
      return
    }
    dataProvider
      .addToDiscovery(discoveryId, { ids: selectedIds })
      .then(() => {
        notify('resources.discovery.notifications.uploaded', {
          type: 'info',
          messageArgs: {
            smart_count: selectedIds.length,
            name: discoveryName,
          },
        })
        refresh()
        handleClose()
      })
      .catch(() => {
        notify('ra.page.error', 'warning')
      })
  }, [dataProvider, discoveryId, discoveryName, handleClose, notify, refresh, selectedIds])

  const songOptions = useMemo(
    () =>
      options.map((song) => ({
        id: song.id,
        primary: song.title || song.name || song.path,
        secondary: [song.artist, song.album].filter(Boolean).join(' • '),
      })),
    [options],
  )

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {translate('resources.discovery.actions.uploadSongs')}
      </DialogTitle>
      <DialogContent className={classes.content}>
        <TextField
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          variant="outlined"
          className={classes.searchField}
          placeholder={translate('resources.discovery.actions.searchSongs')}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <IconButton size="small" tabIndex={-1} disabled>
                  <SearchIcon fontSize="small" />
                </IconButton>
              </InputAdornment>
            ),
          }}
        />
        <Box className={classes.list}>
          {loading ? (
            <div className={classes.loading}>
              <CircularProgress size={24} />
            </div>
          ) : songOptions.length === 0 ? (
            <Typography className={classes.empty} variant="body2">
              {translate('resources.discovery.message.noSongsFound')}
            </Typography>
          ) : (
            <List dense>
              {songOptions.map((song) => (
                <ListItem
                  key={song.id}
                  button
                  onClick={() => toggleSelection(song.id)}
                  className={classes.listItem}
                >
                  <ListItemIcon>
                    <Checkbox checked={selectedIds.includes(song.id)} />
                  </ListItemIcon>
                  <ListItemText
                    primary={song.primary}
                    secondary={song.secondary}
                  />
                </ListItem>
              ))}
            </List>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>
          {translate('ra.action.cancel')}
        </Button>
        <Button
          onClick={handleSubmit}
          disabled={selectedIds.length === 0}
          color="primary"
          variant="contained"
        >
          {translate('resources.discovery.actions.uploadSelected')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}

export default DiscoveryUploadDialog
