import React, { useEffect } from 'react'
import { Pagination as RAPagination, useListContext } from 'react-admin'
import {
  SONGS_PAGINATION_OPTIONS,
  SONGS_PAGINATION_STORAGE_KEY,
} from './songsPaginationConfig'

export const SongsPagination = (props) => {
  const { perPage } = useListContext()

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }

    if (SONGS_PAGINATION_OPTIONS.includes(perPage)) {
      window.localStorage.setItem(
        SONGS_PAGINATION_STORAGE_KEY,
        perPage.toString(),
      )
    }
  }, [perPage])

  return <RAPagination rowsPerPageOptions={SONGS_PAGINATION_OPTIONS} {...props} />
}
