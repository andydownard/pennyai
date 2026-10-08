// One-shot migration: copy any legacy `thriftly_*` keys to `groflo_*` so
// existing users keep their dismissed prompts, redirect intents, and join
// codes after the rebrand. Runs at module load. Idempotent (a second run
// finds the new keys already present and removes the legacy ones).
//
// The rebrand's find-and-replace once turned LEGACY_PREFIX into 'groflo_' too,
// so every page load deleted every groflo_ key (dark mode, dismissed prompts,
// and the invite/redirect links kept across the Rally sign-in redirect).
// storageMigration.test.ts pins both prefixes.
const LEGACY_PREFIX = 'thriftly_'
const PREFIX = 'groflo_'

export function migrateLegacyStorage(stores?: Storage[]): void {
  try {
    const targets = stores ?? [window.localStorage, window.sessionStorage]
    for (const store of targets) {
      const legacyKeys: string[] = []
      for (let i = 0; i < store.length; i++) {
        const k = store.key(i)
        if (k && k.startsWith(LEGACY_PREFIX)) legacyKeys.push(k)
      }
      for (const legacyKey of legacyKeys) {
        const newKey = PREFIX + legacyKey.slice(LEGACY_PREFIX.length)
        const val = store.getItem(legacyKey)
        if (val !== null && store.getItem(newKey) === null) {
          store.setItem(newKey, val)
        }
        store.removeItem(legacyKey)
      }
    }
  } catch { /* private browsing or storage disabled — ignore */ }
}
