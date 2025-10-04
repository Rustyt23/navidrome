import React, { cloneElement } from 'react'
import {
  sanitizeListRestProps,
  TopToolbar,
  CreateButton,
  useTranslate,
  useResourceContext,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import { ToggleFieldsMenu } from '../common'

const PlaylistListActions = ({ className, ...rest }) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  const translate = useTranslate()
  const resource = useResourceContext() || 'playlist'
  const basePath = resource === 'discovery' ? '/discovery' : '/playlist'

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {cloneElement(rest.filters, { context: 'button' })}
      <CreateButton basePath={basePath}>
        {translate('ra.action.create')}
      </CreateButton>
      {isNotSmall && <ToggleFieldsMenu resource={resource} />}
    </TopToolbar>
  )
}

export default PlaylistListActions
