import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { JobProgress } from './JobProgress'

describe('finished LUFS job', () => {
  // A finished job says nothing here.
  //
  // The rejection count used to stay on screen in red once the run ended, and
  // stayed until the next one started: a permanent alarm about work that was
  // already over, beside controls for a job no longer running. It is a property
  // of the library rather than of the run, so the summary panel carries it now
  // and can open the songs it refers to - which the banner never could.
  it('shows nothing once the run is over', () => {
    const { container } = render(
      <JobProgress
        label="Optimising"
        status={{ running: false, rejected: 35, processed: 1678 }}
      />,
    )
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // The raw error text is ffmpeg's own output: hundreds of progress lines
  // around a few words of message. It is kept in the server log, which is where
  // it is read, and never put in front of the client.
  it('does not put the raw job error on the page', () => {
    const { container } = render(
      <JobProgress
        label="Optimising"
        status={{
          running: false,
          rejected: 1,
          error: 'analyzing loudness: signal: killed: Input #0, mp3, from ...',
        }}
      />,
    )
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByText(/signal: killed/)).not.toBeInTheDocument()
  })

  // While the job IS running the progress line still reports itself, including
  // the counters - this only changed what a finished job leaves behind.
  it('still reports progress while the run is going', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{ running: true, processed: 1678, total: 91254 }}
      />,
    )
    expect(screen.getByText(/Optimising 1,678 \/ 91,254/)).toBeInTheDocument()
  })
})
