import React from 'react'
import { Pagination as RAPagination } from 'react-admin'

export const Pagination = ({ rowsPerPage, ...rest }) => (
  <RAPagination
    rowsPerPageOptions={[25, 50, 100, 200, 500]}
    rowsPerPage={rowsPerPage ?? 50}
    {...rest}
  />
)
