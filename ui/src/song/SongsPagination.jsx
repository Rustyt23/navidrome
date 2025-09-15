import React, { useCallback, useEffect, useMemo, useRef } from 'react'
import PropTypes from 'prop-types'
import { TablePagination, Toolbar, useMediaQuery } from '@material-ui/core'
import {
  ComponentPropType,
  sanitizeListRestProps,
  useListPaginationContext,
  useTranslate,
} from 'react-admin'
import DefaultPaginationActions from 'ra-ui-materialui/esm/list/pagination/PaginationActions'
import DefaultPaginationLimit from 'ra-ui-materialui/esm/list/pagination/PaginationLimit'
import {
  DEFAULT_SONGS_PER_PAGE,
  SONGS_PER_PAGE_STORAGE_KEY,
  SONGS_ROWS_PER_PAGE_OPTIONS,
} from './songPaginationConfig'

const SongsPagination = (props) => {
  const {
    rowsPerPageOptions = SONGS_ROWS_PER_PAGE_OPTIONS,
    actions = DefaultPaginationActions,
    limit = <DefaultPaginationLimit />,
    ...rest
  } = props

  const { loading, page, perPage, total, setPage, setPerPage } =
    useListPaginationContext(props)
  const translate = useTranslate()
  const isSmall = useMediaQuery((theme) => theme.breakpoints.down('sm'))
  const totalPages = useMemo(() => Math.ceil(total / perPage) || 1, [perPage, total])
  const hasSyncedPerPage = useRef(false)

  useEffect(() => {
    if (hasSyncedPerPage.current) {
      return
    }
    if (typeof window === 'undefined' || !window.localStorage) {
      hasSyncedPerPage.current = true
      return
    }
    const storedValue = parseInt(
      window.localStorage.getItem(SONGS_PER_PAGE_STORAGE_KEY),
      10,
    )
    if (rowsPerPageOptions.includes(storedValue) && storedValue !== perPage) {
      setPerPage(storedValue)
    } else if (!rowsPerPageOptions.includes(perPage)) {
      setPerPage(DEFAULT_SONGS_PER_PAGE)
    } else {
      window.localStorage.setItem(
        SONGS_PER_PAGE_STORAGE_KEY,
        perPage.toString(),
      )
    }
    hasSyncedPerPage.current = true
  }, [perPage, rowsPerPageOptions, setPerPage])

  useEffect(() => {
    if (typeof window === 'undefined' || !window.localStorage) {
      return
    }
    if (rowsPerPageOptions.includes(perPage)) {
      window.localStorage.setItem(
        SONGS_PER_PAGE_STORAGE_KEY,
        perPage.toString(),
      )
    }
  }, [perPage, rowsPerPageOptions])

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
      }
    },
    [setPerPage],
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

SongsPagination.propTypes = {
  actions: ComponentPropType,
  limit: PropTypes.element,
  rowsPerPageOptions: PropTypes.arrayOf(PropTypes.number),
}

export default SongsPagination
