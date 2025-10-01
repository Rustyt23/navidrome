import React from 'react'
import { Route } from 'react-router-dom'
import Personal from './personal/Personal'
import DiscoveryBrowser from './discovery/DiscoveryBrowser'

const routes = [
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
  <Route
    exact
    path="/discovery"
    render={() => <DiscoveryBrowser />}
    key={'discovery'}
  />,
]

export default routes
