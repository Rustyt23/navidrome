import { Fragment, useCallback, useMemo, useState } from 'react'
import {
  useUnselectAll,
  useListContext,
  useDataProvider,
  useNotify,
  useRefresh,
  useTranslate,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import DeleteIcon from '@material-ui/icons/Delete'
import CompareArrowsIcon from '@material-ui/icons/CompareArrows'
import {
  LockOpen,
  Lock,
  Close as CloseIcon,
  DeleteSweep as DeleteSweepIcon,
} from '@material-ui/icons'
import Button from '@material-ui/core/Button'
import Dialog from '@material-ui/core/Dialog'
import DialogActions from '@material-ui/core/DialogActions'
import DialogContent from '@material-ui/core/DialogContent'
import DialogTitle from '@material-ui/core/DialogTitle'
import IconButton from '@material-ui/core/IconButton'
import Typography from '@material-ui/core/Typography'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemText from '@material-ui/core/ListItemText'
import Divider from '@material-ui/core/Divider'
import { buildDuplicateInfo } from './playlistComparison'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
  closeButton: {
    position: 'absolute',
    right: theme.spacing(1),
    top: theme.spacing(1),
  },
  duplicateList: {
    maxHeight: 360,
    overflow: 'auto',
    marginBottom: theme.spacing(1),
  },
  hint: {
    marginBottom: theme.spacing(1),
  },
}))

const getRecord = (data, id) =>
  Array.isArray(data) ? data.find((r) => r && r.id === id) : data?.[id]

async function safeUpdateMany(dataProvider, resource, ids, data) {
  try {
    const res = await dataProvider.updateMany(resource, { ids, data })
    return { data: Array.isArray(res?.data) ? res.data : [] }
  } catch (e) {
    const settled = await Promise.allSettled(
      ids.map((id) => dataProvider.update(resource, { id, data }))
    )
    const okIds = settled
      .map((r, i) => (r.status === 'fulfilled' ? ids[i] : null))
      .filter(Boolean)
    return { data: okIds }
  }
}

async function safeDeleteMany(dataProvider, resource, ids) {
  try {
    const res = await dataProvider.deleteMany(resource, { ids })
    return { data: Array.isArray(res?.data) ? res.data : [] }
  } catch (e) {
    const settled = await Promise.allSettled(
      ids.map((id) => dataProvider.delete(resource, { id }))
    )
    const okIds = settled
      .map((r, i) => (r.status === 'fulfilled' ? ids[i] : null))
      .filter(Boolean)
    return { data: okIds }
  }
}

const useBulkActionHandler = (listResource, actionKind, makePublic) => {
  const { selectedIds, data } = useListContext()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const unselectAll = useUnselectAll()
  const refresh = useRefresh()
  const [loading, setLoading] = useState(false)

  return useMemo(
    () =>
      async () => {
        if (!selectedIds?.length || loading) return
        setLoading(true)
        try {
          const playlistIds = []
          const folderIds = []

          selectedIds.forEach((id) => {
            const rec = getRecord(data, id)
            if (!rec) return
            if (rec.type === 'playlist') playlistIds.push(id)
            else if (rec.type === 'folder') folderIds.push(id)
          })

          const ops = []

          if (actionKind === 'togglePublic') {
            if (playlistIds.length)
              ops.push(safeUpdateMany(dataProvider, 'playlist', playlistIds, { public: makePublic }))
            if (folderIds.length)
              ops.push(
                safeUpdateMany(dataProvider, 'folder', folderIds, { public: makePublic })
              )
          }

          if (actionKind === 'delete') {
            if (playlistIds.length)
              ops.push(safeDeleteMany(dataProvider, 'playlist', playlistIds))
            if (folderIds.length)
              ops.push(
                safeDeleteMany(dataProvider, 'folder', folderIds)
              )
          }

          const settled = await Promise.allSettled(ops)
          const successCount = settled.reduce((sum, r) => {
            if (r.status === 'fulfilled') {
              return sum + (Array.isArray(r.value?.data) ? r.value.data.length : 0)
            }
            return sum
          }, 0)

          const rejected = settled.find((r) => r.status === 'rejected')
          if (rejected) {
            notify(rejected.reason?.message || 'ra.notification.http_error', {
              type: 'warning',
            })
          }

          if (successCount > 0) {
            notify(
              actionKind === 'delete' ? 'ra.notification.deleted' : 'ra.notification.updated',
              { type: 'info', messageArgs: { smart_count: successCount } }
            )
          }

          unselectAll(listResource)
          refresh({ hard: true })
        } finally {
          setLoading(false)
        }
      },
    [selectedIds, data, dataProvider, notify, unselectAll, refresh, listResource, actionKind, makePublic, loading]
  )
}

const CustomBulkDeleteButton = ({ resource }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const handleBulkDelete = useBulkActionHandler(resource, 'delete')

  return (
    <Button
      onClick={handleBulkDelete}
      startIcon={<DeleteIcon />}
      className={classes.button}
      aria-label={translate('ra.action.delete')}
    >
      {translate('ra.action.delete')}
    </Button>
  )
}

