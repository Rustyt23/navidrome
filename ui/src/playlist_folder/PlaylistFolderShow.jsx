import { useMemo, useState, useCallback, useEffect } from 'react'
import {
  ShowBase,
  List,
  DateField,
  Filter,
  ReferenceInput,
  SearchInput,
  SelectInput,
  TextField,
  useUpdate,
  useNotify,
  useRecordContext,
  usePermissions,
  useResourceContext,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import Switch from '@material-ui/core/Switch'

import {
  Writable,
  isWritable,
  useSelectedFields,
  useResourceRefresh,
  Title,
} from '../common'
import PlaylistListActions from './PlaylistListActions'
import EmptyPlaylist from './EmptyPlaylist'
import TypeIconField from './TypeIconField'
import TypeAwareEditButton from './TypeAwareEditButton'
import PlaylistFolderBulkActions from './PlaylistFolderBulkActions'
import { PlaylistFolderDataGrid } from './PlaylistFolderDataGrid'

const PlaylistFolderFilter = (props) => {
  const { permissions } = usePermissions()
  const resource = useResourceContext() || 'folder'
  return (
    <Filter {...props} variant="outlined">
      <SearchInput source="q" alwaysOn resettable />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label={`resources.${resource}.fields.ownerName`}
          reference="user"
          perPage={50}
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
  const parentResource = useResourceContext() || 'folder'

  const serverValue = Boolean(record?.[source])

  const [checked, setChecked] = useState(serverValue)

  useEffect(() => {
    setChecked(serverValue)
  }, [serverValue])

  const handleChange = useCallback(
    (e) => {
      e.stopPropagation()
      if (!record?.id) return
      const resource =
        record.type === 'folder'
          ? parentResource === 'discoveryFolder'
            ? 'discoveryFolder'
            : 'folder'
          : record.type
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
    [checked, notify, parentResource, record, update]
  )

  return (
    <Switch
      checked={checked}
      onChange={handleChange}
      onClick={(e) => e.stopPropagation()}
      disabled={isLoading || !isWritable(record?.ownerId)}
      color="primary"
      size="small"
      inputProps={{ 'aria-label': 'toggle-public' }}
    />
  )
}

const FolderChildrenList = (props) => {
  const record = useRecordContext()
  const resource = useResourceContext() || 'folder'
  const isDiscovery = resource === 'discoveryFolder'
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh(resource)

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      updatedAt: isDesktop && <DateField source="updatedAt" />,
      public: !isXsmall && <TogglePublicInput source="public" />,
    }),
    [isDesktop, isXsmall]
  )

  const columns = useSelectedFields({
    resource,
    columns: toggleableFields,
  })

  const parentId = record?.id ?? ''

  const handleRowClick = useCallback(
    (id, rec) => {
      const folderPath = isDiscovery ? '/discoveryFolder' : '/folder'
      const playlistPath = isDiscovery ? '/discovery' : '/playlist'
      return rec?.type === 'folder'
        ? `${folderPath}/${id}/show`
        : `${playlistPath}/${id}/show`
    },
    [isDiscovery],
  )

  return (
    <List
      {...props}
      resource={resource}
      exporter={false}
      title={<Title subTitle={record?.name} />}
      filters={<PlaylistFolderFilter />}
      actions={<PlaylistListActions />}
      bulkActionButtons={!isXsmall && <PlaylistFolderBulkActions resource={resource} />}
      empty={<EmptyPlaylist />}
      perPage={isXsmall ? 50 : 50}
      filter={{ parent_id: parentId }}
      filterDefaultValues={{ parent_id: parentId }}
    >
      <PlaylistFolderDataGrid rowClick={handleRowClick}>
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

const PlaylistFolderShow = (props) => (
  <ShowBase {...props}>
    <FolderChildrenList {...props} />
  </ShowBase>
)

export default PlaylistFolderShow
