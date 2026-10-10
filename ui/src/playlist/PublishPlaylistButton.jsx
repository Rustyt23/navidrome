import React from 'react'
import PropTypes from 'prop-types'
import PublishIcon from '@material-ui/icons/Publish'
import { Button as RaButton, useTranslate, useNotify } from 'react-admin'
import { REST_URL } from '../consts'
import { httpClient } from '../dataProvider'

const PublishPlaylistButton = ({ record }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const [loading, setLoading] = React.useState(false)

  const handlePublish = React.useCallback(() => {
    if (loading || !record?.id) {
      return Promise.resolve()
    }

    setLoading(true)
    return httpClient(`${REST_URL}/playlist/${record.id}/publish`, {
      method: 'POST',
    })
      .then(() => {
        notify('resources.playlist.notifications.published', 'info', {
          smart_count: record.songCount,
          name: record.name,
        })
      })
      .catch(() => {
        notify('ra.page.error', 'warning')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [loading, notify, record])

  return (
    <RaButton
      onClick={handlePublish}
      disabled={loading || !record?.id}
      label={translate('resources.playlist.actions.publish')}
    >
      <PublishIcon />
    </RaButton>
  )
}

PublishPlaylistButton.propTypes = {
  record: PropTypes.shape({
    id: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
    name: PropTypes.string,
    songCount: PropTypes.number,
  }),
}

PublishPlaylistButton.defaultProps = {
  record: null,
}

export default PublishPlaylistButton
