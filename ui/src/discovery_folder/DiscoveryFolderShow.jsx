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
import DiscoveryListActions from './DiscoveryListActions'
import EmptyDiscovery from './EmptyDiscovery'
import TypeIconField from './TypeIconField'
import TypeAwareEditButton from './TypeAwareEditButton'
import DiscoveryFolderBulkActions from './DiscoveryFolderBulkActions'
import { DiscoveryFolderDataGrid } from './DiscoveryFolderDataGrid'

const DiscoveryFolderFilter = (props) => {
  const { permissions } = usePermissions()
  return (
    <Filter {...props} variant="outlined">
      <SearchInput source="q" alwaysOn resettable />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label="resources.playlist.fields.ownerName"
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
      size="small"
      inputProps={{ 'aria-label': 'toggle-public' }}
    />
  )
}

const rowClick = (id, record) =>
  record?.type === 'folder' ? `/folder/${id}/show` : `/playlist/${id}/show`

const FolderChildrenList = (props) => {
  const record = useRecordContext()
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('folder')

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      updatedAt: isDesktop && <DateField source="updatedAt" />,
      public: !isXsmall && <TogglePublicInput source="public" />,
    }),
    [isDesktop, isXsmall]
  )

  const columns = useSelectedFields({
    resource: 'folder',
    columns: toggleableFields,
  })

  const parentId = record?.id ?? ''

  return (
    <List
      {...props}
      resource="folder"
      exporter={false}
      title={<Title subTitle={record?.name} />}
      filters={<DiscoveryFolderFilter />}
      actions={<DiscoveryListActions />}
      bulkActionButtons={!isXsmall && <DiscoveryFolderBulkActions />}
      empty={<EmptyDiscovery />}
      perPage={isXsmall ? 50 : 50}
      filter={{ parent_id: parentId }}
      filterDefaultValues={{ parent_id: parentId }}
    >
      <DiscoveryFolderDataGrid rowClick={rowClick}>
        <TypeIconField label={false} />
        <TextField source="name" />
        {columns}
        <Writable>
          <TypeAwareEditButton />
        </Writable>
      </DiscoveryFolderDataGrid>
    </List>
  )
}

const DiscoveryFolderShow = (props) => (
  <ShowBase {...props}>
    <FolderChildrenList {...props} />
  </ShowBase>
)

export default DiscoveryFolderShow
