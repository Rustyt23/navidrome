import React from 'react'
import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SongInfo } from './SongInfo'

const useRecordContext = vi.fn()

vi.mock('react-admin', () => ({
  BooleanField: ({ source }) => <span>boolean-{source}</span>,
  DateField: ({ source }) => <span>date-{source}</span>,
  TextField: ({ source }) => <span>text-{source}</span>,
  NumberField: ({ source }) => <span>number-{source}</span>,
  FunctionField: () => <span>function-field</span>,
  useTranslate: () => (key) => key,
  useRecordContext: (...args) => useRecordContext(...args),
}))

vi.mock('./index', () => ({
  ArtistLinkField: () => <span>artist</span>,
  BitrateField: () => <span>bitrate</span>,
  ParticipantsInfo: () => null,
  PathField: () => <span>path</span>,
  SizeField: () => <span>size</span>,
}))

vi.mock('./MultiLineTextField', () => ({
  MultiLineTextField: () => <span>multi</span>,
}))

vi.mock('../song/AlbumLinkField', () => ({
  AlbumLinkField: () => <span>album</span>,
}))

vi.mock('../config', () => ({
  default: { enableReplayGain: false },
}))

describe('SongInfo', () => {
  beforeEach(() => {
    useRecordContext.mockReset()
  })

  it('renders without crashing when participants data is missing', () => {
    useRecordContext.mockReturnValue({
      id: '1',
      genres: [{ name: 'Rock' }],
      tags: {},
      playCount: 0,
    })

    const { getByText } = render(<SongInfo />)

    expect(getByText(/resources\.song\.fields\.path/)).toBeInTheDocument()
  })
})
