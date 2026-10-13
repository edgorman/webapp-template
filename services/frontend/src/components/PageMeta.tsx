import { useApp } from '../context/AppContext'

/**
 * A page's `<title>`, description and canonical link. React 19 hoists all three into `<head>`
 * wherever they render, and swaps them as the route changes.
 */
export function PageMeta({ title, description, path }: { title?: string; description?: string; path: string }) {
  const { config } = useApp()
  return (
    <>
      <title>{title ? `${title} · ${config.siteName}` : config.siteName}</title>
      <meta name="description" content={description ?? config.siteName} />
      <link rel="canonical" href={`${config.siteUrl}${path}`} />
    </>
  )
}
