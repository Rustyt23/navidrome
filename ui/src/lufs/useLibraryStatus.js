import { useJobStatus } from './useJobStatus'

export const LIBRARY_URL = '/api/song/loudness/library'

// useLibraryStatus follows the run that rewrites files - the whole-library
// pass, and the pass that applies phase 2 decisions.
export const useLibraryStatus = () => useJobStatus(LIBRARY_URL)
