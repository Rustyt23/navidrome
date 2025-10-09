import { useCallback } from 'react'
import PropTypes from 'prop-types'
import { FunctionField, useListContext } from 'react-admin'

const PlaylistTrackNumberField = (props) => {
  const { ids = [], page = 1, perPage } = useListContext()

  const renderValue = useCallback(
    (record) => {
      if (!record) {
        return ''
      }
      const index = ids.indexOf(record.id)
      if (index === -1) {
        return ''
      }
      const offset = perPage ? (page - 1) * perPage : 0
      return offset + index + 1
    },
    [ids, page, perPage],
  )

  return <FunctionField {...props} render={renderValue} />
}

PlaylistTrackNumberField.propTypes = {
  label: PropTypes.oneOfType([PropTypes.string, PropTypes.object]),
}

export default PlaylistTrackNumberField
