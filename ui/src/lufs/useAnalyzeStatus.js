import { useJobStatus } from './useJobStatus'

export const ANALYZE_URL = '/api/song/loudness/analyze'

// useAnalyzeStatus follows the measuring pass, which reads files and rewrites
// their audit records but never touches the audio.
export const useAnalyzeStatus = () => useJobStatus(ANALYZE_URL)
