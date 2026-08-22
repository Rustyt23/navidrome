import { describe, it, expect } from 'vitest'
import { explainFailure } from './failure'

// Every string below is one ffmpeg actually produced against the twelve songs
// that failed on the production library, captured verbatim. Invented strings
// would only prove the patterns match themselves.
describe('explainFailure', () => {
  it('names the mislabelled cover art', () => {
    const { headline, detail } = explainFailure(
      'applying gain: exit status 234: [mp3 @ 0x1596154c0] dimensions not set\n' +
        '[out#0/mp3 @ 0x159615400] Could not write header (incorrect codec parameters ?): Invalid argument',
    )
    expect(headline).toBe('Cover art could not be copied')
    expect(detail).toContain('claims one format and contains another')
  })

  it('recognises the art failure from the decoder complaint alone', () => {
    expect(
      explainFailure(
        '[png @ 0x14c7060d0] Invalid PNG signature 0xFFD8FFE000104A46.',
      ).headline,
    ).toBe('Cover art could not be copied')
    // The reverse case, when forcing mjpeg on genuine PNG art fails.
    expect(
      explainFailure(
        'Could not find codec parameters for stream 1 (Video: mjpeg): unspecified size',
      ).headline,
    ).toBe('Cover art could not be copied')
  })

  it('names an empty or truncated file', () => {
    // Exactly what ProbeFile now records for a zero-byte mp3, ffprobe's own
    // words included.
    const { headline, detail } = explainFailure(
      'probing "/music/x.mp3": exit status 1: [mp3 @ 0x128e133e0] Failed to find two ' +
        'consecutive MPEG audio frames.; /music/x.mp3: Invalid data found when processing input',
    )
    expect(headline).toBe('No readable audio in the file')
    expect(detail).toContain('fetching again from its source')
  })

  it('does not read a broken picture as missing audio', () => {
    // The regression this whole pattern set exists for. ffmpeg prints "Invalid
    // data found when processing input" when it cannot decode a cover picture,
    // and matching on that phrase reported ten optimised songs as having no
    // audio at all.
    const nullTest =
      'null test: measuring null residual: exit status 69: [png @ 0x1216080b0] Invalid PNG ' +
      'signature 0xFFD8FFE000104A46.\n[in#0/mp3] Could not find codec parameters for stream 1 ' +
      '(Video: png, none): unspecified size\n[dec:png] Error submitting packet to decoder: ' +
      'Invalid data found when processing input'
    expect(explainFailure(nullTest).headline).toBe(
      'Cover art could not be copied',
    )
  })

  it('does not read a broken audio stream as a picture problem', () => {
    // The mirror image: the same sentence about an audio stream is not art.
    expect(
      explainFailure(
        'Could not find codec parameters for stream 0 (Audio: mp3, 0 channels)',
      ).headline,
    ).toBe('Could not be processed')
  })

  it('distinguishes a full disk from a broken song', () => {
    expect(
      explainFailure('write /music/x.tmp: no space left on device').headline,
    ).toBe('Could not write the new file')
    expect(
      explainFailure('open /music/x.mp3: permission denied').headline,
    ).toBe('The file could not be opened')
    expect(explainFailure('signal: killed').headline).toBe(
      'Took too long and was stopped',
    )
  })

  it('shows what the engine said when it recognises nothing', () => {
    const { headline, detail } = explainFailure(
      'something entirely new went wrong',
    )
    expect(headline).toBe('Could not be processed')
    expect(detail).toBe('something entirely new went wrong')
  })

  it('says so rather than going blank when there is no error text', () => {
    expect(explainFailure('').detail).toContain('recorded no reason')
    expect(explainFailure(null).headline).toBe('Could not be processed')
  })
})
