import { useSelector } from 'react-redux'

// MusicBrainz surfaces (fetch buttons, progress cards, external links) can be
// hidden across the whole UI from Settings > Personal, leaving only the Spotify
// equivalents visible. Defaults to visible when the setting was never set.
export const useMusicBrainzVisible = () =>
  useSelector((state) => state.settings?.showMusicBrainz !== false)
