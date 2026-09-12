import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { JobProgress } from './JobProgress'

describe('finished LUFS job', () => {
  it('keeps rejected results visible after processing finishes', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{ running: false, rejected: 2 }}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      '2 rejected; working audio unchanged',
    )
  })

  it('reports both record-save errors and rejected candidates', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{
          running: false,
          rejected: 1,
          error: 'Could not save LUFS records',
        }}
      />,
    )
    expect(screen.getAllByRole('alert')).toHaveLength(2)
    expect(screen.getByText('Could not save LUFS records')).toBeInTheDocument()
  })
})
