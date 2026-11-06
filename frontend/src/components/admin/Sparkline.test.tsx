import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { Sparkline, BarChart } from './Sparkline'

describe('Sparkline', () => {
  it('renders nothing for fewer than two values', () => {
    expect(render(<Sparkline values={[]} />).container.firstChild).toBeNull()
    expect(render(<Sparkline values={[5]} />).container.firstChild).toBeNull()
  })

  it('draws a line, area and end dot at the default size', () => {
    const { container } = render(<Sparkline values={[1, 2, 3]} />)
    const svg = container.querySelector('svg')!
    expect(svg).toHaveAttribute('width', '280')
    expect(svg).toHaveAttribute('height', '60')
    expect(container.querySelector('polyline')!.getAttribute('points')!.split(' ')).toHaveLength(3)
    expect(container.querySelector('path')).toBeInTheDocument()
    expect(container.querySelectorAll('circle')).toHaveLength(1)
  })

  it('puts the end dot on the last point and honours size and colour', () => {
    const { container } = render(<Sparkline values={[0, 10]} width={104} height={44} color="red" />)
    const circle = container.querySelector('circle')!
    expect(circle).toHaveAttribute('cx', '100')
    expect(circle).toHaveAttribute('cy', '4')
    expect(circle).toHaveAttribute('fill', 'red')
    expect(container.querySelector('polyline')).toHaveAttribute('stroke', 'red')
  })

  it('survives a flat series without NaN', () => {
    const { container } = render(<Sparkline values={[3, 3, 3]} />)
    expect(container.querySelector('polyline')!.getAttribute('points')).not.toMatch(/NaN/)
  })
})

describe('BarChart', () => {
  it('renders nothing when empty', () => {
    expect(render(<BarChart values={[]} />).container.firstChild).toBeNull()
  })

  it('draws one bar per value, tallest for the max', () => {
    const { container } = render(<BarChart values={[1, 4, 2]} />)
    const rects = [...container.querySelectorAll('rect')]
    expect(rects).toHaveLength(3)
    const h = rects.map(r => Number(r.getAttribute('height')))
    expect(h[1]).toBeGreaterThan(h[0])
    expect(h[1]).toBeGreaterThan(h[2])
  })

  it('keeps zero-value bars visible', () => {
    const { container } = render(<BarChart values={[0, 0]} />)
    for (const r of container.querySelectorAll('rect')) expect(Number(r.getAttribute('height'))).toBeGreaterThanOrEqual(1)
  })

  it('labels only evenly spaced bars', () => {
    const labels = Array.from({ length: 12 }, (_, i) => `L${i}`)
    const { container } = render(<BarChart values={Array(12).fill(1)} labels={labels} />)
    const shown = [...container.querySelectorAll('text')].map(t => t.textContent)
    expect(shown).toEqual(['L0', 'L2', 'L4', 'L6', 'L8', 'L10'])
  })

  it('draws no labels when none are given', () => {
    const { container } = render(<BarChart values={[1, 2]} />)
    expect(container.querySelectorAll('text')).toHaveLength(0)
  })
})
