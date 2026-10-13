import { Component, type ReactNode } from 'react'
import { Link, Route, Routes, useLocation } from 'react-router-dom'
import { NavBar } from './components/NavBar'
import { useApp } from './context/AppContext'
import { AppProvider } from './context/AppProvider'
import type { RuntimeConfig } from './lib/config'
import { releaseUrl } from './lib/format'
import { Landing } from './pages/Landing'
import { Note } from './pages/Note'
import { NoteEditor } from './pages/NoteEditor'

function NotFound() {
  return (
    <div className="page">
      <p className="center-note">Page not found.</p>
      <Link to="/">← Back to feed</Link>
    </div>
  )
}

/**
 * Catches a render error in any page, so the reader gets a message and the navbar rather than a
 * blank tab. The error belongs to the path it happened on, so following a link out clears it.
 */
interface BoundaryProps {
  pathname: string
  children: ReactNode
}

class PageErrorBoundary extends Component<BoundaryProps, { failed: boolean; pathname: string }> {
  state = { failed: false, pathname: this.props.pathname }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  static getDerivedStateFromProps(props: BoundaryProps, state: { pathname: string }) {
    return props.pathname === state.pathname ? null : { failed: false, pathname: props.pathname }
  }

  componentDidCatch(error: unknown) {
    console.error(error)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className="page">
        <p role="alert" className="center-note">
          Something went wrong showing this page.
        </p>
        <Link to="/">← Back to feed</Link>
      </div>
    )
  }
}

function Footer() {
  const { config } = useApp()
  if (!config.githubRepo) return null
  return (
    <footer className="site-footer text-muted">
      <a href={`https://github.com/${config.githubRepo}`}>GitHub</a>
      {config.version && <a href={releaseUrl(config.githubRepo, config.version)}>{config.version}</a>}
    </footer>
  )
}

function App({ config }: { config: RuntimeConfig }) {
  const { pathname } = useLocation()
  return (
    <AppProvider config={config}>
      <NavBar />
      <PageErrorBoundary pathname={pathname}>
        <Routes>
          <Route path="/" element={<Landing />} />
          {/* "new" is the editor rather than a note - React Router ranks the literal above the
              wildcard beside it, and no note id can be "new" (ids are generated). */}
          <Route path="/note/new" element={<NoteEditor />} />
          <Route path="/note/:id" element={<Note />} />
          <Route path="/note/:id/edit" element={<NoteEditor />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </PageErrorBoundary>
      <Footer />
    </AppProvider>
  )
}

export default App
