import {
  buildRetailStatusUrl,
  calculateSpectrumMetrics,
  getDeviceFromLocation,
  getNavidromeBasePath,
  getNowPlaying,
} from './player-utils.js'

const POLL_INTERVAL_MS = 2000
const NEXT_TRACK_INTERVAL_MS = 650
const PLAY_PATH = 'M20 12 52 32 20 52Z'
const PAUSE_PATH = 'M18 14H27V50H18ZM37 14H46V50H37Z'
const REDUCED_MOTION = window.matchMedia(
  '(prefers-reduced-motion: reduce)',
).matches

const audio = document.getElementById('retail-audio')
const button = document.getElementById('play-button')
const icon = document.getElementById('play-icon')
const statusElement = document.getElementById('player-status')
const canvas = document.getElementById('resonance')
const device = getDeviceFromLocation(window.location)
const basePath = getNavidromeBasePath(window.location.pathname)
const statusUrl = device ? buildRetailStatusUrl(basePath, device) : ''

let audioContext = null
let analyser = null
let frequencyData = null
let currentTrack = null
let userStarted = false
let waitingForNextTrack = false
let pollTimer = null
let syncInFlight = false
let animationFrame = null
let bassBaseline = 0.08
let lastBeatAt = 0

const setStatus = (message) => {
  statusElement.textContent = message
}

const setPlayingState = (isPlaying) => {
  document.body.classList.toggle('is-playing', isPlaying)
  button.setAttribute(
    'aria-label',
    isPlaying ? 'Pause retail device' : 'Play retail device',
  )
  icon.setAttribute('d', isPlaying ? PAUSE_PATH : PLAY_PATH)
}

class ResonanceField {
  constructor(targetCanvas) {
    this.canvas = targetCanvas
    this.context = targetCanvas.getContext('2d', { alpha: false })
    this.width = 1
    this.height = 1
    this.pixelRatio = 1
    this.active = false
    this.energy = 0
    this.bass = 0
    this.centroid = 0
    this.beatPulse = 0
    this.mode = [2, 4]
    this.modes = [
      [1, 2],
      [2, 3],
      [2, 4],
      [3, 4],
      [3, 5],
      [4, 6],
      [5, 7],
      [6, 8],
    ]
    this.particleCount = REDUCED_MOTION ? 1500 : 4800
    this.x = new Float32Array(this.particleCount)
    this.y = new Float32Array(this.particleCount)
    this.vx = new Float32Array(this.particleCount)
    this.vy = new Float32Array(this.particleCount)

    for (let index = 0; index < this.particleCount; index += 1) {
      this.x[index] = Math.random()
      this.y[index] = Math.random()
    }

    this.resize = this.resize.bind(this)
    window.addEventListener('resize', this.resize)
    this.resize()
  }

  resize() {
    this.pixelRatio = Math.min(window.devicePixelRatio || 1, 2)
    this.width = Math.max(1, window.innerWidth)
    this.height = Math.max(1, window.innerHeight)
    this.canvas.width = Math.round(this.width * this.pixelRatio)
    this.canvas.height = Math.round(this.height * this.pixelRatio)
    this.context.setTransform(this.pixelRatio, 0, 0, this.pixelRatio, 0, 0)
    this.context.fillStyle = '#05070a'
    this.context.fillRect(0, 0, this.width, this.height)
  }

  setActive(active) {
    this.active = active
  }

  setAudio(metrics, beat) {
    this.energy += (metrics.energy - this.energy) * 0.25
    this.bass += (metrics.bass - this.bass) * 0.3
    this.centroid += (metrics.centroid - this.centroid) * 0.12

    if (beat) {
      this.beatPulse = 1
      const modeIndex = Math.min(
        this.modes.length - 1,
        Math.floor(this.centroid * this.modes.length * 1.8),
      )
      this.mode = this.modes[modeIndex]
    }
  }

  gaussianNoise() {
    return Math.random() + Math.random() + Math.random() - 1.5
  }

