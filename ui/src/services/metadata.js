import httpClient from '../dataProvider/httpClient'

export const getAllSongsMetadata = async () => {
  try {
    const response = await httpClient('/rest/getSongs.view?f=json')
    const payload = response?.data ?? response?.json
    return payload?.['subsonic-response']?.songs?.song ?? []
  } catch (error) {
    throw error
  }
}
