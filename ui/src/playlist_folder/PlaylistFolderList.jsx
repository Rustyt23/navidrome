import { useMemo, useState, useCallback, useEffect } from 'react'
import {
  DateField,
  Filter,
  NumberField,
  ReferenceInput,
  SearchInput,
  SelectInput,
  TextField,
  useUpdate,
  useNotify,
  useRecordContext,
  usePermissions,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import Switch from '@material-ui/core/Switch'

import {
  DurationField,
  List,
  Writable,
  isWritable,
  useSelectedFields,
  useResourceRefresh,
} from '../common'
import PlaylistListActions from './PlaylistListActions'
import EmptyPlaylist from './EmptyPlaylist'
import TypeIconField from './TypeIconField'
import TypeAwareEditButton from './TypeAwareEditButton'
import PlaylistFolderBulkActions from './PlaylistFolderBulkActions'
import { PlaylistFolderDataGrid } from './PlaylistFolderDataGrid'

const PlaylistFolderFilter = (props) => {
  const { permissions } = usePermissions()
  return (
    <Filter {...props} variant="outlined">
      <SearchInput source="q" alwaysOn />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label="resources.playlist.fields.ownerName"
          reference="user"
          perPage={25}
          sort={{ field: 'name', order: 'ASC' }}
          alwaysOn
        >
          <SelectInput optionText="name" />
        </ReferenceInput>
      )}
    </Filter>
  )
}

const TogglePublicInput = ({ source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const [update, { isLoading }] = useUpdate()

  const serverValue = Boolean(record?.[source])

  const [checked, setChecked] = useState(serverValue)

  useEffect(() => {
    setChecked(serverValue)
  }, [serverValue])

  const handleChange = useCallback(
    (e) => {
      e.stopPropagation()
      if (!record?.id) return
      const resource = record.type
      const next = !checked
      setChecked(next)

      update(
        resource,
        record.id,
        { ...record, public: next },
        {
          onFailure: () => {
            setChecked(!next)
            notify('ra.page.error', 'warning')
          },
        }
      )
    },
    [checked, notify, record, update]
  )

  return (
    <Switch
      checked={checked}
      onChange={handleChange}
      onClick={(e) => e.stopPropagation()}
      disabled={isLoading || !isWritable(record?.ownerId)}
      color="primary"
      inputProps={{ 'aria-label': 'toggle-public' }}
    />
  )
}

const ToggleAutoImport = ({ source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const [update, { isLoading }] = useUpdate()

  if (record?.type !== 'playlist' || !record?.path) return null

  const handleChange = (e) => {
    e.stopPropagation()
    if (!record?.id) return
    const next = !record?.[source]

    update(
      record.type,
      record.id,
      { ...record, [source]: next },
      {
        onFailure: () => {
          notify('ra.page.error', 'warning')
        },
      },
    )
  }

  return (
    <Switch
      checked={Boolean(record?.[source])}
      onChange={handleChange}
      onClick={(e) => e.stopPropagation()}
      disabled={isLoading || !isWritable(record?.ownerId)}
      color="primary"
      inputProps={{ 'aria-label': 'toggle-auto-import' }}
    />
  )
}

const rowClick = (id, record) =>
  record?.type === 'folder' ? `/folder/${id}/show` : `/playlist/${id}/show`

const PlaylistFolderList = (props) => {
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('folder')

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      songCount: !isXsmall && <NumberField source="songCount" />,
      duration: !isXsmall && <DurationField source="duration" />,
      createdAt: isDesktop && <DateField source="createdAt" />,
      updatedAt: isDesktop && <DateField source="updatedAt" />,
      public: !isXsmall && <TogglePublicInput source="public" />,
      comment: isDesktop && <TextField source="comment" />,
      path: isDesktop && <TextField source="path" />,
      sync: !isXsmall && <ToggleAutoImport source="sync" />,
    }),
    [isDesktop, isXsmall],
  )

  const columns = useSelectedFields({
    resource: 'folder',
    columns: toggleableFields,
  })

  return (
    <List
      {...props}
      exporter={false}
      filters={<PlaylistFolderFilter />}
      actions={<PlaylistListActions />}
      bulkActionButtons={!isXsmall && <PlaylistFolderBulkActions />}
      empty={<EmptyPlaylist />}
      perPage={isXsmall ? 50 : 25}
    >
      <PlaylistFolderDataGrid rowClick={rowClick}>
        <TypeIconField label={false} />
        <TextField source="name" />
        {columns}
        <Writable>
          <TypeAwareEditButton />
        </Writable>
      </PlaylistFolderDataGrid>
    </List>
  )
}

export default PlaylistFolderList
