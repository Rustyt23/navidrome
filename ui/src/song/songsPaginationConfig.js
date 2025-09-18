export const SONGS_PAGINATION_OPTIONS = [15, 25, 50, 100, 200]
export const SONGS_PAGINATION_STORAGE_KEY = 'songs.rowsPerPage'
export const SONGS_DEFAULT_PER_PAGE = 50

export const getStoredSongsPerPage = () => {
  if (typeof window === 'undefined') {
    return SONGS_DEFAULT_PER_PAGE
  }

  const storedValue = window.localStorage.getItem(SONGS_PAGINATION_STORAGE_KEY)
  const parsedValue = storedValue ? parseInt(storedValue, 10) : NaN

  return SONGS_PAGINATION_OPTIONS.includes(parsedValue)
    ? parsedValue
    : SONGS_DEFAULT_PER_PAGE
}
