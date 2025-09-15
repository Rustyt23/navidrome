import React, { useCallback, useEffect, useMemo, useRef } from 'react'
import PropTypes from 'prop-types'
import { TablePagination, Toolbar, useMediaQuery } from '@material-ui/core'
import {
  sanitizeListRestProps,
  useListPaginationContext,
  useTranslate,
  useStore,
} from 'react-admin'
import DefaultPaginationActions from 'ra-ui-materialui/esm/list/pagination/PaginationActions'
import DefaultPaginationLimit from 'ra-ui-materialui/esm/list/pagination/PaginationLimit'
import {
  DEFAULT_PLAYLIST_MOBILE_PER_PAGE,
  PLAYLIST_MOBILE_PER_PAGE_STORAGE_KEY,
  PLAYLIST_MOBILE_ROWS_PER_PAGE_OPTIONS,
} from './playlistPaginationConfig'

const PlaylistMobilePagination = ({
  rowsPerPageOptions = PLAYLIST_MOBILE_ROWS_PER_PAGE_OPTIONS,
  actions = DefaultPaginationActions,
  limit = <DefaultPaginationLimit />,
  ...rest
}) => {
  const { loading, page, perPage, total, setPage, setPerPage } =
    useListPaginationContext(rest)
  const translate = useTranslate()
  const isSmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const [storedPerPage, setStoredPerPage] = useStore(
    PLAYLIST_MOBILE_PER_PAGE_STORAGE_KEY,
    DEFAULT_PLAYLIST_MOBILE_PER_PAGE,
  )
  const totalPages = useMemo(
    () => Math.ceil(total / perPage) || 1,
    [perPage, total],
  )
  const hasSyncedPerPage = useRef(false)

  useEffect(() => {
    if (!hasSyncedPerPage.current) {
      if (!rowsPerPageOptions.includes(storedPerPage)) {
        setStoredPerPage(DEFAULT_PLAYLIST_MOBILE_PER_PAGE)
        setPerPage(DEFAULT_PLAYLIST_MOBILE_PER_PAGE)
      } else if (perPage !== storedPerPage) {
        setPerPage(storedPerPage)
      }
      hasSyncedPerPage.current = true
      return
    }

    if (rowsPerPageOptions.includes(perPage) && storedPerPage !== perPage) {
      setStoredPerPage(perPage)
    }
  }, [
    perPage,
    rowsPerPageOptions,
    setPerPage,
    setStoredPerPage,
    storedPerPage,
  ])

  const handlePageChange = useCallback(
    (event, newPage) => {
      if (event) {
        event.stopPropagation()
      }
      if (newPage < 0 || newPage > totalPages - 1) {
        throw new Error(
          translate('ra.navigation.page_out_of_boundaries', {
            page: newPage + 1,
          }),
        )
      }
      setPage(newPage + 1)
    },
    [setPage, totalPages, translate],
  )

  const handlePerPageChange = useCallback(
    (event) => {
      const value = parseInt(event.target.value, 10)
      if (!Number.isNaN(value)) {
        setPerPage(value)
        setStoredPerPage(value)
      }
    },
    [setPerPage, setStoredPerPage],
  )

  const labelDisplayedRows = useCallback(
    ({ from, to, count }) =>
      translate('ra.navigation.page_range_info', {
        offsetBegin: from,
        offsetEnd: to,
        total: count,
      }),
    [translate],
  )

  if (total === null || total === 0 || page < 1 || page > totalPages) {
    return loading ? <Toolbar variant="dense" /> : limit
  }

  const tablePaginationProps = {
    count: total,
    rowsPerPage: perPage,
    page: page - 1,
    onPageChange: handlePageChange,
    onRowsPerPageChange: handlePerPageChange,
    component: 'span',
    labelRowsPerPage: translate('ra.navigation.page_rows_per_page'),
    labelDisplayedRows,
    rowsPerPageOptions,
    ...sanitizeListRestProps(rest),
  }

  if (!isSmall) {
    tablePaginationProps.ActionsComponent = actions
  }

  return <TablePagination {...tablePaginationProps} />
}

PlaylistMobilePagination.propTypes = {
  actions: PropTypes.elementType,
  limit: PropTypes.element,
  rowsPerPageOptions: PropTypes.arrayOf(PropTypes.number),
}

export default PlaylistMobilePagination
