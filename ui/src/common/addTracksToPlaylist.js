import { httpClient } from '../dataProvider'
import { REST_URL } from '../consts'

export const addTracksToPlaylist = async (
  playlistId,
  trackIds,
  duplicatePolicy = 'allow',
) => {
  if (!trackIds?.length) {
    return { added: 0 }
  }
  const ids = duplicatePolicy === 'skip' ? Array.from(new Set(trackIds)) : trackIds
  const res = await httpClient(`${REST_URL}/playlist/${playlistId}/tracks`, {
    method: 'POST',
    body: JSON.stringify({ ids }),
  })
  return res.json
}

export default addTracksToPlaylist
