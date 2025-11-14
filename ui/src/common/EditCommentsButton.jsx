import React, { useCallback } from 'react'
import PropTypes from 'prop-types'
import { useDispatch } from 'react-redux'
import { Button, useTranslate, useUnselectAll } from 'react-admin'
import { BiCommentEdit } from 'react-icons/bi'
import { openEditComments } from '../actions'

export const EditCommentsButton = ({
  resource,
  selectedIds = [],
  targetIds,
  initialComment = '',
  className,
}) => {
  const translate = useTranslate()
  const dispatch = useDispatch()
  const unselectAll = useUnselectAll()
  const effectiveTargetIds =
    targetIds && targetIds.length ? targetIds : selectedIds

  const handleClick = useCallback(() => {
    if (!effectiveTargetIds || effectiveTargetIds.length === 0) {
      return
    }
    dispatch(
      openEditComments({
        resource,
        selectedIds,
        targetIds: effectiveTargetIds,
        initialComment,
        onSuccess: () => unselectAll(resource),
      }),
    )
  }, [
    dispatch,
    resource,
    selectedIds,
    effectiveTargetIds,
    initialComment,
    unselectAll,
  ])

  return (
    <Button
      onClick={handleClick}
      className={className}
      disabled={!effectiveTargetIds || effectiveTargetIds.length === 0}
      label={translate('resources.song.actions.editComments')}
    >
      <BiCommentEdit />
    </Button>
  )
}

EditCommentsButton.propTypes = {
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  targetIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  initialComment: PropTypes.string,
  className: PropTypes.string,
}
