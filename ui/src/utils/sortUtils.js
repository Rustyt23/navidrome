import { useCallback, useMemo, useState } from 'react'

export const SortDirection = {
  ASC: 'asc',
  DESC: 'desc',
}

export const SortType = {
  AUTO: 'auto',
  STRING: 'string',
  NUMBER: 'number',
  DATE: 'date',
}

const COLLATOR_CACHE = new Map()

const getCollator = (locale, options) => {
  const key = `${locale || 'default'}::${JSON.stringify(options || {})}`
  if (!COLLATOR_CACHE.has(key)) {
    COLLATOR_CACHE.set(key, new Intl.Collator(locale, options))
  }
  return COLLATOR_CACHE.get(key)
}

const normalizeDirection = (direction) =>
  direction && direction.toLowerCase() === SortDirection.DESC
    ? SortDirection.DESC
    : SortDirection.ASC

const numberLikeRegex = /^-?\d+(?:\.\d+)?$/

const inferSortType = (values, fallback = SortType.STRING) => {
  if (!Array.isArray(values)) {
    return fallback
  }
  for (let index = 0; index < values.length; index += 1) {
    const value = values[index]
    if (value === null || value === undefined) {
      continue
    }
    if (value instanceof Date) {
      return SortType.DATE
    }
    const valueType = typeof value
    if (valueType === 'number') {
      return SortType.NUMBER
    }
    if (valueType === 'boolean') {
      return SortType.NUMBER
    }
    if (valueType === 'string') {
      const trimmed = value.trim()
      if (trimmed === '') {
        continue
      }
      if (numberLikeRegex.test(trimmed)) {
        return SortType.NUMBER
      }
      const parsedDate = Date.parse(trimmed)
      if (!Number.isNaN(parsedDate)) {
        return SortType.DATE
      }
      return SortType.STRING
    }
  }
  return fallback
}

const normalizeValue = (value, type) => {
  if (value === null || value === undefined) {
    return null
  }
  switch (type) {
    case SortType.NUMBER: {
      if (typeof value === 'number') {
        return Number.isFinite(value) ? value : null
      }
      if (typeof value === 'boolean') {
        return value ? 1 : 0
      }
      if (typeof value === 'string') {
        const trimmed = value.trim()
        if (trimmed === '') {
          return null
        }
        const coerced = Number(trimmed)
        return Number.isFinite(coerced) ? coerced : null
      }
      if (value instanceof Date) {
        const time = value.getTime()
        return Number.isFinite(time) ? time : null
      }
      return null
    }
    case SortType.DATE: {
      if (value instanceof Date) {
        const time = value.getTime()
        return Number.isFinite(time) ? time : null
      }
      if (typeof value === 'number') {
        return Number.isFinite(value) ? value : null
      }
      if (typeof value === 'string') {
        const trimmed = value.trim()
        if (trimmed === '') {
          return null
        }
        const parsed = Date.parse(trimmed)
        return Number.isNaN(parsed) ? null : parsed
      }
      return null
    }
    case SortType.STRING:
    default: {
      const base = typeof value === 'string' ? value : String(value)
      return base.replace(/^\s+/, '')
    }
  }
}

