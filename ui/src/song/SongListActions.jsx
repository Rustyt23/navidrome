import React, { cloneElement } from 'react'
import { sanitizeListRestProps, TopToolbar } from 'react-admin'
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
  ...rest
}) => {
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
      <ToggleFieldsMenu resource="song" />
    </TopToolbar>
  )
}

SongListActions.defaultProps = {
  selectedIds: [],
  onUnselectItems: () => null,
}
