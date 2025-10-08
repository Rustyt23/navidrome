import PropTypes from 'prop-types'
import React from 'react'
import { usePermissions, useRecordContext } from 'react-admin'
import config from '../config'

export const PathField = (props) => {
  const record = useRecordContext(props)
  const { permissions } = usePermissions()

  if (!record || record.missing || !record.path) {
    return <span></span>
  }

  const basePath = permissions === 'admin' ? record.libraryPath : ''

  if (!basePath) {
    return <span>{record.path}</span>
  }

  const separator = config.separator
  const normalizedBasePath = basePath.endsWith(separator)
    ? basePath
    : `${basePath}${separator}`

  return <span>{`${normalizedBasePath}${record.path}`}</span>
}

PathField.propTypes = {
  record: PropTypes.object,
}
