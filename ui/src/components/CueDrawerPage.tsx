import React, { useEffect, useMemo, useState } from 'react'

const RETAIL_PLAYER_DEVICE_ID = 'b2f2a1c7-cf47-46fa-8d11-3d77ed15e65d'

type Trigger = {
  id?: string
  name?: string
  label?: string
  title?: string
  description?: string
}

type CueDrawerPageProps = {
  open: boolean
  onClose: () => void
}

const CueDrawerPage: React.FC<CueDrawerPageProps> = ({ open, onClose }) => {
  const [triggers, setTriggers] = useState<Trigger[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const triggerList = useMemo(() => {
    if (Array.isArray(triggers)) {
      return triggers
    }
    return []
  }, [triggers])

  useEffect(() => {
    if (open) {
      loadTriggers()
    }
  }, [open])

  const loadTriggers = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await fetch(
        `/api/retailplayer/triggers?device=${RETAIL_PLAYER_DEVICE_ID}`,
      )
      if (!response.ok) {
        throw new Error('Failed to load cues')
      }
      const data = await response.json()
      const entries = Array.isArray(data?.data)
        ? data.data
        : Array.isArray(data)
          ? data
          : []
      setTriggers(entries)
    } catch (err) {
      setError('Unable to load cues')
    } finally {
      setLoading(false)
    }
  }

  const handlePlay = async (cue: string | undefined) => {
    if (!cue) return
    try {
      await fetch('/api/retailplayer/play', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          device: RETAIL_PLAYER_DEVICE_ID,
          cue,
        }),
      })
    } catch (err) {
      setError('Unable to start cue')
    }
  }

  const handleStop = async () => {
    try {
      await fetch('/api/retailplayer/stop', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ device: RETAIL_PLAYER_DEVICE_ID }),
      })
    } catch (err) {
      setError('Unable to stop cue')
    }
  }

  if (!open) {
    return null
  }

  return (
    <div className="cue-drawer-overlay">
      <div className={`cue-drawer ${open ? 'open' : ''}`}>
        <div className="cue-drawer-header">
          <div className="cue-drawer-title">Cues</div>
          <button
            aria-label="Close cues"
            className="cue-close"
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </div>
        <div className="cue-drawer-subtitle">Press a button to Play</div>
        <button
          type="button"
          className="cue-action stop"
          onClick={handleStop}
        >
          Stop
        </button>
        <div className="cue-list">
          {loading && <div className="cue-status">Loading cues…</div>}
          {!loading && error && <div className="cue-status error">{error}</div>}
          {!loading && !error && triggerList.length === 0 && (
            <div className="cue-status">No cues available</div>
          )}
          {!loading &&
            !error &&
            triggerList.map((trigger) => {
              const id = trigger.id || trigger.title || trigger.name || trigger.label
              const label = trigger.label || trigger.name || trigger.title || 'Cue'
              return (
                <button
                  key={id}
                  type="button"
                  className="cue-action"
                  onClick={() => handlePlay(id || '')}
                >
                  {label}
                </button>
              )
            })}
        </div>
      </div>
    </div>
  )
}

export default CueDrawerPage
