import React, { useEffect, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import { createPortal } from 'react-dom'
import { BiDislike } from 'react-icons/bi'
import { MdSkipNext } from 'react-icons/md'
import {
  VolumeMuteIcon,
  VolumeUnmuteIcon,
} from 'navidrome-music-player/es/components/Icon'

const DEFAULT_CONTAINER_SELECTOR =
  '.music-player-panel .panel-content .player-content .play-sounds'

const NowPlayingControls = ({
  anchorKey,
  containerSelector,
  isMuted,
  onDislike,
  onMuteToggle,
  onSkip,
}) => {
  const selector = useMemo(
    () => containerSelector || DEFAULT_CONTAINER_SELECTOR,
    [containerSelector],
  )
  const [container, setContainer] = useState(null)

  useEffect(() => {
    const element = document.querySelector(selector)
    setContainer(element || null)
  }, [anchorKey, selector])

  if (!container) {
    return null
  }

  return createPortal(
    <div className="now-playing-controls" role="group" aria-label="Now playing controls">
      <div className="now-playing-buttons">
        <button
          type="button"
          aria-label="Dislike"
          className="now-playing-button"
          onClick={onDislike}
        >
          <BiDislike size={26} />
        </button>
        <button
          type="button"
          aria-label="Mute/Unmute"
          className="now-playing-button"
          onClick={onMuteToggle}
        >
          {isMuted ? <VolumeMuteIcon size={26} /> : <VolumeUnmuteIcon size={26} />}
        </button>
        <button
          type="button"
          aria-label="Skip"
          className="now-playing-button"
          onClick={onSkip}
        >
          <MdSkipNext size={26} />
        </button>
      </div>
    </div>,
    container,
  )
}

NowPlayingControls.propTypes = {
  anchorKey: PropTypes.string,
  containerSelector: PropTypes.string,
  isMuted: PropTypes.bool,
  onDislike: PropTypes.func.isRequired,
  onMuteToggle: PropTypes.func.isRequired,
  onSkip: PropTypes.func.isRequired,
}

NowPlayingControls.defaultProps = {
  anchorKey: undefined,
  containerSelector: DEFAULT_CONTAINER_SELECTOR,
  isMuted: false,
}

export default NowPlayingControls
