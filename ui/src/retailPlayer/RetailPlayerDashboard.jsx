import React, { useMemo } from 'react'
import { Redirect, Route, Switch, useRouteMatch } from 'react-router-dom'
import RetailPlayerLayout from './RetailPlayerLayout'
import DevicesPage from './DevicesPage'
import ChannelsPage from './ChannelsPage'
import ChannelListsPage from './ChannelListsPage'

const RetailPlayerDashboard = () => {
  const { path, url } = useRouteMatch()

  const menuItems = useMemo(
    () => [
      { label: 'Devices', to: `${url}/devices`, exact: true },
      { label: 'Channels', to: `${url}/channels`, exact: true },
      { label: 'Channel Lists', to: `${url}/channel-lists`, exact: true },
    ],
    [url],
  )

  return (
    <RetailPlayerLayout menuItems={menuItems}>
      <Switch>
        <Route exact path={path}>
          <Redirect to={`${url}/devices`} />
        </Route>
        <Route path={`${path}/devices`}>
          <DevicesPage />
        </Route>
        <Route path={`${path}/channels`}>
          <ChannelsPage />
        </Route>
        <Route path={`${path}/channel-lists`}>
          <ChannelListsPage />
        </Route>
        <Redirect to={`${url}/devices`} />
      </Switch>
    </RetailPlayerLayout>
  )
}

export default RetailPlayerDashboard
