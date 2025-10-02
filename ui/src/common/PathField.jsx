import PropTypes from 'prop-types'
import React from 'react'
import { usePermissions, useRecordContext } from 'react-admin'
import config from '../config'

export const PathField = (props) => {
  const record = useRecordContext(props) || {}
  const { permissions } = usePermissions()

  if (!record.path || record.missing) {
    return <span></span>
  }

  const libraryPath = permissions === 'admin' ? record.libraryPath || '' : ''

  if (!libraryPath) {
    return <span>{record.path}</span>
  }

  const separator = libraryPath.endsWith(config.separator)
    ? ''
    : config.separator

  return <span>{`${libraryPath}${separator}${record.path}`}</span>
}

PathField.propTypes = {
  record: PropTypes.object,
}
