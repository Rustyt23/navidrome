import React from 'react'
import { Route } from 'react-router-dom'
import Personal from './personal/Personal'
import RetailPlayerDashboard from './retailPlayer/RetailPlayerDashboard'

const routes = [
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
  <Route
    path="/retailplayer"
    render={() => <RetailPlayerDashboard />}
    key={'retailplayer'}
  />,
]

export default routes
