import { useRef, useState } from 'react'
import CloudUploadIcon from '@material-ui/icons/CloudUpload'
import { Button, CircularProgress } from '@material-ui/core'
import { useNotify, useRefresh, useTranslate } from 'react-admin'
import { httpClientRaw } from '../dataProvider/httpClient'

const PlaylistUploadButton = () => {
  const inputRef = useRef(null)
  const [loading, setLoading] = useState(false)
  const notify = useNotify()
  const refresh = useRefresh()
  const translate = useTranslate()

  const handleClick = () => {
    if (loading) {
      return
    }
    inputRef.current?.click()
  }

  const handleChange = async (event) => {
    const file = event.target.files?.[0]
    if (!file) {
      return
    }

    setLoading(true)
    try {
      const response = await httpClientRaw('/api/playlist', {
        method: 'POST',
        body: file,
        headers: new Headers({
          'Content-Type': file.type || 'audio/x-mpegurl',
        }),
      })

      if (!response.ok) {
        const message = await response.text()
        throw new Error(message || 'resources.playlist.notifications.uploadError')
      }

      notify('resources.playlist.notifications.uploaded', 'info', {
        name: file.name,
      })
      refresh()
    } catch (error) {
      const message = error?.message?.trim()
      notify('resources.playlist.notifications.uploadError', 'warning', {
        _: message,
        name: file.name,
      })
    } finally {
      setLoading(false)
      if (event.target) {
        event.target.value = ''
      }
    }
  }

  return (
    <>
      <input
        ref={inputRef}
        type="file"
        accept=".m3u,.m3u8,.nsp,text/plain,audio/x-mpegurl"
        style={{ display: 'none' }}
        onChange={handleChange}
      />
      <Button
        color="primary"
        onClick={handleClick}
        startIcon={
          loading ? <CircularProgress size={18} color="inherit" /> : <CloudUploadIcon />
        }
        disabled={loading}
      >
        {translate('resources.playlist.actions.upload')}
      </Button>
    </>
  )
}

export default PlaylistUploadButton
