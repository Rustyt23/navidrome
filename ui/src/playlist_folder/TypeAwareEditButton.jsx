import { useCallback } from 'react'
import { Button, useRecordContext } from 'react-admin'
import EditIcon from '@material-ui/icons/Edit'
import { Link } from 'react-router-dom'

const TypeAwareEditButton = () => {
  const record = useRecordContext()

  const stop = useCallback((e) => e.stopPropagation(), [])

  if (!record) return null

  return (
   <Button
      component={Link}
      to={`/${record.type}/${record.id}`}
      label="ra.action.edit"
      onClick={stop}
      size="small"
      style={{ minWidth: 0, padding: '0px 0px', fontSize: 12 }}
    >
      <EditIcon fontSize="small" style={{ fontSize: 14 }} />
    </Button>
  )
}

export default TypeAwareEditButton