  update() {
    if (!this.active) {
      return
    }

    const [modeM, modeN] = this.mode
    const a = modeM * Math.PI
    const b = modeN * Math.PI
    const kick = this.beatPulse
    const movement = REDUCED_MOTION ? 0.35 : 1
    const force = (0.000004 + this.energy * 0.000016) * movement
    const jitter = (0.0001 + this.bass * 0.0014 + kick * 0.003) * movement

    for (let index = 0; index < this.particleCount; index += 1) {
      let x = this.x[index]
      let y = this.y[index]
      const sinAX = Math.sin(a * x)
      const cosAX = Math.cos(a * x)
      const sinBX = Math.sin(b * x)
      const cosBX = Math.cos(b * x)
      const sinAY = Math.sin(a * y)
      const cosAY = Math.cos(a * y)
      const sinBY = Math.sin(b * y)
      const cosBY = Math.cos(b * y)
      const amplitude = sinAX * sinBY + sinBX * sinAY
      const gradientX = amplitude * (a * cosAX * sinBY + b * cosBX * sinAY)
      const gradientY = amplitude * (b * sinAX * cosBY + a * sinBX * cosAY)
      let velocityX =
        (this.vx[index] - gradientX * force + jitter * this.gaussianNoise()) *
        0.9
      let velocityY =
        (this.vy[index] - gradientY * force + jitter * this.gaussianNoise()) *
        0.9

      x += velocityX
      y += velocityY

      if (x < 0) {
        x = -x
        velocityX *= -0.5
      } else if (x > 1) {
        x = 2 - x
        velocityX *= -0.5
      }
      if (y < 0) {
        y = -y
        velocityY *= -0.5
      } else if (y > 1) {
        y = 2 - y
        velocityY *= -0.5
      }

      this.x[index] = x
      this.y[index] = y
      this.vx[index] = velocityX
      this.vy[index] = velocityY
    }

    this.beatPulse *= 0.86
  }

  render() {
    if (!this.active) {
      return
    }

    const plateSize = Math.min(this.width, this.height) * 0.9
    const offsetX = (this.width - plateSize) / 2
    const offsetY = (this.height - plateSize) / 2
    const grainSize = Math.max(0.8, Math.min(1.65, plateSize / 620))
    const glow = Math.min(1, this.energy * 2.4 + this.beatPulse * 0.65)

    this.context.fillStyle = `rgba(5, 7, 10, ${0.2 + (1 - glow) * 0.12})`
    this.context.fillRect(0, 0, this.width, this.height)
    this.context.fillStyle = `rgba(255, ${190 + Math.round(glow * 35)}, ${
      116 + Math.round(glow * 60)
    }, ${0.34 + glow * 0.58})`

    for (let index = 0; index < this.particleCount; index += 1) {
      this.context.fillRect(
        offsetX + this.x[index] * plateSize,
        offsetY + this.y[index] * plateSize,
        grainSize,
        grainSize,
      )
    }
  }

  frame() {
    this.update()
    this.render()
  }
}

const resonance = new ResonanceField(canvas)

const ensureAudioGraph = async () => {
  if (!audioContext) {
    const AudioContext = window.AudioContext || window.webkitAudioContext
    audioContext = new AudioContext()
    analyser = audioContext.createAnalyser()
    analyser.fftSize = 1024
    analyser.smoothingTimeConstant = 0.72
    frequencyData = new Uint8Array(analyser.frequencyBinCount)
    const source = audioContext.createMediaElementSource(audio)
    source.connect(analyser)
    analyser.connect(audioContext.destination)
  }

  if (audioContext.state === 'suspended') {
    await audioContext.resume()
  }
}

const loadTrack = async (track, shouldPlay) => {
  const isNewTrack = !currentTrack || currentTrack.signature !== track.signature
  currentTrack = track
  document.title = track.artist
    ? `${track.title} — ${track.artist}`
    : track.title
  setStatus(
    track.artist
      ? `Now playing ${track.title} by ${track.artist}`
      : `Now playing ${track.title}`,
  )

  if (isNewTrack || !audio.src) {
    audio.src = new URL(track.streamUrl, window.location.href).href
    audio.load()
  }

  if (shouldPlay && (isNewTrack || audio.paused)) {
    waitingForNextTrack = false
    try {
      await audio.play()
      resonance.setActive(true)
      setPlayingState(true)
    } catch (error) {
      setPlayingState(false)
      setStatus(`Playback could not start: ${error.message}`)
    }
  }
}

