import React, { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { FaBullhorn } from 'react-icons/fa'
import CueDrawerPage from './CueDrawerPage'

const PlayerControls: React.FC = () => {
  const [cueOpen, setCueOpen] = useState(false)
  const [controlGroup, setControlGroup] = useState<Element | null>(null)

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setControlGroup(
        document.querySelector(
          '.react-jinke-music-player-main .group, .react-jinke-music-player .group',
        ),
      )
    }, 0)
    return () => window.clearTimeout(handle)
  }, [])

  const cueButton = (
    <button
      type="button"
      className="cue-button"
      aria-label="Open cue drawer"
      onClick={() => setCueOpen(true)}
    >
      <div className="cue-icon">
        <FaBullhorn />
      </div>
    </button>
  )

  return (
    <>
      {controlGroup && createPortal(cueButton, controlGroup)}
      <CueDrawerPage open={cueOpen} onClose={() => setCueOpen(false)} />
    </>
  )
}

export default PlayerControls
