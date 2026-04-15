import { describe, it, expect } from 'vitest'
import { buildDuplicateInfo } from './playlistComparison'

describe('buildDuplicateInfo', () => {
  it('returns unique duplicates sorted by title', () => {
    const leftTracks = [
      { mediaFileId: 'm2', title: 'Zebra', artist: 'Two' },
      { mediaFileId: 'm1', title: 'Alpha', artist: 'One' },
      { mediaFileId: 'm1', title: 'Alpha', artist: 'One' },
    ]
    const rightTracks = [
      { mediaFileId: 'm3', title: 'Other' },
      { mediaFileId: 'm1', title: 'Alpha' },
      { mediaFileId: 'm2', title: 'Zebra' },
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
