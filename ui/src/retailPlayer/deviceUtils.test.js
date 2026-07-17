import { describe, expect, it } from 'vitest'
import { isHiddenRetailPlayerChannel } from './deviceUtils'

describe('isHiddenRetailPlayerChannel', () => {
  it.each([
    { label: 'Empty' },
    { name: ' empty ' },
    { metadata: { channelName: 'EMPTY' } },
    { metadata: { channel_name: 'Empty' } },
    { raw: { name: 'Empty' } },
    { raw: { channelName: 'Empty' } },
  ])(
    'hides the reserved Empty channel from every supported payload shape',
    (channel) => {
      expect(isHiddenRetailPlayerChannel(channel)).toBe(true)
    },
  )

  it.each([
    null,
    {},
    { label: 'Empty Room' },
    { name: 'Not Empty' },
    { metadata: { channelName: 'Storage Test' } },
  ])('keeps regular channels visible', (channel) => {
    expect(isHiddenRetailPlayerChannel(channel)).toBe(false)
  })
})