const ComparePlaylistsButton = ({ resource }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const { selectedIds = [], data } = useListContext()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const unselectAll = useUnselectAll()
  const refresh = useRefresh()

  const [open, setOpen] = useState(false)
  const [duplicates, setDuplicates] = useState([])
  const [playlists, setPlaylists] = useState([])
  const [loading, setLoading] = useState(false)

  const closeDialog = useCallback(() => {
    setOpen(false)
    setDuplicates([])
    setPlaylists([])
  }, [])

  const selectedPlaylists = useMemo(
    () =>
      selectedIds
        .map((id) => getRecord(data, id))
        .filter((record) => record?.type === 'playlist'),
    [selectedIds, data]
  )

  const handleCompare = useCallback(async () => {
    if (loading) return

    if (selectedIds.length !== 2 || selectedPlaylists.length !== 2) {
      notify('resources.playlist.message.compareSelectTwoPlaylists', { type: 'warning' })
      return
    }

    setLoading(true)
    try {
      const [left, right] = selectedPlaylists

      const [leftResult, rightResult] = await Promise.all([
        dataProvider.getList('playlistTrack', {
          filter: { playlist_id: left.id },
          pagination: { page: 1, perPage: 0 },
          sort: { field: 'id', order: 'ASC' },
        }),
        dataProvider.getList('playlistTrack', {
          filter: { playlist_id: right.id },
          pagination: { page: 1, perPage: 0 },
          sort: { field: 'id', order: 'ASC' },
        }),
      ])

      const matches = buildDuplicateInfo(leftResult?.data || [], rightResult?.data || [])
      setDuplicates(matches)
      setPlaylists([
        { id: left.id, name: left.name || left.id },
        { id: right.id, name: right.name || right.id },
      ])
      setOpen(true)
    } catch (e) {
      notify('ra.notification.http_error', { type: 'warning' })
    } finally {
      setLoading(false)
    }
  }, [loading, selectedIds, selectedPlaylists, dataProvider, notify])

  const handleRemoveDuplicates = useCallback(
    async (playlistId) => {
      if (!duplicates.length || !playlistId) return
      setLoading(true)
      try {
        const duplicateMediaIds = duplicates.map((duplicate) => duplicate.mediaFileId)
        const result = await safeDeleteMany(
          dataProvider,
          `playlist/${playlistId}/tracks`,
          duplicateMediaIds
        )

        const removedCount = Array.isArray(result?.data) ? result.data.length : 0
        notify('ra.notification.deleted', {
          type: removedCount > 0 ? 'info' : 'warning',
          messageArgs: { smart_count: removedCount },
        })
        closeDialog()
        unselectAll(resource)
        refresh({ hard: true })
      } catch (e) {
        notify('ra.notification.http_error', { type: 'warning' })
      } finally {
        setLoading(false)
      }
    },
    [duplicates, dataProvider, notify, closeDialog, unselectAll, resource, refresh]
  )

  return (
    <>
      <Button
        onClick={handleCompare}
        startIcon={<CompareArrowsIcon />}
        className={classes.button}
        aria-label={translate('resources.playlist.actions.compare')}
        disabled={loading}
      >
        {translate('resources.playlist.actions.compare')}
      </Button>

      <Dialog open={open} onClose={closeDialog} fullWidth maxWidth="sm">
        <DialogTitle disableTypography>
          <Typography variant="h6">{translate('resources.playlist.actions.compare')}</Typography>
          <IconButton
            aria-label={translate('ra.action.close')}
            className={classes.closeButton}
            onClick={closeDialog}
          >
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent>
          {duplicates.length > 0 ? (
            <>
              <Typography variant="body2" className={classes.hint}>
                {translate('resources.playlist.message.compareDuplicatesFound', {
                  smart_count: duplicates.length,
                })}
              </Typography>
              <List className={classes.duplicateList} dense>
                {duplicates.map((duplicate) => (
                  <ListItem key={duplicate.mediaFileId}>
                    <ListItemText
                      primary={duplicate.title || duplicate.mediaFileId}
                      secondary={duplicate.artist || undefined}
                    />
                  </ListItem>
                ))}
              </List>
              <Divider />
              <Typography variant="body2" className={classes.hint}>
                {translate('resources.playlist.message.compareRemovePrompt')}
              </Typography>
            </>
          ) : (
            <Typography variant="body2">
              {translate('resources.playlist.message.compareNoDuplicates')}
            </Typography>
          )}
        </DialogContent>
        <DialogActions>
          {duplicates.length > 0 &&
            playlists.map((playlist) => (
              <Button
                key={playlist.id}
                startIcon={<DeleteSweepIcon />}
                onClick={() => handleRemoveDuplicates(playlist.id)}
                color="secondary"
                disabled={loading}
              >
                {translate('resources.playlist.actions.removeDuplicatesFrom', {
                  name: playlist.name,
                })}
              </Button>
            ))}
          <Button onClick={closeDialog}>{translate('ra.action.cancel')}</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}

const ChangePublicStatusButton = ({ resource, makePublic }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const handleChangeStatus = useBulkActionHandler(resource, 'togglePublic', makePublic)
  const label = makePublic
    ? translate('resources.playlist.actions.makePublic')
    : translate('resources.playlist.actions.makePrivate')
  const icon = makePublic ? <LockOpen /> : <Lock />

  return (
    <Button
      onClick={handleChangeStatus}
      startIcon={icon}
      className={classes.button}
      aria-label={label}
    >
      {label}
    </Button>
  )
}

const PlaylistFolderBulkActions = (props) => (
  <Fragment>
    <ComparePlaylistsButton {...props} />
    <ChangePublicStatusButton makePublic={true} {...props} />
    <ChangePublicStatusButton makePublic={false} {...props} />
    <CustomBulkDeleteButton {...props} />
  </Fragment>
)

export default PlaylistFolderBulkActions
