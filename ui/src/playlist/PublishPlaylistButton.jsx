import React from 'react'
import PropTypes from 'prop-types'
import PublishIcon from '@material-ui/icons/Publish'
import { Button, Confirm, useTranslate, useNotify } from 'react-admin'

import { REST_URL } from '../consts'
import { httpClient } from '../dataProvider'

const PublishPlaylistButton = ({ record }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const [open, setOpen] = React.useState(false)
  const [loading, setLoading] = React.useState(false)

  const handleDialogOpen = React.useCallback(() => {
    setOpen(true)
  }, [])

  const handleDialogClose = React.useCallback(() => {
    if (!loading) {
      setOpen(false)
    }
  }, [loading])

  const handlePublish = React.useCallback(() => {
    if (!record?.id) {
      setOpen(false)
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
        setOpen(false)
      })
      .catch(() => {
        notify('ra.page.error', 'warning')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [notify, record])

  return (
    <>
      <Button
        onClick={handleDialogOpen}
        disabled={loading}
        label={translate('resources.playlist.actions.publish')}
      >
        <PublishIcon />
      </Button>
      <Confirm
        isOpen={open}
        loading={loading}
        title={translate('resources.playlist.actions.publish')}
        content={translate('ra.message.are_you_sure')}
        onConfirm={handlePublish}
        onClose={handleDialogClose}
      />
    </>
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
