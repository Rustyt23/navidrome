import React, { cloneElement } from 'react'
import { sanitizeListRestProps, TopToolbar } from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import { ShuffleAllButton, ToggleFieldsMenu } from '../common'

export const SongListActions = ({
  currentSort,
  className,
  resource,
  filters,
  displayedFilters,
  filterValues,
  permanentFilter,
  exporter,
  basePath,
  selectedIds,
  onUnselectItems,
  showFilter,
  maxResults,
  total,
  ids,
  columnsOrder,
  onColumnsOrderChange,
  ...rest
}) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      <ShuffleAllButton filters={filterValues} />
      {filters &&
        cloneElement(filters, {
          resource,
          showFilter,
          displayedFilters,
          filterValues,
          context: 'button',
        })}
      {isNotSmall && (
        <ToggleFieldsMenu
          resource="song"
          columnsOrder={columnsOrder}
          onColumnsOrderChange={onColumnsOrderChange}
        />
      )}
    </TopToolbar>
  )
}

SongListActions.defaultProps = {
  selectedIds: [],
  onUnselectItems: () => null,
}
