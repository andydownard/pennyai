// =============================================================================
// HAPTIC FEEDBACK
// =============================================================================
// Single source of truth for haptic patterns. Previously duplicated inline in
// App.tsx, hooks/useListState.ts and hooks/useSwapState.ts. navigator.vibrate
// is optional-chained so this is a no-op on unsupported devices/browsers.

export const haptic = {
  light:   () => navigator.vibrate?.([5]),
  tap:     () => navigator.vibrate?.([10]),
  medium:  () => navigator.vibrate?.([15]),
  heavy:   () => navigator.vibrate?.([20]),
  success: () => navigator.vibrate?.([10, 50, 5, 50, 15]),
  error:   () => navigator.vibrate?.([20, 40, 20]),
  double:  () => navigator.vibrate?.([5, 30, 5]),
}
