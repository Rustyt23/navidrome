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
