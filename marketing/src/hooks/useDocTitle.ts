import { useEffect } from 'react'

const BASE_TITLE = 'Groflo'
const BASE_URL = 'https://groflo.io'
const DEFAULT_DESC = "No more 'Did you get milk?' Penny builds a shared grocery list for your whole household — plus free products and deals from brands."

export function useDocTitle(pageTitle?: string, metaDescription?: string) {
  useEffect(() => {
    document.title = pageTitle ? `${pageTitle} — ${BASE_TITLE}` : `${BASE_TITLE} — The AI Shopping List for Families`

    const desc = metaDescription || DEFAULT_DESC
    const metaTag = document.querySelector('meta[name="description"]')
    if (metaTag) metaTag.setAttribute('content', desc)

    // Set canonical URL
    const path = window.location.pathname
    const canonicalUrl = `${BASE_URL}${path === '/' ? '' : path}`
    let canonicalTag = document.querySelector('link[rel="canonical"]') as HTMLLinkElement | null
    if (!canonicalTag) {
      canonicalTag = document.createElement('link')
      canonicalTag.rel = 'canonical'
      document.head.appendChild(canonicalTag)
    }
    canonicalTag.href = canonicalUrl

    // Set breadcrumb structured data for sub-pages
    const breadcrumbId = 'breadcrumb-jsonld'
    const existingBreadcrumb = document.getElementById(breadcrumbId)
    if (existingBreadcrumb) existingBreadcrumb.remove()

    if (pageTitle && path !== '/') {
      const breadcrumbScript = document.createElement('script')
      breadcrumbScript.id = breadcrumbId
      breadcrumbScript.type = 'application/ld+json'
      breadcrumbScript.textContent = JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'BreadcrumbList',
        'itemListElement': [
          { '@type': 'ListItem', 'position': 1, 'name': 'Home', 'item': BASE_URL },
          { '@type': 'ListItem', 'position': 2, 'name': pageTitle, 'item': canonicalUrl }
        ]
      })
      document.head.appendChild(breadcrumbScript)
    }

    return () => {
      document.title = `${BASE_TITLE} — The AI Shopping List for Families`
      if (metaTag) metaTag.setAttribute('content', DEFAULT_DESC)
      const bc = document.getElementById(breadcrumbId)
      if (bc) bc.remove()
    }
  }, [pageTitle, metaDescription])
}
