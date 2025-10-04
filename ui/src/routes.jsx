import React from 'react'
import { Route } from 'react-router-dom'
import Personal from './personal/Personal'
import DiscoveryShow from './discovery/DiscoveryShow'

const routes = [
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
  <Route exact path="/discovery/:id" render={() => <DiscoveryShow />} key={'discovery-show'} />,
]

export default routes
