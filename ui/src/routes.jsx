import React from 'react'
import { Redirect, Route } from 'react-router-dom'
import Personal from './personal/Personal'
import RetailPlayerDashboard from './retailPlayer/RetailPlayerDashboard'
import RetailPlayerDeviceManagement from './retailPlayer/RetailPlayerDeviceManagement'
import PublicRetailPlayer from './retailPlayer/PublicRetailPlayer'

const routes = [
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
  <Route
    exact
    path="/retailplayer"
    render={() => <Redirect to="/retailplayer/devices" />}
    key={'retailplayer-redirect'}
  />,
  <Route
    exact
    path="/retailplayer/devices"
    render={() => <RetailPlayerDeviceManagement />}
    key={'retailplayer-devices'}
  />,
  <Route
    exact
    path="/retail-player/devices"
    render={() => <RetailPlayerDeviceManagement />}
    key={'retailplayer-devices-settings'}
  />,
  <Route
    exact
    path="/retailplayer/:deviceSlug"
    render={() => <RetailPlayerDashboard />}
    key={'retailplayer-device'}
  />,
  <Route
    exact
    path="/musicmatters/:deviceSlug"
    render={() => <RetailPlayerDashboard />}
    key={'musicmatters-device'}
  />,
  <Route
    exact
    path="/player/:deviceSlug"
    render={() => <PublicRetailPlayer />}
    key={'public-retailplayer-device'}
  />,
]

export default routes
