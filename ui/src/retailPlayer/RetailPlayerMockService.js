const clamp = (value, min, max) => Math.min(Math.max(value, min), max)

const parseNowPlaying = (value) => {
  if (typeof value !== 'string') {
    return { label: '', artist: '' }
  }
  const [label = '', artist = ''] = value.split('|').map((part) => part.trim())
  return { label, artist }
}

const scheduleLabelFromNowPlaying = (value) => parseNowPlaying(value).label
const scheduleArtistFromNowPlaying = (value) => parseNowPlaying(value).artist

const initialDevices = {
  'thompson-chicago-lobby': {
    id: 'thompson-chicago-lobby',
    name: 'ThompsonChicago_Lobby',
    channel: 'Lobby Channel',
    channelList: 'Chicago Rotation',
    organization: 'Thompson Hotels',
    isConnected: true,
    hasSignal: true,
    isMuted: true,
    schedules: [
      {
        key: 'early',
        label: 'ThompsonChicago_LobbyEarly',
        artist: 'Dog Trainer',
        isActive: true,
      },
      {
        key: 'late',
        label: 'ThompsonChicago_LobbyLate',
        artist: 'Late Night Riders',
      },
      {
        key: 'mid',
        label: 'ThompsonChicago_LobbyMid',
        artist: 'Midday Parade',
      },
    ],
    nowPlaying: 'The Kids | Dog Trainer',
    volume: 75,
    currentTrackIndex: 0,
    tracks: [
      {
        id: 'kids-dog-trainer',
        title: 'The Kids',
        artist: 'Dog Trainer',
        album: 'Lobby Rotation Vol. 1',
        artwork: null,
      },
      {
        id: 'evening-hush',
        title: 'Evening Hush',
        artist: 'Late Night Riders',
        album: 'Chicago After Hours',
        artwork: null,
      },
      {
        id: 'midday-march',
        title: 'Midday March',
        artist: 'Midday Parade',
        album: 'City Strolls',
        artwork: null,
      },
    ],
  },
  'thompson-chicago-rooftop': {
    id: 'thompson-chicago-rooftop',
    name: 'ThompsonChicago_Rooftop',
    channel: 'Rooftop Beats',
    channelList: 'Evening Mix',
    organization: 'Thompson Hotels',
    isConnected: true,
    hasSignal: true,
    isMuted: false,
    schedules: [
      {
        key: 'early',
        label: 'ThompsonChicago_RooftopEarly',
        artist: 'Sunrise Syndicate',
      },
      {
        key: 'late',
        label: 'ThompsonChicago_RooftopLate',
        artist: 'Moon District',
        isActive: true,
      },
      {
        key: 'mid',
        label: 'ThompsonChicago_RooftopMid',
        artist: 'Skyline Ensemble',
      },
    ],
    nowPlaying: 'Skyline Drift | Moon District',
    volume: 62,
    currentTrackIndex: 1,
    tracks: [
      {
        id: 'sunrise-sessions',
        title: 'Sunrise Sessions',
        artist: 'Sunrise Syndicate',
        album: 'Rooftop Dawn',
        artwork: null,
      },
      {
        id: 'skyline-drift',
        title: 'Skyline Drift',
        artist: 'Moon District',
        album: 'Moonlit Mixes',
        artwork: null,
      },
      {
        id: 'night-spark',
        title: 'Night Spark',
        artist: 'Skyline Ensemble',
        album: 'City Lights',
        artwork: null,
      },
    ],
  },
  'thompson-miami-pool': {
    id: 'thompson-miami-pool',
    name: 'ThompsonMiami_Pool',
    channel: 'Poolside Chill',
    channelList: 'Daytime Flow',
    organization: 'Thompson Resorts',
    isConnected: true,
    hasSignal: true,
    isMuted: false,
    schedules: [
      {
        key: 'early',
        label: 'ThompsonMiami_PoolEarly',
        artist: 'Sunrunners',
        isActive: true,
      },
      {
        key: 'late',
        label: 'ThompsonMiami_PoolLate',
        artist: 'Twilight Current',
      },
      {
        key: 'mid',
        label: 'ThompsonMiami_PoolMid',
        artist: 'Harbor Crew',
      },
    ],
    nowPlaying: 'Sea Breeze | Sunrunners',
    volume: 68,
    currentTrackIndex: 0,
    tracks: [
      {
        id: 'sea-breeze',
        title: 'Sea Breeze',
        artist: 'Sunrunners',
        album: 'Poolside Flow',
        artwork: null,
      },
      {
        id: 'twilight-currents',
        title: 'Twilight Currents',
        artist: 'Twilight Current',
        album: 'Evening Reflections',
        artwork: null,
      },
      {
        id: 'harbor-glide',
        title: 'Harbor Glide',
        artist: 'Harbor Crew',
        album: 'Beachline',
        artwork: null,
      },
    ],
  },
  'thompson-denver-lounge': {
    id: 'thompson-denver-lounge',
    name: 'ThompsonDenver_Lounge',
    channel: 'Lounge Sessions',
    channelList: 'Mountain Nights',
    organization: 'Thompson Collective',
    isConnected: false,
    hasSignal: false,
    isMuted: true,
    schedules: [
      {
        key: 'early',
        label: 'ThompsonDenver_LoungeEarly',
        artist: 'Morning Summit',
      },
      {
        key: 'late',
        label: 'ThompsonDenver_LoungeLate',
        artist: 'Alpine Echo',
        isActive: true,
      },
      {
        key: 'mid',
        label: 'ThompsonDenver_LoungeMid',
        artist: 'Denver Collective',
      },
    ],
    nowPlaying: 'Quiet Hours | Alpine Echo',
    volume: 40,
    currentTrackIndex: 1,
    tracks: [
      {
        id: 'summit-glow',
        title: 'Summit Glow',
        artist: 'Morning Summit',
        album: 'Mountain Dawn',
        artwork: null,
      },
      {
        id: 'quiet-hours',
        title: 'Quiet Hours',
        artist: 'Alpine Echo',
        album: 'Twilight Peak',
        artwork: null,
      },
      {
        id: 'lounge-lines',
        title: 'Lounge Lines',
        artist: 'Denver Collective',
        album: 'City Mountains',
        artwork: null,
      },
    ],
  },
}

