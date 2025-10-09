import React from 'react'
import { useMediaQuery } from '@material-ui/core'
import { useTranslate } from 'react-admin'

const resolvePluralText = (value, args) => {
  if (typeof value !== 'string' || !value.includes('||||')) {
    return value
  }

  const [singular = '', plural = ''] = value.split('||||').map((part) => part.trim())
  const smartCount =
    (args && (args.smart_count ?? args.smartCount ?? args.count)) !== undefined
      ? args.smart_count ?? args.smartCount ?? args.count
      : 1

  if (smartCount === 1) {
    return singular || plural || value
  }

  return plural || singular || value
}

export const Title = ({ subTitle, args }) => {
  const translate = useTranslate()
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const translated = translate(subTitle, { ...args, _: subTitle })
  const text = resolvePluralText(translated, args)
  const brand = <span className="brand-title">MusicMatters</span>

  if (isDesktop) {
    return <span className="title-line">{brand}{text ? ` - ${text}` : ''}</span>
  }
  return <span className="title-line">{text ? text : brand}</span>
}
