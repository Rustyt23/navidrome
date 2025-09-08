import { describe, it, expect, beforeEach, vi } from 'vitest'

vi.mock('react-dnd', () => ({
  useDrag: vi.fn(() => [null, () => {}]),
}))

vi.mock('react-admin', async () => {
  const actual = await vi.importActual('react-admin')
  const React = await vi.importActual('react')
  return {
    ...actual,
    PureDatagridRow: React.forwardRef(function PureDatagridRowMock(
      { children, ...props },
      ref,
    ) {
      return (
        <div ref={ref} {...props}>
          {children}
        </div>
      )
    }),
  }
})

// mock Material-UI components used in SongDatagridRow
vi.mock('@material-ui/core', () => ({
  TableRow: ({ children, ...props }) => <div {...props}>{children}</div>,
  TableCell: ({ children, ...props }) => <div {...props}>{children}</div>,
  Typography: ({ children }) => <div>{children}</div>,
  useMediaQuery: () => true,
}))

vi.mock('@material-ui/core/styles', () => ({
  makeStyles: () => () => ({}),
}))

vi.mock('@material-ui/icons/Album', () => ({
  __esModule: true,
  default: () => <span />,
}))

vi.mock('../common', () => ({
  AlbumContextMenu: () => null,
}))

import React from 'react'
import { render } from '@testing-library/react'
import { ListContextProvider } from 'react-admin'
import { SongDatagridRow } from './SongDatagrid'
import { useDrag } from 'react-dnd'

describe('<SongDatagridRow />', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('includes all selected ids when dragging a selected song', () => {
    const record1 = { id: '1', mediaFileId: 'm1', title: 'Song 1' }
    const record2 = { id: '2', mediaFileId: 'm2', title: 'Song 2' }
    const contextValue = {
      data: { '1': record1, '2': record2 },
      ids: ['1', '2'],
      selectedIds: ['1', '2'],
    }
    render(
      <ListContextProvider value={contextValue}>
        <SongDatagridRow record={record1} firstTracksOfDiscs={new Set()}>
          <span />
        </SongDatagridRow>
      </ListContextProvider>,
    )
    const spec = useDrag.mock.calls[1][0]()
    expect(spec.item.ids).toEqual(['m1', 'm2'])
  })

  it('includes only current id when no multi-selection', () => {
    const record1 = { id: '1', mediaFileId: 'm1', title: 'Song 1' }
    const contextValue = {
      data: { '1': record1 },
      ids: ['1'],
      selectedIds: [],
    }
    render(
      <ListContextProvider value={contextValue}>
        <SongDatagridRow record={record1} firstTracksOfDiscs={new Set()}>
          <span />
        </SongDatagridRow>
      </ListContextProvider>,
    )
    const spec = useDrag.mock.calls[1][0]()
    expect(spec.item.ids).toEqual(['m1'])
  })

  it('remains draggable when a song is selected', () => {
    const record1 = { id: '1', mediaFileId: 'm1', title: 'Song 1' }
    const contextValue = {
      data: { '1': record1 },
      ids: ['1'],
      selectedIds: ['1'],
    }
    const { container } = render(
      <ListContextProvider value={contextValue}>
        <SongDatagridRow record={record1} firstTracksOfDiscs={new Set()}>
          <span />
        </SongDatagridRow>
      </ListContextProvider>,
    )
    expect(container.firstChild.getAttribute('draggable')).toBe('true')
  })
})
