// Why a song could not be processed at all, in words.
//
// A hard failure is an ffmpeg error, and ffmpeg errors are written for whoever
// wrote ffmpeg. "Could not write header (incorrect codec parameters ?): Invalid
// argument" tells the person looking at this page nothing except that something
// broke, and nothing at all about whether it is fixable or which of six hundred
// songs to go and look at.
//
// The patterns below are matched loosely and always fall back to the raw text,
// so an error shape this file has not seen shows what the engine actually said
// rather than a blank cell or a wrong guess.

// The art case, which is by far the most common and the most fixable. ffmpeg
// reports it several different ways depending on which stage gives up first.
//
// The codec-parameters line is pinned to a video stream on purpose: the same
// sentence about an audio stream means something else entirely.
const ART_BROKEN =
  /dimensions not set|Invalid (PNG|JPEG) signature|Could not write header|Could not find codec parameters for stream \d+ \(Video|unspecified size/i

// Nothing playable in the file - usually a truncated or zero-length download.
//
// "Invalid data found when processing input" is deliberately absent. ffmpeg
// prints it for any decode failure, including a cover picture it cannot read,
// so matching on it reported ten perfectly good songs as having no audio. A
// phrase that broad cannot carry a diagnosis.
const NO_AUDIO =
  /Failed to find two consecutive MPEG audio frames|no audio stream found|does not contain any stream|moov atom not found|Output file is empty/i

const UNREADABLE = /no such file|permission denied|Is a directory|I\/O error/i
const TOO_LONG = /context deadline exceeded|signal: killed|timed out/i
const NO_SPACE = /no space left|disk quota exceeded|read-only file system/i

// explainFailure turns one engine error into a short headline and a sentence.
export const explainFailure = (error) => {
  const text = String(error || '')
  if (!text) {
    return {
      headline: 'Could not be processed',
      detail: 'The engine gave up on this song but recorded no reason.',
    }
  }

  if (NO_AUDIO.test(text)) {
    return {
      headline: 'No readable audio in the file',
      detail:
        'The file has no playable audio in it - normally a download that was truncated or never finished. ' +
        'Nothing here can repair that; the song needs fetching again from its source.',
    }
  }
  if (ART_BROKEN.test(text)) {
    return {
      headline: 'Cover art could not be copied',
      detail:
        'The embedded picture claims one format and contains another - usually an ID3 tag saying PNG over ' +
        'JPEG data. The engine retries with the format the bytes actually are, so a song still failing here ' +
        'has artwork it could not read either way. Your original file is untouched.',
    }
  }
  if (NO_SPACE.test(text)) {
    return {
      headline: 'Could not write the new file',
      detail:
        'There was nowhere to put the result - the disk is full, over quota, or mounted read-only. ' +
        'Your original file is untouched.',
    }
  }
  if (UNREADABLE.test(text)) {
    return {
      headline: 'The file could not be opened',
      detail:
        'The song is not where the library says it is, or cannot be read. Check the path and its permissions.',
    }
  }
  if (TOO_LONG.test(text)) {
    return {
      headline: 'Took too long and was stopped',
      detail:
        'Processing ran past its time limit and was cut short. Your original file is untouched.',
    }
  }
  return {
    headline: 'Could not be processed',
    detail: `${text.trim()}`,
  }
}
