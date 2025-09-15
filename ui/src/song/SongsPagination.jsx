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
  SONGS_ROWS_PER_PAGE_OPTIONS,
} from './songPaginationConfig'

const SongsPagination = (props) => {
  const {
    rowsPerPageOptions = SONGS_ROWS_PER_PAGE_OPTIONS,
    actions = DefaultPaginationActions,
    limit = <DefaultPaginationLimit />,
    onPerPageChange,
    ...rest
  } = props

  const { loading, page, perPage, total, setPage, setPerPage } =
    useListPaginationContext(props)
  const translate = useTranslate()
  const isSmall = useMediaQuery((theme) => theme.breakpoints.down('sm'))
  const totalPages = useMemo(() => Math.ceil(total / perPage) || 1, [perPage, total])
  const lastStoredPerPage = useRef()

  useEffect(() => {
    const fallback = rowsPerPageOptions.includes(DEFAULT_SONGS_PER_PAGE)
      ? DEFAULT_SONGS_PER_PAGE
      : rowsPerPageOptions[0]

    if (!rowsPerPageOptions.includes(perPage)) {
      if (perPage !== fallback) {
        setPerPage(fallback)
      }
      if (onPerPageChange) {
        onPerPageChange(fallback)
      }
      lastStoredPerPage.current = fallback
      return
    }

    if (lastStoredPerPage.current !== perPage) {
      if (onPerPageChange) {
        onPerPageChange(perPage)
      }
      lastStoredPerPage.current = perPage
    }
  }, [
    onPerPageChange,
    perPage,
    rowsPerPageOptions,
    setPerPage,
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
        if (onPerPageChange) {
          onPerPageChange(value)
        }
      }
    },
    [onPerPageChange, setPerPage],
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
  onPerPageChange: PropTypes.func,
}

export default SongsPagination
