import { useState } from 'react'
import { PennyAvatar } from './PennyAvatar'

const FONT = "'Avenir', 'Avenir Next', -apple-system, 'Segoe UI', sans-serif"

const demoItems = [
  { dept: 'Produce', icon: '🥬', items: [
    { name: 'Organic bananas', qty: '1 bunch', brand: 'Dole' },
    { name: 'Baby spinach', qty: '5 oz', brand: 'Earthbound' },
    { name: 'Avocados', qty: '3', brand: '' },
  ]},
  { dept: 'Dairy & Eggs', icon: '🥛', items: [
    { name: 'Whole milk', qty: '1 gal', brand: 'Horizon' },
    { name: 'Greek yogurt', qty: '32 oz', brand: 'Chobani' },
    { name: 'Shredded cheddar', qty: '8 oz', brand: 'Tillamook' },
  ]},
  { dept: 'Pantry & Dry Goods', icon: '🫙', items: [
    { name: 'Peanut butter', qty: '16 oz', brand: 'Jif' },
    { name: 'Pasta', qty: '1 lb', brand: 'Barilla' },
  ]},
]

export function EmbedView() {
  const [checked, setChecked] = useState<Set<string>>(() => new Set(['Baby spinach', 'Greek yogurt']))

  const toggle = (name: string) => {
    setChecked(prev => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  const total = demoItems.reduce((n, d) => n + d.items.length, 0)
  const done = checked.size

  return (
    <div style={{
      height: '100dvh',
      display: 'flex',
      flexDirection: 'column',
      backgroundColor: '#F7F6F3',
      fontFamily: FONT,
      overflow: 'hidden',
      color: '#2A2A2A',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        padding: '12px 16px 8px',
        borderBottom: '1px solid #E0DDD6',
        backgroundColor: '#FFFFFF',
        flexShrink: 0,
      }}>
        <PennyAvatar size={28} />
        <span style={{ fontFamily: FONT, fontSize: '1.125rem', fontWeight: 700, color: '#35644F', letterSpacing: '-0.3px' }}>
          Penny AI
        </span>
        <span style={{
          marginLeft: 'auto',
          fontSize: '0.75rem',
          color: '#525252',
          fontWeight: 500,
        }}>
          {done}/{total} done
        </span>
      </div>

      {/* Penny greeting bar */}
      <div style={{
        padding: '8px 16px',
        backgroundColor: '#F5F3EF',
        borderBottom: '1px solid #E0DDD6',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        flexShrink: 0,
      }}>
        <span style={{ fontSize: '0.8rem', color: '#525252', fontStyle: 'italic' }}>
          "I organized your list by department — happy shopping!"
        </span>
      </div>

      {/* List */}
      <div style={{
        flex: 1,
        overflowY: 'auto',
        overscrollBehavior: 'none',
        WebkitOverflowScrolling: 'touch',
      }}>
        {demoItems.map(dept => (
          <div key={dept.dept}>
            {/* Department header */}
            <div style={{
              padding: '8px 16px 4px',
              fontSize: '0.6875rem',
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.04em',
              color: '#35644F',
              backgroundColor: '#F7F6F3',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
            }}>
              <span>{dept.icon}</span>
              <span>{dept.dept}</span>
            </div>

            {/* Items */}
            {dept.items.map(item => {
              const isDone = checked.has(item.name)
              return (
                <div
                  key={item.name}
                  onClick={() => toggle(item.name)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    borderBottom: '1px solid #E0DDD6',
                    backgroundColor: '#FFFFFF',
                    cursor: 'pointer',
                    WebkitTapHighlightColor: 'transparent',
                    minHeight: 44,
                    opacity: isDone ? 0.5 : 1,
                    transition: 'opacity 0.2s ease',
                  }}
                >
                  {/* Checkbox */}
                  <div style={{
                    padding: '12px 8px 12px 14px',
                    flexShrink: 0,
                    display: 'flex',
                    alignItems: 'center',
                  }}>
                    <div style={{
                      width: 20,
                      height: 20,
                      borderRadius: '50%',
                      border: `2px solid ${isDone ? '#35644F' : '#525252'}`,
                      backgroundColor: isDone ? '#35644F' : 'transparent',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      transition: 'all 0.2s ease',
                      flexShrink: 0,
                    }}>
                      {isDone && <span style={{ color: '#fff', fontSize: 11, fontWeight: 700, lineHeight: 1 }}>✓</span>}
                    </div>
                  </div>

                  {/* Item info */}
                  <div style={{ flex: 1, padding: '10px 0', minWidth: 0 }}>
                    <div style={{
                      fontSize: '0.9375rem',
                      fontWeight: 500,
                      color: '#2A2A2A',
                      textDecoration: isDone ? 'line-through' : 'none',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}>
                      {item.name}
                    </div>
                    <div style={{
                      fontSize: '0.75rem',
                      color: '#525252',
                      marginTop: 1,
                    }}>
                      {item.qty}{item.brand ? ` · ${item.brand}` : ''}
                    </div>
                  </div>

                  {/* Right padding */}
                  <div style={{ width: 14, flexShrink: 0 }} />
                </div>
              )
            })}
          </div>
        ))}

        {/* Bottom padding */}
        <div style={{ height: 16 }} />
      </div>

      {/* Bottom bar — CTA */}
      <div style={{
        padding: '10px 16px',
        borderTop: '1px solid #E0DDD6',
        backgroundColor: '#FFFFFF',
        flexShrink: 0,
        textAlign: 'center',
      }}>
        <a
          href="https://pennyai.app"
          target="_blank"
          rel="noopener noreferrer"
          style={{
            display: 'inline-block',
            padding: '10px 24px',
            backgroundColor: '#35644F',
            color: '#FFFFFF',
            borderRadius: 10,
            fontSize: '0.875rem',
            fontWeight: 600,
            fontFamily: FONT,
            textDecoration: 'none',
            letterSpacing: '-0.2px',
          }}
        >
          Try Penny AI — it's free
        </a>
      </div>
    </div>
  )
}
