import React from 'react'
import { Pagination as RAPagination, useListContext } from 'react-admin'

export const Pagination = (props) => {
  const { perPage } = useListContext()
  const rowsPerPage = perPage ?? 50

  return (
    <RAPagination
      rowsPerPageOptions={[25, 50, 100, 200, 500]}
      rowsPerPage={rowsPerPage}
      {...props}
    />
  )
}
