import React, { cloneElement } from 'react'
import {
  sanitizeListRestProps,
  TopToolbar,
  useTranslate,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import { ToggleFieldsMenu } from '../common'

const DiscoveryListActions = ({ className, filters, ...rest }) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  const translate = useTranslate()

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {filters && cloneElement(filters, { context: 'button' })}
      <span className="ra-top-toolbar__label">{translate('resources.discovery.name')}</span>
      {isNotSmall && <ToggleFieldsMenu resource="discovery" />}
    </TopToolbar>
  )
}

export default DiscoveryListActions
