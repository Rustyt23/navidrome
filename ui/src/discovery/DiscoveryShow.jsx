import React from 'react'
import {
  Show,
  SimpleShowLayout,
  TextField,
  NumberField,
  DateField,
  ReferenceManyField,
  useShowController,
  ShowContextProvider,
} from 'react-admin'
import { Card, CardContent, Typography } from '@material-ui/core'
import { DurationField, Title, useResourceRefresh } from '../common'
import DiscoverySongs from './DiscoverySongs'

const DiscoveryDetails = ({ record }) => {
  useResourceRefresh('song')
  if (!record) {
    return null
  }
  return (
    <Card>
      <CardContent>
        <Typography variant="h5">{record.name}</Typography>
        <Typography variant="body2" color="textSecondary">
          {record.comment}
        </Typography>
      </CardContent>
    </Card>
  )
}

const DiscoveryShowLayout = (props) => {
  const { record } = props
  return (
    <>
      {record && <Title subTitle={record.name} />}
      <DiscoveryDetails record={record} />
      <SimpleShowLayout {...props}>
        <TextField source="name" />
        <TextField source="ownerName" />
        <NumberField source="songCount" />
        <DurationField source="duration" />
        <DateField source="createdAt" showTime />
        <DateField source="updatedAt" showTime />
      </SimpleShowLayout>
      {record && (
        <ReferenceManyField
          reference="discoveryTrack"
          target="discovery_id"
          sort={{ field: 'id', order: 'ASC' }}
          perPage={100}
          addLabel={false}
        >
          <DiscoverySongs discoveryId={record.id} />
        </ReferenceManyField>
      )}
    </>
  )
}

const DiscoveryShow = (props) => {
  const controllerProps = useShowController(props)
  return (
    <ShowContextProvider value={controllerProps}>
      <Show {...props} component="div">
        <DiscoveryShowLayout {...controllerProps} {...props} />
      </Show>
    </ShowContextProvider>
  )
}

export default DiscoveryShow