const deepClone = (value) => JSON.parse(JSON.stringify(value))

class RetailPlayerMockService {
  constructor() {
    this.devices = deepClone(initialDevices)
  }

  listDevices() {
    return Object.values(this.devices).map((device) => ({
      id: device.id,
      name: device.name,
      channel: device.channel,
      channelList: device.channelList,
      organization: device.organization,
    }))
  }

  getDevice(deviceId) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }
    return deepClone(device)
  }

  setMute(deviceId, muted) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }
    device.isMuted = Boolean(muted)
    return this.getDevice(deviceId)
  }

  setVolume(deviceId, volume) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }
    device.volume = clamp(Math.round(volume), 0, 100)
    return this.getDevice(deviceId)
  }

  setActiveChannel(deviceId, channelKey) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }

    let nextNowPlaying = device.nowPlaying
    device.schedules = device.schedules.map((schedule) => {
      const isActive = schedule.key === channelKey
      if (isActive) {
        nextNowPlaying = `${schedule.label} | ${schedule.artist}`
      }
      return { ...schedule, isActive }
    })

    device.nowPlaying = nextNowPlaying
    device.currentTrackIndex = 0
    if (Array.isArray(device.tracks) && device.tracks.length > 0) {
      device.tracks = device.tracks.map((track, index) => {
        if (index === 0) {
          return {
            ...track,
            title: scheduleLabelFromNowPlaying(nextNowPlaying),
            artist: scheduleArtistFromNowPlaying(nextNowPlaying),
          }
        }
        return track
      })
    }
    return this.getDevice(deviceId)
  }

  getArtwork(deviceId) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }
    return null
  }

  setCurrentTrack(deviceId, trackIndex) {
    const device = this.devices[deviceId]
    if (!device || !Array.isArray(device.tracks) || device.tracks.length === 0) {
      return null
    }

    const total = device.tracks.length
    const normalizedIndex = ((trackIndex % total) + total) % total
    device.currentTrackIndex = normalizedIndex
    const track = device.tracks[normalizedIndex]
    device.nowPlaying = `${track.title} | ${track.artist}`
    return this.getDevice(deviceId)
  }

  skipTrack(deviceId) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }

    const nextIndex =
      typeof device.currentTrackIndex === 'number' ? device.currentTrackIndex + 1 : 0
    return this.setCurrentTrack(deviceId, nextIndex)
  }

  getCurrentTrack(deviceId) {
    const device = this.devices[deviceId]
    if (!device || !Array.isArray(device.tracks) || device.tracks.length === 0) {
      return null
    }

    const index =
      typeof device.currentTrackIndex === 'number' ? device.currentTrackIndex : 0
    return deepClone(device.tracks[index % device.tracks.length])
  }
}

const retailPlayerMockService = new RetailPlayerMockService()

export default retailPlayerMockService
