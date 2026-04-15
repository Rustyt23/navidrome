import { describe, it, expect } from 'vitest'
import {
  buildDuplicateInfo,
  buildDuplicateTrackIdsByPlaylist,
} from './playlistComparison'

describe('buildDuplicateInfo', () => {
  it('returns unique duplicates sorted by title', () => {
    const leftTracks = [
      { id: 'lt2', mediaFileId: 'm2', title: 'Zebra', artist: 'Two' },
      { id: 'lt1', mediaFileId: 'm1', title: 'Alpha', artist: 'One' },
      { id: 'lt1-dup', mediaFileId: 'm1', title: 'Alpha', artist: 'One' },
    ]
    const rightTracks = [
      { id: 'rt3', mediaFileId: 'm3', title: 'Other' },
      { id: 'rt1', mediaFileId: 'm1', title: 'Alpha' },
      { id: 'rt2', mediaFileId: 'm2', title: 'Zebra' },
    ]

    expect(buildDuplicateInfo(leftTracks, rightTracks)).toEqual([
      { mediaFileId: 'm1', title: 'Alpha', artist: 'One' },
      { mediaFileId: 'm2', title: 'Zebra', artist: 'Two' },
    ])
  })

  it('ignores tracks without mediaFileId', () => {
    const leftTracks = [
      { title: 'Unknown' },
      { mediaFileId: '', title: 'Blank' },
      { mediaFileId: null, title: 'Null' },
    ]
    const rightTracks = [{ mediaFileId: 'm1', title: 'Exists' }]

    expect(buildDuplicateInfo(leftTracks, rightTracks)).toEqual([])
  })
})

describe('buildDuplicateTrackIdsByPlaylist', () => {
  it('returns playlist-track IDs to delete per selected playlist', () => {
    const leftTracks = [
      { id: 'lt1', mediaFileId: 'm1', title: 'Alpha' },
      { id: 'lt2', mediaFileId: 'm2', title: 'Bravo' },
      { id: 'lt3', mediaFileId: 'm1', title: 'Alpha duplicate in left' },
    ]
    const rightTracks = [
      { id: 'rt1', mediaFileId: 'm1', title: 'Alpha' },
      { id: 'rt4', mediaFileId: 'm4', title: 'Delta' },
    ]

    expect(buildDuplicateTrackIdsByPlaylist(leftTracks, rightTracks)).toEqual({
      left: ['lt1', 'lt3'],
      right: ['rt1'],
    })
  })
})