export const smartSort = (
  items,
  {
    accessor,
    direction = SortDirection.ASC,
    type = SortType.AUTO,
    locale,
  } = {},
) => {
  const source = Array.isArray(items) ? items : []
  if (source.length === 0) {
    return []
  }

  const extractor =
    typeof accessor === 'function'
      ? accessor
      : (row) => (row == null || accessor == null ? row : row[accessor])

  const decorated = source.map((item, index) => ({
    index,
    item,
    rawValue: extractor(item, index),
  }))

  const resolvedType =
    type === SortType.AUTO
      ? inferSortType(
          decorated.map((entry) => entry.rawValue),
          SortType.STRING,
        )
      : type

  const collator =
    resolvedType === SortType.STRING
      ? getCollator(locale, { sensitivity: 'base', numeric: false })
      : null

  decorated.forEach((entry) => {
    entry.normalizedValue = normalizeValue(entry.rawValue, resolvedType)
  })

  const multiplier = normalizeDirection(direction) === SortDirection.DESC ? -1 : 1

  decorated.sort((a, b) => {
    const { normalizedValue: valueA } = a
    const { normalizedValue: valueB } = b

    if (valueA === null && valueB === null) {
      return a.index - b.index
    }
    if (valueA === null) {
      return 1 * multiplier
    }
    if (valueB === null) {
      return -1 * multiplier
    }

    if (resolvedType === SortType.STRING) {
      const comparison = collator.compare(valueA, valueB)
      if (comparison !== 0) {
        return comparison * multiplier
      }
    } else if (valueA < valueB) {
      return -1 * multiplier
    } else if (valueA > valueB) {
      return 1 * multiplier
    }

    return a.index - b.index
  })

  return decorated.map((entry) => entry.item)
}

export const useSmartSort = (
  data,
  {
    initialKey = null,
    initialDirection = SortDirection.ASC,
    columns = {},
    defaultType = SortType.AUTO,
  } = {},
) => {
  const columnConfig = useMemo(() => {
    const entries = Object.entries(columns || {})
    if (!entries.length) {
      return {}
    }
    return entries.reduce((acc, [key, config]) => {
      if (!config) {
        return acc
      }
      acc[key] = {
        accessor: config.accessor,
        type: config.type || defaultType,
        locale: config.locale,
        defaultDirection: normalizeDirection(
          config.defaultDirection || SortDirection.ASC,
        ),
      }
      return acc
    }, {})
  }, [columns, defaultType])

  const [sortState, setSortState] = useState(() => {
    if (!initialKey) {
      return {
        key: null,
        direction: normalizeDirection(initialDirection),
        type: defaultType,
        customAccessor: null,
      }
    }
    const column = columnConfig[initialKey] || {}
    return {
      key: initialKey,
      direction: column.defaultDirection || normalizeDirection(initialDirection),
      type: column.type || defaultType,
      customAccessor: column.accessor ? null : null,
    }
  })

  const sortedData = useMemo(() => {
    const source = Array.isArray(data) ? data : []
    if (!sortState.key) {
      return source
    }
    const column = columnConfig[sortState.key] || {}
    const accessor =
      sortState.customAccessor || column.accessor || ((row) => row?.[sortState.key])

    return smartSort(source, {
      accessor,
      direction: sortState.direction,
      type: sortState.type || column.type || defaultType,
      locale: column.locale,
    })
  }, [data, sortState, columnConfig, defaultType])

  const requestSort = useCallback(
    (key, overrides = {}) => {
      if (!key) {
        return
      }
      setSortState((previous) => {
        const isSameKey = previous.key === key
        const column = columnConfig[key] || {}
        const nextDirection = (() => {
          if (overrides.direction) {
            return normalizeDirection(overrides.direction)
          }
          if (isSameKey) {
            return previous.direction === SortDirection.ASC
              ? SortDirection.DESC
              : SortDirection.ASC
          }
          return column.defaultDirection || SortDirection.ASC
        })()

        return {
          key,
          direction: nextDirection,
          type: overrides.type || column.type || defaultType,
          customAccessor:
            typeof overrides.accessor === 'function' ? overrides.accessor : null,
        }
      })
    },
    [columnConfig, defaultType],
  )

  const clearSort = useCallback(() => {
    setSortState((previous) => ({
      key: null,
      direction: previous.direction,
      type: previous.type,
      customAccessor: null,
    }))
  }, [])

  const getSortDirection = useCallback(
    (key) => {
      if (!key || sortState.key !== key) {
        return null
      }
      return sortState.direction
    },
    [sortState.key, sortState.direction],
  )

  return {
    sortedData,
    sortState,
    requestSort,
    getSortDirection,
    clearSort,
  }
}

