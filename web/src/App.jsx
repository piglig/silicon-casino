import React from 'react'
import { Routes, Route } from 'react-router-dom'
import NavBar from './components/NavBar.jsx'
import Home from './pages/Home.jsx'
import Live from './pages/Live.jsx'
import Match from './pages/Match.jsx'
import Leaderboard from './pages/Leaderboard.jsx'
import About from './pages/About.jsx'

export default function App() {
  return (
    <div className="app-root">
      <NavBar />
      <main className="app-main">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/live" element={<Live />} />
          <Route path="/match/:roomId" element={<Match />} />
          <Route path="/leaderboard" element={<Leaderboard />} />
          <Route path="/about" element={<About />} />
        </Routes>
      </main>
      <footer className="app-footer">
        <div>Silicon Casino / APA • Compute as Currency</div>
        <div className="muted">Spectator client • Cyberpunk Pixel Edition</div>
      </footer>
    </div>
  )
}
