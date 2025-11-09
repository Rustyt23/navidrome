import { describe, it, expect } from 'vitest'
import { renderHook, act } from '@testing-library/react-hooks'
import { smartSort, useSmartSort, SortDirection, SortType } from './sortUtils'

describe('smartSort', () => {
  it('sorts strings ignoring case and leading spaces without dropping articles', () => {
    const data = [
      { name: '  Zebra' },
      { name: 'apple' },
      { name: 'The Avayas' },
      { name: 'beatles' },
    ]

    const result = smartSort(data, {
      accessor: (item) => item.name,
      type: SortType.STRING,
    })

    expect(result.map((item) => item.name)).toEqual([
      'apple',
      'beatles',
      'The Avayas',
      '  Zebra',
    ])
  })

  it('sorts numeric values numerically even when stored as strings', () => {
    const data = [
      { value: '10' },
      { value: 2 },
      { value: '2' },
      { value: 3 },
    ]

    const result = smartSort(data, {
      accessor: (item) => item.value,
      type: SortType.NUMBER,
    })

    expect(result.map((item) => Number(item.value))).toEqual([2, 2, 3, 10])
  })

  it('sorts dates chronologically regardless of format', () => {
    const data = [
      { played: '2024-02-01T00:00:00Z' },
      { played: new Date('2023-12-01T12:00:00Z') },
      { played: '2024-01-15' },
    ]

    const result = smartSort(data, {
      accessor: (item) => item.played,
      type: SortType.DATE,
    })

    expect(result.map((item) => item.played)).toEqual([
      new Date('2023-12-01T12:00:00Z'),
      '2024-01-15',
      '2024-02-01T00:00:00Z',
    ])
  })

  it('maintains stable ordering for identical values', () => {
    const data = [
      { id: 1, name: 'Alpha' },
      { id: 2, name: 'alpha' },
      { id: 3, name: 'ALPHA' },
    ]

    const result = smartSort(data, {
      accessor: (item) => item.name,
      type: SortType.STRING,
    })

    expect(result.map((item) => item.id)).toEqual([1, 2, 3])
  })
})

describe('useSmartSort', () => {
  it('sorts data and toggles direction on request', () => {
    const data = [
      { id: 1, name: 'Bravo', plays: 5 },
      { id: 2, name: 'Alpha', plays: 12 },
      { id: 3, name: 'Charlie', plays: 2 },
    ]

    const { result } = renderHook(() =>
      useSmartSort(data, {
        initialKey: 'name',
        columns: { name: { type: SortType.STRING }, plays: { type: SortType.NUMBER } },
      }),
    )

    expect(result.current.sortedData.map((item) => item.name)).toEqual([
      'Alpha',
      'Bravo',
      'Charlie',
    ])

    act(() => {
      result.current.requestSort('plays')
    })

    expect(result.current.sortedData.map((item) => item.plays)).toEqual([2, 5, 12])
    expect(result.current.getSortDirection('plays')).toBe(SortDirection.ASC)

    act(() => {
      result.current.requestSort('plays')
    })

    expect(result.current.sortedData.map((item) => item.plays)).toEqual([12, 5, 2])
    expect(result.current.getSortDirection('plays')).toBe(SortDirection.DESC)
  })

  it('returns the original array when no sort key is active', () => {
    const items = [{ id: 1 }, { id: 2 }]

    const { result } = renderHook(() => useSmartSort(items))

    expect(result.current.sortedData).toBe(items)
  })
})
