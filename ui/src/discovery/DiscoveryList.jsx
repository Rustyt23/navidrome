import React, { useMemo } from 'react'
import { List, Datagrid, TextField, NumberField, DateField } from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import { DurationField, useSelectedFields, useResourceRefresh } from '../common'

const DiscoveryList = (props) => {
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('discovery')

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      songCount: !isXsmall && <NumberField source="songCount" />,
      duration: <DurationField source="duration" />,
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
    <List {...props} exporter={false} actions={false}>
      <Datagrid rowClick="show">
        <TextField source="name" />
        {columns}
      </Datagrid>
    </List>
  )
}

export default DiscoveryList
