import { useJobStatus } from '../lufs/useJobStatus'

export const SILENCE_ANALYZE_URL = '/api/song/silence-trim/analyze'
export const SILENCE_APPLY_URL = '/api/song/silence-trim/apply'

export const useSilenceAnalyzeStatus = () => useJobStatus(SILENCE_ANALYZE_URL)

export const useSilenceApplyStatus = () => useJobStatus(SILENCE_APPLY_URL)
