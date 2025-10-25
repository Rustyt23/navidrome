import React from 'react'
import PropTypes from 'prop-types'
import { BiDislike } from 'react-icons/bi'
import { MdSkipNext } from 'react-icons/md'
import {
  VolumeMuteIcon,
  VolumeUnmuteIcon,
} from 'navidrome-music-player/es/components/Icon'

const NowPlayingControls = ({ isMuted, onDislike, onMuteToggle, onSkip }) => {
  return (
    <div className="group now-playing-controls" role="group" aria-label="Now playing controls">
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
    </div>
  )
}

NowPlayingControls.propTypes = {
  isMuted: PropTypes.bool,
  onDislike: PropTypes.func.isRequired,
  onMuteToggle: PropTypes.func.isRequired,
  onSkip: PropTypes.func.isRequired,
}

NowPlayingControls.defaultProps = {
  isMuted: false,
}

export default NowPlayingControls
