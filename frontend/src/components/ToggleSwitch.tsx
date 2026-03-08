export function ToggleSwitch({ value, onChange, label }: { value: boolean; onChange: () => void; label?: string }) {
  return (
    <button
      role="switch"
      aria-checked={value}
      aria-label={label}
      onClick={onChange}
      style={{
        width: '44px', height: '24px', borderRadius: '12px', border: 'none', cursor: 'pointer',
        backgroundColor: value ? 'var(--brand)' : 'var(--border)', position: 'relative', transition: 'background-color 0.2s',
        padding: 0, flexShrink: 0,
      }}
    >
      <span style={{
        position: 'absolute', top: '2px', left: value ? '22px' : '2px',
        width: '20px', height: '20px', borderRadius: '50%', backgroundColor: '#fff',
        transition: 'left 0.2s', boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
      }} />
    </button>
  )
}
