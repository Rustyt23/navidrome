import { useCallback } from 'react'
import { Button, useRecordContext, useResourceContext } from 'react-admin'
import EditIcon from '@material-ui/icons/Edit'
import { Link } from 'react-router-dom'

const TypeAwareEditButton = () => {
  const record = useRecordContext()
  const resource = useResourceContext() || 'folder'

  const stop = useCallback((e) => e.stopPropagation(), [])

  if (!record) return null

  let target = `/${record.type}/${record.id}`
  if (record.type === 'folder') {
    target = `${resource === 'discoveryFolder' ? '/discoveryFolder' : '/folder'}/${record.id}`
  } else if (record.type === 'discovery') {
    target = `/discovery/${record.id}`
  }

  return (
    <Button
      component={Link}
      to={target}
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