const scheduleSync = (delay = POLL_INTERVAL_MS) => {
  window.clearTimeout(pollTimer)
  pollTimer = window.setTimeout(syncDevice, delay)
}

const syncDevice = async () => {
  if (!statusUrl || syncInFlight) {
    scheduleSync(POLL_INTERVAL_MS)
    return
  }

  syncInFlight = true
  try {
    const response = await fetch(statusUrl, {
      cache: 'no-store',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
    })
    if (!response.ok) {
      throw new Error(`Device status returned ${response.status}`)
    }

    const nextTrack = getNowPlaying(await response.json())
    if (nextTrack) {
      const hasChanged =
        !currentTrack || currentTrack.signature !== nextTrack.signature
      // If this copy ends before the device reports its next song, keep it
      // queued instead of replaying the same stream from the beginning.
      if (!(waitingForNextTrack && !hasChanged)) {
        await loadTrack(nextTrack, userStarted && hasChanged)
      }
    } else if (!currentTrack) {
      setStatus('Waiting for the retail device to start a Navidrome song')
    }
  } catch (error) {
    setStatus(`Unable to follow retail device: ${error.message}`)
  } finally {
    syncInFlight = false
    scheduleSync(
      waitingForNextTrack ? NEXT_TRACK_INTERVAL_MS : POLL_INTERVAL_MS,
    )
  }
}

const animate = (now) => {
  if (analyser && frequencyData && !audio.paused) {
    analyser.getByteFrequencyData(frequencyData)
    const metrics = calculateSpectrumMetrics(frequencyData)
    bassBaseline = bassBaseline * 0.94 + metrics.bass * 0.06
    const beat =
      metrics.bass > Math.max(0.12, bassBaseline * 1.35) &&
      now - lastBeatAt > 190
    if (beat) {
      lastBeatAt = now
    }
    resonance.setAudio(metrics, beat)
  } else {
    resonance.setAudio(
      { bass: 0, centroid: resonance.centroid, energy: 0 },
      false,
    )
  }

  resonance.frame()
  animationFrame = window.requestAnimationFrame(animate)
}

const togglePlayback = async () => {
  if (!device) {
    setStatus('No retail device was supplied in the player URL')
    return
  }

  button.disabled = true
  try {
    await ensureAudioGraph()
    userStarted = true

    if (!currentTrack) {
      await syncDevice()
    }

    if (!audio.src) {
      setStatus('Waiting for the retail device to start a Navidrome song')
      return
    }

    if (audio.paused) {
      waitingForNextTrack = false
      await audio.play()
      resonance.setActive(true)
      setPlayingState(true)
    } else {
      userStarted = false
      audio.pause()
      setPlayingState(false)
      setStatus('Retail device audio paused')
    }
  } catch (error) {
    setPlayingState(false)
    setStatus(`Playback could not start: ${error.message}`)
  } finally {
    button.disabled = false
  }
}

button.addEventListener('click', togglePlayback)

audio.addEventListener('ended', () => {
  waitingForNextTrack = true
  setPlayingState(false)
  setStatus('Waiting for the retail device to queue its next song')
  scheduleSync(0)
})

audio.addEventListener('playing', () => setPlayingState(true))
audio.addEventListener('pause', () => {
  if (!audio.ended) {
    setPlayingState(false)
  }
})

audio.addEventListener('error', () => {
  waitingForNextTrack = true
  setPlayingState(false)
  setStatus(
    'The Navidrome song could not be loaded; waiting for the next device update',
  )
  scheduleSync(NEXT_TRACK_INTERVAL_MS)
})

window.addEventListener('beforeunload', () => {
  window.clearTimeout(pollTimer)
  window.cancelAnimationFrame(animationFrame)
})

if (!device) {
  setStatus('Use /app/player/DEVICE_NAME to open this player')
  button.setAttribute('aria-label', 'Retail device missing')
} else {
  setStatus(`Loading retail device ${device}`)
  syncDevice()
}

animationFrame = window.requestAnimationFrame(animate)
