import React, { useMemo } from 'react'
import PlaylistDetails from '../playlist/PlaylistDetails'

const DiscoveryDetails = (props) => {
  const { record, ...rest } = props
  const playlistRecord = useMemo(() => {
    if (!record) {
      return record
    }
    return { ...record, sync: false }
  }, [record])

  return <PlaylistDetails {...rest} record={playlistRecord} />
}

export default DiscoveryDetails
