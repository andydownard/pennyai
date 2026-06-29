import { describe, it, expect } from 'vitest'
import { tokenizeAddInput, looksLikeAddCommand } from './parse-utils'

describe('looksLikeAddCommand', () => {
  it('treats plain item lists as adds', () => {
    expect(looksLikeAddCommand('milk, eggs, bread')).toBe(true)
    expect(looksLikeAddCommand('almond milk')).toBe(true)
  })
  it('rejects commands and questions', () => {
    expect(looksLikeAddCommand('remove milk')).toBe(false)
    expect(looksLikeAddCommand('clear my list')).toBe(false)
    expect(looksLikeAddCommand('check eggs')).toBe(false)
    expect(looksLikeAddCommand('what is on my list?')).toBe(false)
    expect(looksLikeAddCommand('how much have I saved?')).toBe(false)
  })
  it('handles empty input', () => {
    expect(looksLikeAddCommand('')).toBe(false)
    expect(looksLikeAddCommand('   ')).toBe(false)
  })
})

describe('tokenizeAddInput', () => {
  it('splits multi-item adds on commas', () => {
    expect(tokenizeAddInput('milk, eggs, bread')).toEqual(['milk', 'eggs', 'bread'])
  })
  it('splits on newlines, semicolons, &, +, and " and "', () => {
    expect(tokenizeAddInput('milk\neggs\nbread')).toEqual(['milk', 'eggs', 'bread'])
    expect(tokenizeAddInput('milk; eggs; bread')).toEqual(['milk', 'eggs', 'bread'])
    expect(tokenizeAddInput('milk & eggs')).toEqual(['milk', 'eggs'])
    expect(tokenizeAddInput('milk + eggs')).toEqual(['milk', 'eggs'])
    expect(tokenizeAddInput('milk and eggs')).toEqual(['milk', 'eggs'])
  })
  it('keeps multi-word single items intact', () => {
    expect(tokenizeAddInput('almond milk')).toEqual(['almond milk'])
  })
  it('returns one token for a single item', () => {
    expect(tokenizeAddInput('bananas')).toEqual(['bananas'])
  })
  it('returns no rows for commands/questions', () => {
    expect(tokenizeAddInput('remove milk')).toEqual([])
    expect(tokenizeAddInput('what is on my list?')).toEqual([])
  })
  it('drops empties and caps the count', () => {
    expect(tokenizeAddInput('milk,,, eggs')).toEqual(['milk', 'eggs'])
    const many = Array.from({ length: 30 }, (_, i) => `item${i}`).join(', ')
    expect(tokenizeAddInput(many).length).toBe(12)
  })
})
