// =============================================================================
// Parse helpers for optimistic add UI
// =============================================================================
// These decide ONLY how many "pending" shimmer rows to show while the server
// parses an add. The server is always the source of truth for what actually
// lands on the list; these rows are render-only ghosts that get cleared the
// moment the request completes. So an imperfect token count is harmless.

// Leading words that mean the input is a COMMAND, not an add. We skip optimistic
// rows for these (we don't want a ghost "remove milk" row).
const COMMAND_LEADERS = [
  'remove', 'delete', 'clear', 'check', 'uncheck', 'undo', 'rename',
  'change', 'mark', 'cross', 'set', 'show', 'hide', 'what', 'how', 'why',
  'who', 'where', 'when', 'find', 'search', 'help',
]

/** True if the input looks like a question/command rather than items to add. */
export function looksLikeAddCommand(input: string): boolean {
  const t = input.trim().toLowerCase()
  if (!t) return false
  if (t.endsWith('?')) return false
  const first = t.split(/\s+/)[0]
  return !COMMAND_LEADERS.includes(first)
}

/**
 * Split a raw add into candidate item tokens for optimistic shimmer rows.
 * Splits on commas / newlines / semicolons / "&" / "+" / " and " — but not bare
 * spaces, so multi-word items ("almond milk") stay intact. Capped to keep the
 * ghost cluster sane on huge pastes.
 */
export function tokenizeAddInput(input: string): string[] {
  if (!looksLikeAddCommand(input)) return []
  return input
    .split(/\r?\n|,|;|&|\+|\sand\s/i)
    .map(s => s.trim())
    .filter(s => s.length > 0 && s.length <= 40)
    .slice(0, 12)
}
