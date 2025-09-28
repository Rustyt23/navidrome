import httpClient from './httpClient'
import { REST_URL } from '../consts'
import { pushNotification } from '../layout/notificationStore'

const notifiedPlaylists = new Set()

const parseTotalCount = (response) => {
  const header = response.headers?.get('X-Total-Count')
  if (!header) {
    return undefined
  }
  const parsed = parseInt(header, 10)
  return Number.isNaN(parsed) ? undefined : parsed
}

const fetchPlaylistName = async (playlistId, fallbackName) => {
  if (fallbackName) {
    return fallbackName
  }
  const response = await httpClient(`${REST_URL}/playlist/${playlistId}`)
  const data = response.json || {}
  return data.name || fallbackName || playlistId
}

const fetchMissingCount = async (playlistId) => {
  const response = await httpClient(
    `${REST_URL}/playlist/${playlistId}/tracks?missing=true&_start=0&_end=1`,
  )
  const headerCount = parseTotalCount(response)
  if (headerCount !== undefined) {
    return headerCount
  }
  const payload = response.json
  if (Array.isArray(payload)) {
    return payload.length
  }
  if (payload && Array.isArray(payload.data)) {
    return payload.data.length
  }
  return 0
}

export const notifyPlaylistMissingSongs = async (playlistId, fallbackName) => {
  if (!playlistId || notifiedPlaylists.has(playlistId)) {
    return
  }
  try {
    const missingCount = await fetchMissingCount(playlistId)
    if (missingCount <= 0) {
      return
    }
    const playlistName = await fetchPlaylistName(playlistId, fallbackName)
    pushNotification({
      title: 'Missing songs detected',
      description: `${missingCount} songs in playlist '${playlistName}' are missing from the server.`,
      time: 'Just now',
    })
    notifiedPlaylists.add(playlistId)
  } catch (error) {
    // Silently ignore failures to avoid disrupting create flow
  }
}
