import { useRecordContext } from 'react-admin'
import { PathField } from '../common'

const PlaylistPathField = (props) => {
  const record = useRecordContext(props)

  if (!record || record.missing) {
    return <span />
  }

  return <PathField {...props} />
}

export default PlaylistPathField
