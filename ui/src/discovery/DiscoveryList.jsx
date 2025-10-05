import React, { useMemo } from 'react'
import {
  Datagrid,
  DateField,
  Filter,
  NumberField,
  SearchInput,
  TextField,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import {
  DurationField,
  List,
  SizeField,
  useSelectedFields,
  useResourceRefresh,
} from '../common'
import DiscoveryListActions from './DiscoveryListActions'

const DiscoveryFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="q" alwaysOn />
  </Filter>
)

const DiscoveryList = (props) => {
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('discovery')

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      songCount: !isXsmall && <NumberField source="songCount" />,
      duration: <DurationField source="duration" />,
      size: isDesktop && <SizeField source="size" />, 
      updatedAt: isDesktop && <DateField source="updatedAt" showTime />, 
      createdAt: <DateField source="createdAt" showTime />, 
      comment: <TextField source="comment" />, 
    }),
    [isDesktop, isXsmall],
  )

  const columns = useSelectedFields({
    resource: 'discovery',
    columns: toggleableFields,
    defaultOff: ['comment'],
  })

  return (
    <List
      {...props}
      exporter={false}
      filters={<DiscoveryFilter />}
      actions={<DiscoveryListActions />}
      bulkActionButtons={false}
    >
      <Datagrid rowClick="show">
        <TextField source="name" />
        {columns}
      </Datagrid>
    </List>
  )
}

export default DiscoveryList
