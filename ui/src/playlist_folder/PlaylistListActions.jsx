import { cloneElement } from 'react'
import { sanitizeListRestProps, TopToolbar } from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import { ToggleFieldsMenu } from '../common'
import PlaylistFolderCreateButton from './PlaylistFolderCreateButton'

const PlaylistListActions = ({ className, ...rest }) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {rest.filters ? cloneElement(rest.filters, { context: 'button' }) : null}
      <PlaylistFolderCreateButton recordId={rest?.filterValues?.parent_id} />
      {isNotSmall && <ToggleFieldsMenu resource="folder" />}
    </TopToolbar>
  )
}

export default PlaylistListActions
