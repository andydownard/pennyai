import { render } from '@testing-library/react'
import { describe, it, expect, afterEach } from 'vitest'
import { useDocTitle } from '../hooks/useDocTitle'

// Helper component that uses the hook
function TestComponent({ title }: { title?: string }) {
  useDocTitle(title)
  return <div>Test Component</div>
}

describe('useDocTitle hook', () => {
  const DEFAULT_TITLE = 'Thriftly \u2014 The AI Shopping List for Families'

  afterEach(() => {
    // Reset document title after each test
    document.title = DEFAULT_TITLE
  })

  // ===== BASIC BEHAVIOR =====
  describe('basic behavior', () => {
    it('sets document title with page name and base title', () => {
      render(<TestComponent title="For Brands" />)
      expect(document.title).toBe('For Brands \u2014 Thriftly')
    })

    it('sets default title when no page title is provided', () => {
      render(<TestComponent />)
      expect(document.title).toBe(DEFAULT_TITLE)
    })

    it('sets default title when page title is undefined', () => {
      render(<TestComponent title={undefined} />)
      expect(document.title).toBe(DEFAULT_TITLE)
    })
  })

  // ===== SPECIFIC PAGE TITLES =====
  describe('specific page titles', () => {
    it('sets title for Pricing page', () => {
      render(<TestComponent title="Pricing" />)
      expect(document.title).toBe('Pricing \u2014 Thriftly')
    })

    it('sets title for About page', () => {
      render(<TestComponent title="About" />)
      expect(document.title).toBe('About \u2014 Thriftly')
    })

    it('sets title for Privacy Policy page', () => {
      render(<TestComponent title="Privacy Policy" />)
      expect(document.title).toBe('Privacy Policy \u2014 Thriftly')
    })

    it('sets title for Terms of Service page', () => {
      render(<TestComponent title="Terms of Service" />)
      expect(document.title).toBe('Terms of Service \u2014 Thriftly')
    })

    it('sets title for Create Campaign page', () => {
      render(<TestComponent title="Create Campaign" />)
      expect(document.title).toBe('Create Campaign \u2014 Thriftly')
    })

    it('sets title for Page Not Found page', () => {
      render(<TestComponent title="Page Not Found" />)
      expect(document.title).toBe('Page Not Found \u2014 Thriftly')
    })
  })

  // ===== CLEANUP BEHAVIOR =====
  describe('cleanup behavior', () => {
    it('restores default title on unmount', () => {
      const { unmount } = render(<TestComponent title="Pricing" />)
      expect(document.title).toBe('Pricing \u2014 Thriftly')
      unmount()
      expect(document.title).toBe(DEFAULT_TITLE)
    })

    it('restores default title when page-title component unmounts', () => {
      const { unmount } = render(<TestComponent title="About" />)
      expect(document.title).toBe('About \u2014 Thriftly')
      unmount()
      expect(document.title).toBe(DEFAULT_TITLE)
    })

    it('restores default title from default-title component unmount', () => {
      const { unmount } = render(<TestComponent />)
      expect(document.title).toBe(DEFAULT_TITLE)
      unmount()
      expect(document.title).toBe(DEFAULT_TITLE)
    })
  })

  // ===== TITLE FORMAT =====
  describe('title format', () => {
    it('uses em dash separator between page title and brand', () => {
      render(<TestComponent title="Test" />)
      expect(document.title).toContain('\u2014')
    })

    it('always includes Thriftly brand name', () => {
      render(<TestComponent title="Any Page" />)
      expect(document.title).toContain('Thriftly')
    })

    it('default title includes AI and family keywords for SEO', () => {
      render(<TestComponent />)
      expect(document.title).toContain('AI Shopping List')
      expect(document.title).toContain('Families')
    })
  })

  // ===== UPDATES =====
  describe('title updates on prop change', () => {
    it('updates title when page title prop changes', () => {
      const { rerender } = render(<TestComponent title="First" />)
      expect(document.title).toBe('First \u2014 Thriftly')
      rerender(<TestComponent title="Second" />)
      expect(document.title).toBe('Second \u2014 Thriftly')
    })

    it('switches to default title when prop removed', () => {
      const { rerender } = render(<TestComponent title="Page" />)
      expect(document.title).toBe('Page \u2014 Thriftly')
      rerender(<TestComponent />)
      expect(document.title).toBe(DEFAULT_TITLE)
    })

    it('switches from default to custom title', () => {
      const { rerender } = render(<TestComponent />)
      expect(document.title).toBe(DEFAULT_TITLE)
      rerender(<TestComponent title="Custom" />)
      expect(document.title).toBe('Custom \u2014 Thriftly')
    })
  })

  // ===== EDGE CASES =====
  describe('edge cases', () => {
    it('handles empty string title', () => {
      render(<TestComponent title="" />)
      // Empty string is falsy, so should use default
      expect(document.title).toBe(DEFAULT_TITLE)
    })

    it('handles special characters in title', () => {
      render(<TestComponent title="Terms & Conditions" />)
      expect(document.title).toBe('Terms & Conditions \u2014 Thriftly')
    })

    it('handles long page titles', () => {
      render(<TestComponent title="A Very Long Page Title That Goes On And On" />)
      expect(document.title).toBe('A Very Long Page Title That Goes On And On \u2014 Thriftly')
    })
  })

  // ===== MULTIPLE COMPONENTS =====
  describe('multiple components using hook', () => {
    it('last mounted component sets the title', () => {
      render(<TestComponent title="First" />)
      expect(document.title).toBe('First \u2014 Thriftly')

      render(<TestComponent title="Second" />)
      expect(document.title).toBe('Second \u2014 Thriftly')
    })

    it('title reflects the most recent render', () => {
      const { rerender } = render(<TestComponent title="Alpha" />)
      expect(document.title).toBe('Alpha \u2014 Thriftly')

      rerender(<TestComponent title="Beta" />)
      expect(document.title).toBe('Beta \u2014 Thriftly')

      rerender(<TestComponent title="Gamma" />)
      expect(document.title).toBe('Gamma \u2014 Thriftly')
    })
  })

  // ===== BRAND CONSISTENCY =====
  describe('brand consistency', () => {
    it('default title includes Thriftly brand name', () => {
      render(<TestComponent />)
      expect(document.title).toContain('Thriftly')
    })

    it('page-specific title includes Thriftly brand name', () => {
      render(<TestComponent title="About" />)
      expect(document.title).toContain('Thriftly')
    })

    it('base title is always Thriftly (not ThriftLy or THRIFTLY)', () => {
      render(<TestComponent title="Test" />)
      expect(document.title).toMatch(/Thriftly/)
      expect(document.title).not.toMatch(/THRIFTLY/)
      expect(document.title).not.toMatch(/ThriftLy/)
    })
  })
})
