import { describe, expect, it } from 'vitest'
import {
  buildRetailStatusUrl,
  calculateSpectrumMetrics,
  getDeviceFromLocation,
  getNavidromeBasePath,
  getNowPlaying,
} from '../../public/player-assets/player-utils'

describe('retail resonance player utilities', () => {
  it('reads a device slug from the clean player path', () => {
    expect(
      getDeviceFromLocation({
        pathname: '/app/player/AlilaMarea_Pool',
        search: '',
      }),
    ).toBe('AlilaMarea_Pool')
    expect(
      getDeviceFromLocation({
        pathname: '/music/app/player/Pool%20Deck',
        search: '',
      }),
    ).toBe('Pool Deck')
  })

  it('keeps Navidrome base paths in status URLs', () => {
    expect(getNavidromeBasePath('/music/app/player/Pool')).toBe('/music')
    expect(buildRetailStatusUrl('/music', 'Pool/Deck')).toBe(
      '/music/api/retailplayer/devices/Pool%2FDeck/status',
    )
  })

  it('selects metadata for the active retail stream', () => {
    const result = getNowPlaying({
      status: {
        activeResource: 'track-2.mp3',
        activeStreamName: 'Pool Schedule',
      },
      streamMetadata: [
        {
          activeResource: 'track-1.mp3',
          metadata: { artist: 'First Artist', title: 'First Song' },
        },
        {
          activeResource: 'track-2.mp3',
          metadata: { artist: 'Bob Marley', title: 'Midnight Ravers' },
        },
      ],
      artwork: {
        mediaFileId: 'media-2',
        streamUrl: '/share/s/signed-token',
      },
    })

    expect(result).toEqual({
      artist: 'Bob Marley',
      signature: 'media-2',
      streamUrl: '/share/s/signed-token',
      title: 'Midnight Ravers',
    })
  })

  it('falls back to artist and title embedded in a stream filename', () => {
    expect(
      getNowPlaying({
        status: { activeStream: '/music/Bob Marley - Midnight Ravers.mp3' },
        artwork: { streamUrl: '/share/s/token' },
      }),
    ).toMatchObject({
      artist: 'Bob Marley',
      title: 'Midnight Ravers',
    })
  })

  it('waits when no signed Navidrome stream is available', () => {
    expect(
      getNowPlaying({
        status: { activeStreamName: 'Midnight Ravers' },
        streamMetadata: [{ metadata: { title: 'Midnight Ravers' } }],
      }),
    ).toBeNull()
  })

  it('calculates normalized energy, bass, and spectral centroid', () => {
    const silence = calculateSpectrumMetrics(new Uint8Array(8))
    expect(silence).toEqual({ bass: 0, centroid: 0, energy: 0 })

    const bassHeavy = calculateSpectrumMetrics(new Uint8Array([255, 0, 0, 0]))
    expect(bassHeavy.bass).toBe(1)
    expect(bassHeavy.energy).toBe(0.5)
    expect(bassHeavy.centroid).toBe(0)

    const trebleHeavy = calculateSpectrumMetrics(new Uint8Array([0, 0, 0, 255]))
    expect(trebleHeavy.centroid).toBe(1)
  })
})
