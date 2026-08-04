import { useJobStatus } from './useJobStatus'

export const RESTORE_URL = '/api/song/loudness/restore'

// useRestoreStatus follows the run that puts stored originals back.
//
// Restoring used to happen inside the request that asked for it, so a large
// selection left the browser waiting on a request that could outlast it, with
// nothing to show for the wait. It is a background job like the other two now,
// and is watched the same way.
export const useRestoreStatus = () => useJobStatus(RESTORE_URL)
