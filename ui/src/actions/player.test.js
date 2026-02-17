import { describe, it, expect } from 'vitest'
import { filterSongs, playTracks } from './player'
import { playerReducer } from '../reducers/playerReducer'

describe('player queue ordering', () => {
  it('preserves explicit id order for numeric-like playlist track ids', () => {
    const songs = {
      1: { id: 's1', title: 'Song 1' },
      2: { id: 's2', title: 'Song 2' },
      10: { id: 's10', title: 'Song 10' },
    }

    const ordered = filterSongs(songs, ['10', '2', '1'])

    expect(Object.keys(ordered)).toEqual(['_10', '_2', '_1'])
  })

  it('keeps selected track when reducer consumes normalized queue keys', () => {
    const songs = {
      1: { id: 's1', title: 'Song 1' },
      2: { id: 's2', title: 'Song 2' },
      10: { id: 's10', title: 'Song 10' },
    }

    const action = playTracks(songs, ['10', '2', '1'], '2')
    const nextState = playerReducer(undefined, action)

    expect(nextState.playIndex).toBe(1)
    expect(nextState.queue.map((item) => item.name)).toEqual([
      'Song 10',
      'Song 2',
      'Song 1',
    ])
  })
})
