export const retailDevices = [
  {
    id: 'thompson-chicago-lobby',
    name: 'ThompsonChicago_Lobby',
    channel: 'Lobby Channel',
    channelList: 'Chicago Rotation',
    organization: 'Thompson Hotels',
  },
  {
    id: 'thompson-chicago-rooftop',
    name: 'ThompsonChicago_Rooftop',
    channel: 'Rooftop Beats',
    channelList: 'Evening Mix',
    organization: 'Thompson Hotels',
  },
  {
    id: 'thompson-miami-pool',
    name: 'ThompsonMiami_Pool',
    channel: 'Poolside Chill',
    channelList: 'Daytime Flow',
    organization: 'Thompson Resorts',
  },
  {
    id: 'thompson-denver-lounge',
    name: 'ThompsonDenver_Lounge',
    channel: 'Lounge Sessions',
    channelList: 'Mountain Nights',
    organization: 'Thompson Collective',
  },
]

export const retailDeviceDetails = {
  'thompson-chicago-lobby': {
    id: 'thompson-chicago-lobby',
    name: 'ThompsonChicago_Lobby',
    isConnected: true,
    time: '16:40',
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
    isConnected: true,
    time: '18:20',
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
    isConnected: true,
    time: '14:05',
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
    isConnected: false,
    time: '10:12',
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
