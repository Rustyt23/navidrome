import { useCallback } from 'react'
import { Button, useRecordContext } from 'react-admin'
import EditIcon from '@material-ui/icons/Edit'
import { Link } from 'react-router-dom'

const TypeAwareEditButton = () => {
  const record = useRecordContext()

  const stop = useCallback((e) => e.stopPropagation(), [])

  if (!record) return null

  return (
    <Button component={Link} to={`/${record.type}/${record.id}`} label="ra.action.edit" onClick={stop}>
      <EditIcon />
    </Button>
  )
}

export default TypeAwareEditButton
