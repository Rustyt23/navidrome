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

  // The raw error text is not shown to the client. It is ffmpeg's own output,
  // which carries hundreds of progress lines around a few words of message, and
  // it stays on screen until the next run starts. It is still recorded in the
  // server log, which is where it is read.
  it('does not put the raw job error on the page', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{
          running: false,
          rejected: 1,
          error: 'analyzing loudness: signal: killed: Input #0, mp3, from ...',
        }}
      />,
    )
    expect(screen.getAllByRole('alert')).toHaveLength(1)
    expect(screen.getByRole('alert')).toHaveTextContent('1 rejected')
    expect(screen.queryByText(/signal: killed/)).not.toBeInTheDocument()
  })
})
