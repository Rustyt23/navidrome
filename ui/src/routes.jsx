import React from 'react'
import { Redirect, Route } from 'react-router-dom'
import Personal from './personal/Personal'
import RetailPlayerDashboard from './retailPlayer/RetailPlayerDashboard'
import RetailPlayerDevicesList from './retailPlayer/RetailPlayerDevicesList'
import RetailPlayerFolderList from './retailPlayer/RetailPlayerFolderList'

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
    path="/retailplayer/folder"
    render={() => <RetailPlayerFolderList />}
    key={'retailplayer-folder'}
  />,
  <Route
    exact
    path="/retailplayer/devices"
    render={() => <RetailPlayerDevicesList />}
    key={'retailplayer-devices'}
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
]

export default routes
