/**
 * A/B Testing Client
 *
 * Usage:
 *   const variant = await getVariant('swap_card_layout');
 *   if (variant === 'variant') { ... show new layout ... }
 *
 *   trackEvent('swap_card_layout', 'swap_accepted', 1);
 */

const API_BASE = '';

// Cache variants in memory to avoid re-fetching per render
const variantCache: Record<string, string> = {};

/**
 * Get the variant assigned to the current user for a given test.
 * Returns empty string if test doesn't exist or user is not in test.
 * Caches in memory for the session.
 */
export async function getVariant(testName: string): Promise<string> {
  if (variantCache[testName] !== undefined) {
    return variantCache[testName];
  }

  try {
    const res = await fetch(`${API_BASE}/api/ab/variant?test=${encodeURIComponent(testName)}`, {
      credentials: 'include',
    });
    if (!res.ok) return '';
    const data = await res.json();
    variantCache[testName] = data.variant || '';
    return variantCache[testName];
  } catch {
    return '';
  }
}

/**
 * Track a metric event for an A/B test.
 * Fire-and-forget — does not block UI.
 */
export function trackEvent(testName: string, event: string, value: number = 1): void {
  fetch(`${API_BASE}/api/ab/event`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ test: testName, event, value }),
  }).catch(() => {}); // Silently fail — analytics should never break the app
}

/**
 * React hook for A/B testing.
 * Returns the variant string ('' while loading, then 'control' or 'variant' etc).
 *
 * Usage:
 *   function SwapCard() {
 *     const variant = useABTest('swap_card_layout');
 *     if (variant === 'variant') return <NewSwapCard />;
 *     return <OldSwapCard />;
 *   }
 */
export function useABTest(_testName: string): string {
  // Use a simple state pattern that works with React 18+
  // Import useState/useEffect at the call site if using as a hook
  // For now, provide the async function — components can call it in useEffect
  return ''; // Placeholder — see getVariant() for async version
}
