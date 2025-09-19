import { useCallback, useEffect, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import TablePagination from '@material-ui/core/TablePagination'
import { useListContext } from 'react-admin'

import { DEFAULT_PAGE_SIZE, PAGE_SIZES } from '../consts'
import {
  getStoredPageSize as getPersistedPageSize,
  setStoredPageSize,
} from './paginationStorage'

const ensureOptions = (options) =>
  Array.isArray(options) && options.length > 0 ? options : PAGE_SIZES

const sanitizePageSize = (value, options, fallback) => {
  const allowed = ensureOptions(options)
  const numeric = Number.parseInt(value, 10)

  if (Number.isFinite(numeric) && allowed.includes(numeric)) {
    return numeric
  }

  if (Number.isFinite(fallback) && allowed.includes(fallback)) {
    return fallback
  }

  return allowed.includes(DEFAULT_PAGE_SIZE)
    ? DEFAULT_PAGE_SIZE
    : allowed[0]
}

const readStoredPageSize = (scope, options, fallback) => {
  const allowed = ensureOptions(options)
  const sanitizedFallback = sanitizePageSize(fallback, allowed, DEFAULT_PAGE_SIZE)

  if (!scope) {
    return sanitizedFallback
  }

  const stored = getPersistedPageSize(scope, DEFAULT_PAGE_SIZE)
  return sanitizePageSize(stored, allowed, sanitizedFallback)
}

export const Pagination = ({
  scope,
  page: pageProp,
  perPage: perPageProp,
  total: totalProp,
  onPageChange,
  onPerPageChange,
  rowsPerPageOptions = PAGE_SIZES,
  className,
  classes,
  style,
}) => {
  const allowedOptions = useMemo(
    () => ensureOptions(rowsPerPageOptions),
    [rowsPerPageOptions],
  )

  const listContext = useListContext()
  const {
    page: contextPage,
    perPage: contextPerPage,
    total: contextTotal,
    setPage: contextSetPage,
    setPerPage: contextSetPerPage,
  } = listContext || {}

  const hasListContext =
    typeof contextSetPage === 'function' &&
    typeof contextSetPerPage === 'function'

  const [preferredPerPage, setPreferredPerPage] = useState(() =>
    readStoredPageSize(scope, allowedOptions, perPageProp ?? DEFAULT_PAGE_SIZE),
  )

  useEffect(() => {
    const next = readStoredPageSize(
      scope,
      allowedOptions,
      perPageProp ?? DEFAULT_PAGE_SIZE,
    )
    setPreferredPerPage((prev) => (prev === next ? prev : next))
  }, [scope, allowedOptions, perPageProp])

  const effectivePerPage = hasListContext
    ? sanitizePageSize(contextPerPage, allowedOptions, preferredPerPage)
    : sanitizePageSize(perPageProp, allowedOptions, preferredPerPage)

  const effectivePage = hasListContext ? contextPage ?? 1 : pageProp ?? 1
  const effectiveTotal = hasListContext
    ? typeof contextTotal === 'number' && contextTotal >= 0
      ? contextTotal
      : 0
    : typeof totalProp === 'number' && totalProp >= 0
      ? totalProp
      : 0

  useEffect(() => {
    if (!hasListContext || !scope) {
      return
    }
    const sanitizedContext = sanitizePageSize(
      contextPerPage,
      allowedOptions,
      preferredPerPage,
    )
    if (sanitizedContext !== preferredPerPage) {
      contextSetPerPage?.(preferredPerPage)
      contextSetPage?.(1)
    }
  }, [
    hasListContext,
    scope,
    preferredPerPage,
    contextPerPage,
    allowedOptions,
    contextSetPerPage,
    contextSetPage,
  ])

  const handlePageChange = useCallback(
    (_, newPage) => {
      const nextPage = newPage + 1
      if (hasListContext) {
        contextSetPage?.(nextPage)
      } else if (typeof onPageChange === 'function') {
        onPageChange(nextPage)
      }
    },
    [hasListContext, contextSetPage, onPageChange],
  )

  const handleRowsPerPageChange = useCallback(
    (event) => {
      const selected = sanitizePageSize(
        event.target.value,
        allowedOptions,
        preferredPerPage,
      )
      setPreferredPerPage(selected)

      if (scope) {
        setStoredPageSize(scope, selected)
      }

      if (hasListContext) {
        contextSetPerPage?.(selected)
        contextSetPage?.(1)
      } else {
        if (typeof onPerPageChange === 'function') {
          onPerPageChange(selected)
        }
        if (typeof onPageChange === 'function') {
          onPageChange(1)
        }
      }
    },
    [
      allowedOptions,
      preferredPerPage,
      scope,
      hasListContext,
      contextSetPerPage,
      contextSetPage,
      onPerPageChange,
      onPageChange,
    ],
  )

  const labelDisplayedRows = useCallback(
    ({ from, to, count }) =>
      `${from}-${to} of ${count < 0 ? to : count ?? 0}`,
    [],
  )

  const currentPage = Math.max(0, (effectivePage || 1) - 1)
  const rowsPerPage = sanitizePageSize(
    effectivePerPage,
    allowedOptions,
    preferredPerPage,
  )

  return (
    <TablePagination
      component="div"
      className={className}
      classes={classes}
      style={style}
      rowsPerPageOptions={allowedOptions}
      rowsPerPage={rowsPerPage}
      page={currentPage}
      count={effectiveTotal}
      onChangePage={handlePageChange}
      onChangeRowsPerPage={handleRowsPerPageChange}
      labelRowsPerPage="Items per page:"
      labelDisplayedRows={labelDisplayedRows}
    />
  )
}

Pagination.propTypes = {
  scope: PropTypes.string,
  page: PropTypes.number,
  perPage: PropTypes.number,
  total: PropTypes.number,
  onPageChange: PropTypes.func,
  onPerPageChange: PropTypes.func,
  rowsPerPageOptions: PropTypes.arrayOf(PropTypes.number),
  className: PropTypes.string,
  classes: PropTypes.object,
  style: PropTypes.object,
}

Pagination.defaultProps = {
  rowsPerPageOptions: PAGE_SIZES,
  page: undefined,
  perPage: undefined,
  total: undefined,
  onPageChange: undefined,
  onPerPageChange: undefined,
  scope: undefined,
  className: undefined,
  classes: undefined,
  style: undefined,
}
