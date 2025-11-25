import React, { useState } from 'react'
import { FaBullhorn } from 'react-icons/fa'
import PlayerToolbar from '../audioplayer/PlayerToolbar'
import CueDrawerPage from './CueDrawerPage'

type PlayerControlsProps = {
  id?: string
  isRadio?: boolean
}

const PlayerControls: React.FC<PlayerControlsProps> = ({ id, isRadio }) => {
  const [cueOpen, setCueOpen] = useState(false)

  return (
    <>
      <div className="cue-control-wrapper">
        <PlayerToolbar id={id} isRadio={isRadio} />
        <button
          className="cue-button"
          onClick={() => setCueOpen(true)}
          aria-label="Open cue drawer"
        >
          <div className="cue-icon">
            <FaBullhorn />
          </div>
        </button>
      </div>
      <CueDrawerPage open={cueOpen} onClose={() => setCueOpen(false)} />
    </>
  )
}

export default PlayerControls
