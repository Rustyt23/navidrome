import httpClient from '../dataProvider/httpClient'
import subsonic from '../subsonic'

const PAGE_SIZE = 500

const extractSongs = (payload) => {
  const subsonicResponse = payload?.['subsonic-response']
  if (!subsonicResponse) {
    throw new Error('Invalid response from server')
  }

  if (subsonicResponse.status && subsonicResponse.status.toLowerCase() !== 'ok') {
    throw new Error(subsonicResponse.error?.message || 'Unable to fetch songs metadata')
  }

  return Array.isArray(subsonicResponse?.songs?.song)
    ? subsonicResponse.songs.song
    : subsonicResponse?.songs?.song
    ? [subsonicResponse.songs.song]
    : []
}

export const getAllSongsMetadata = async () => {
  const allSongs = []
  let offset = 0
  let hasMore = true

  while (hasMore) {
    const response = await httpClient(
      subsonic.url('getSongs', null, { count: PAGE_SIZE, offset }),
    )
    const payload = response?.json ?? response?.data
    const songs = extractSongs(payload)
    allSongs.push(...songs)

    if (songs.length < PAGE_SIZE) {
      hasMore = false
    } else {
      offset += songs.length
    }
  }

  return allSongs
}

export const fetchMissingMetadata = async (songs = []) => {
  if (!Array.isArray(songs) || songs.length === 0) {
    return []
  }

  const response = await httpClient('/api/metadata/fetch', {
    method: 'POST',
    body: JSON.stringify({ songs }),
    headers: new Headers({
      Accept: 'application/json',
      'Content-Type': 'application/json',
    }),
  })

  const payload = response?.json ?? response?.data
  if (!payload) {
    return []
  }

  if (Array.isArray(payload)) {
    return payload
  }

  return Array.isArray(payload.songs) ? payload.songs : []
}
