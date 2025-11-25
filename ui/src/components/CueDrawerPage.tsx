import React, { useEffect, useState } from 'react'
import { FaTimes } from 'react-icons/fa'

const DEVICE_ID = 'b2f2a1c7-cf47-46fa-8d11-3d77ed15e65d'

type CueTrigger = {
  id?: string
  name?: string
  description?: string
  triggerID?: string
}

type CueDrawerPageProps = {
  open: boolean
  onClose: () => void
}

const CueDrawerPage: React.FC<CueDrawerPageProps> = ({ open, onClose }) => {
  const [cues, setCues] = useState<CueTrigger[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) {
      return
    }
    const fetchCues = async () => {
      setLoading(true)
      try {
        const response = await fetch(`/api/retailplayer/triggers?device=${DEVICE_ID}`)
        if (!response.ok) {
          throw new Error('Unable to load cues')
        }
        const data = await response.json()
        const triggers = Array.isArray(data?.data) ? data.data : data?.triggers || data
        setCues(triggers || [])
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error(err)
        setCues([])
      } finally {
        setLoading(false)
      }
    }

    fetchCues()
  }, [open])

  const playCue = async (cueId?: string) => {
    if (!cueId) return
    try {
      await fetch('/api/retailplayer/play', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ device: DEVICE_ID, cue: cueId }),
      })
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error(err)
    }
  }

  const stopCue = async () => {
    try {
      await fetch('/api/retailplayer/stop', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ device: DEVICE_ID }),
      })
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error(err)
    }
  }

  return (
    <div className={`cue-drawer ${open ? 'open' : ''}`}>
      <div className="cue-drawer-header">
        <h3>RetailPlayer Cues</h3>
        <button className="cue-close" onClick={onClose} aria-label="Close cue drawer">
          <FaTimes />
        </button>
      </div>
      <div className="cue-drawer-actions">
        <button className="cue-stop" onClick={stopCue}>
          Stop
        </button>
      </div>
      <div className="cue-drawer-body">
        {loading ? (
          <div className="cue-loading">Loading cues...</div>
        ) : cues.length === 0 ? (
          <div className="cue-empty">No cues available.</div>
        ) : (
          cues.map((cue) => {
            const cueId = cue.id || cue.triggerID
            const label = cue.name || cue.description || cueId || 'Cue'
            return (
              <button
                key={cueId || label}
                className="cue-entry"
                onClick={() => playCue(cueId)}
              >
                {label}
              </button>
            )
          })
        )}
      </div>
    </div>
  )
}

export default CueDrawerPage
