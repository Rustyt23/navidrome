import { buildDeviceSlug, deviceSlugKey } from './deviceUtils'

const clamp = (value, min, max) => Math.min(Math.max(value, min), max)

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
      apiId: device.id,
      name: device.name,
      slug: buildDeviceSlug(device) || device.name || device.id,
      slugKey: deviceSlugKey(device.name || device.id),
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
    return this.getDevice(deviceId)
  }

  getArtwork(deviceId) {
    const device = this.devices[deviceId]
    if (!device) {
      return null
    }
    return null
  }
}

const retailPlayerMockService = new RetailPlayerMockService()

export default retailPlayerMockService
