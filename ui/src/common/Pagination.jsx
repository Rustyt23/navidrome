import React, { useEffect, useMemo } from 'react'
import { Pagination as RAPagination, useListContext } from 'react-admin'

import { DEFAULT_PAGE_SIZE, PAGE_SIZES } from '../consts'

export const Pagination = ({ rowsPerPageOptions, ...props }) => {
  const listContext = useListContext(props)
  const { perPage, setPerPage } = listContext

  const allowedOptions = useMemo(() => {
    if (rowsPerPageOptions && rowsPerPageOptions.length > 0) {
      return rowsPerPageOptions
    }
    return PAGE_SIZES
  }, [rowsPerPageOptions])

  const fallbackPerPage = useMemo(() => {
    if (allowedOptions.includes(DEFAULT_PAGE_SIZE)) {
      return DEFAULT_PAGE_SIZE
    }
    return allowedOptions[0] ?? DEFAULT_PAGE_SIZE
  }, [allowedOptions])

  const sanitizedPerPage = useMemo(() => {
    if (allowedOptions.includes(perPage)) {
      return perPage
    }
    return fallbackPerPage
  }, [allowedOptions, fallbackPerPage, perPage])

  useEffect(() => {
    if (!setPerPage) {
      return
    }

    if (perPage === undefined) {
      setPerPage(sanitizedPerPage)
      return
    }

    if (!allowedOptions.includes(perPage)) {
      setPerPage(sanitizedPerPage)
    }
  }, [allowedOptions, perPage, sanitizedPerPage, setPerPage])

  return (
    <RAPagination
      rowsPerPageOptions={allowedOptions}
      rowsPerPage={sanitizedPerPage}
      {...props}
    />
  )
}
