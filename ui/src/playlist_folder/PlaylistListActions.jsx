import { cloneElement } from 'react'
import { sanitizeListRestProps, TopToolbar, useResourceContext } from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import { ToggleFieldsMenu } from '../common'
import PlaylistFolderCreateButton from './PlaylistFolderCreateButton'

const PlaylistListActions = ({ className, ...rest }) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  const resource = useResourceContext() || 'folder'

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {rest.filters ? cloneElement(rest.filters, { context: 'button' }) : null}
      <PlaylistFolderCreateButton
        recordId={rest?.filterValues?.parent_id}
        resource={resource}
      />
      {isNotSmall && <ToggleFieldsMenu resource={resource} />}
    </TopToolbar>
  )
}

export default PlaylistListActions
