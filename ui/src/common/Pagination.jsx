import React from 'react'
import { Pagination as RAPagination } from 'react-admin'

export const Pagination = (props) => (
 
 <RAPagination
 rowsPerPageOptions={[25, 50, 100, 200, 500]} {...props}
 rowsPerPage={50} //
 />
)
