import React, { useMemo } from 'react'
import {
  Datagrid,
  DateField,
  EditButton,
  Filter,
  NumberField,
  ReferenceInput,
  SearchInput,
  SelectInput,
  TextField,
  useUpdate,
  useNotify,
  useRecordContext,
  BulkDeleteButton,
  usePermissions,
  useResourceContext,
} from 'react-admin'
import Switch from '@material-ui/core/Switch'
import { useMediaQuery } from '@material-ui/core'
import {
  DurationField,
  List,
  Writable,
  isWritable,
  useSelectedFields,
  useResourceRefresh,
} from '../common'
import DiscoveryListActions from './DiscoveryListActions'
import ChangePublicStatusButton from '../playlist/ChangePublicStatusButton'

const DiscoveryFilter = (props) => {
  const { permissions } = usePermissions()
  const resource = useResourceContext() || 'discovery'
  return (
    <Filter {...props} variant={'outlined'}>
      <SearchInput source="q" alwaysOn />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label={`resources.${resource}.fields.ownerName`}
          reference="user"
          perPage={0}
          sort={{ field: 'name', order: 'ASC' }}
          alwaysOn
        >
          <SelectInput optionText="name" />
        </ReferenceInput>
      )}
    </Filter>
  )
}

const TogglePublicInput = ({ resource, source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const contextResource = useResourceContext() || 'discovery'
  const targetResource = resource || contextResource
  const [togglePublic] = useUpdate(
    targetResource,
    record.id,
    {
      ...record,
      public: !record.public,
    },
    {
      undoable: false,
      onFailure: () => {
        notify('ra.page.error', 'warning')
      },
    },
  )

  const handleClick = (e) => {
    togglePublic()
    e.stopPropagation()
  }

  return (
    <Switch
      checked={record[source]}
      onClick={handleClick}
      disabled={!isWritable(record.ownerId)}
    />
  )
}

const ToggleAutoImport = ({ resource, source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const contextResource = useResourceContext() || 'discovery'
  const targetResource = resource || contextResource
  const [ToggleAutoImport] = useUpdate(
    targetResource,
    record.id,
    {
      ...record,
      sync: !record.sync,
    },
    {
      undoable: false,
      onFailure: () => {
        notify('ra.page.error', 'warning')
      },
    },
  )
  const handleClick = (e) => {
    ToggleAutoImport()
    e.stopPropagation()
  }

  return record.path ? (
    <Switch
      checked={record[source]}
      onClick={handleClick}
      disabled={!isWritable(record.ownerId)}
    />
  ) : null
}

const DiscoveryListBulkActions = (props) => (
  <>
    <ChangePublicStatusButton public={true} {...props} />
    <ChangePublicStatusButton public={false} {...props} />
    <BulkDeleteButton {...props} />
  </>
)

const DiscoveryList = (props) => {
  const resource = useResourceContext() || 'discovery'
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh(resource)

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      songCount: !isXsmall && <NumberField source="songCount" />,
      duration: <DurationField source="duration" />,
      updatedAt: isDesktop && (
        <DateField source="updatedAt" sortByOrder={'DESC'} />
      ),
      createdAt: <DateField source="createdAt" showTime />,
      public:
        !isXsmall && (
          <TogglePublicInput resource={resource} source="public" sortByOrder={'DESC'} />
        ),
      comment: <TextField source="comment" />,
      sync: <ToggleAutoImport resource={resource} source="sync" sortByOrder={'DESC'} />,
    }),
    [isDesktop, isXsmall, resource],
  )

  const columns = useSelectedFields({
    resource,
    columns: toggleableFields,
    defaultOff: ['comment'],
  })

  return (
    <List
      {...props}
      exporter={false}
      filters={<DiscoveryFilter />}
      actions={<DiscoveryListActions />}
      bulkActionButtons={!isXsmall && <DiscoveryListBulkActions />}
    >
      <Datagrid rowClick="show" isRowSelectable={(r) => isWritable(r?.ownerId)}>
        <TextField source="name" />
        {columns}
        <Writable>
          <EditButton />
        </Writable>
      </Datagrid>
    </List>
  )
}

export default DiscoveryList
