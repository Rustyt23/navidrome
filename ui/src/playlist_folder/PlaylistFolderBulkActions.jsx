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
import { LockOpen, Lock } from '@material-ui/icons'
import Button from '@material-ui/core/Button'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
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
    <ChangePublicStatusButton makePublic={true} {...props} />
    <ChangePublicStatusButton makePublic={false} {...props} />
    <CustomBulkDeleteButton {...props} />
  </Fragment>
)

export default PlaylistFolderBulkActions
