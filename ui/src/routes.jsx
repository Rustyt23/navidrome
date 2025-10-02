import React from 'react'
import { Route } from 'react-router-dom'
import Personal from './personal/Personal'
import DiscoveryFoldersPage from './discovery_folder/DiscoveryFoldersPage'

const routes = [
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
  <Route
    exact
    path="/discovery"
    render={() => <DiscoveryFoldersPage />}
    key={'discovery'}
  />,
]

export default routes
