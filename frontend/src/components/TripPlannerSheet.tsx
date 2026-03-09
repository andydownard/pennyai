import { useState } from 'react'
import { ActionSheet } from './ActionSheet'
import { FONT, FONT_2XS, FONT_XS, FONT_SM, FONT_BASE, FONT_LG } from '../styles'

export type ShoppingTrip = {
  id: number
  user_id: number
  user_name: string
  trip_date: string
  trip_time: string
  time_label: string
  is_mine: boolean
  created_at: string
}

interface TripPlannerSheetProps {
  activeTrips: ShoppingTrip[]
  onClose: () => void
  onSaveTrip: (day: string, timeLabel: string, exactTime: string) => void
  onCancelTrip: () => void
  formatTripTime: (trip: ShoppingTrip) => string
  isTripInProgress: (trip: ShoppingTrip) => boolean
  getDayOptions: () => Array<{ label: string; value: string }>
  todayStr: string
  tomorrowStr: string
  satStr: string
  listStyles: {
    sheetButton: React.CSSProperties
  }
}

export function TripPlannerSheet({
  activeTrips, onClose, onSaveTrip, onCancelTrip,
  formatTripTime, isTripInProgress, getDayOptions,
  todayStr, tomorrowStr, satStr, listStyles,
}: TripPlannerSheetProps) {
  const [tripDay, setTripDay] = useState('')
  const [tripTimeLabel, setTripTimeLabel] = useState('')
  const [tripExactTime, setTripExactTime] = useState('')

  const myTrip = activeTrips.find(t => t.is_mine)

  if (myTrip) {
    return (
      <ActionSheet isOpen={true} onClose={onClose} title="Your planned trip" maxHeight="60dvh">
        <div style={{ textAlign: 'center', padding: '8px 0 16px' }}>
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--brand)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
          </svg>
          <p style={{ fontFamily: FONT, fontSize: FONT_LG, fontWeight: 600, color: 'var(--text)', margin: '12px 0 4px' }}>
            {isTripInProgress(myTrip) ? 'Shopping now!' : formatTripTime(myTrip)}
          </p>
          <p style={{ fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)', margin: 0 }}>
            Everyone on your list can see this.
          </p>
        </div>
        <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
          <button
            onClick={() => { onCancelTrip(); onClose() }}
            style={{ ...listStyles.sheetButton, flex: 1, backgroundColor: 'var(--error-bg)', color: 'var(--error)', border: 'none' }}
          >Cancel Trip</button>
        </div>
      </ActionSheet>
    )
  }

  return (
    <ActionSheet isOpen={true} onClose={onClose} title="Plan a shopping trip" maxHeight="60dvh">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
        <p style={{ fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--text)', margin: 0 }}>When?</p>
        <div style={{ display: 'flex', gap: '6px', flexWrap: 'nowrap', alignItems: 'center' }}>
          {[
            { label: 'Today', value: todayStr },
            { label: 'Tomorrow', value: tomorrowStr },
            { label: 'Weekend', value: satStr },
          ].map(opt => (
            <button
              key={opt.value}
              onClick={() => setTripDay(opt.value)}
              style={{
                padding: '6px 12px', borderRadius: '20px', border: `1.5px solid ${tripDay === opt.value ? 'var(--brand)' : 'var(--border)'}`,
                backgroundColor: tripDay === opt.value ? 'var(--brand)' : 'var(--surface)', color: tripDay === opt.value ? '#fff' : 'var(--text)',
                fontFamily: FONT, fontSize: FONT_SM, fontWeight: 500, cursor: 'pointer', whiteSpace: 'nowrap', flexShrink: 0,
              }}
            >{opt.label}</button>
          ))}
          <select
            value={getDayOptions().some(o => o.value === tripDay) ? tripDay : ''}
            onChange={e => setTripDay(e.target.value)}
            aria-label="Select another day"
            style={{
              padding: '6px 10px', borderRadius: '20px',
              border: `1.5px solid ${getDayOptions().some(o => o.value === tripDay) ? 'var(--brand)' : 'var(--border)'}`,
              backgroundColor: getDayOptions().some(o => o.value === tripDay) ? 'var(--brand)' : 'var(--surface)',
              color: getDayOptions().some(o => o.value === tripDay) ? '#fff' : 'var(--text-secondary)',
              fontFamily: FONT, fontSize: FONT_SM, cursor: 'pointer', flexShrink: 0, minWidth: 0,
            }}
          >
            <option value="">More...</option>
            {getDayOptions().map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>

        <p style={{ fontFamily: FONT, fontSize: FONT_2XS, fontWeight: 600, color: 'var(--text)', margin: 0 }}>
          What time? <span style={{ fontWeight: 400, color: 'var(--text-secondary)' }}>(optional)</span>
        </p>
        {tripTimeLabel === 'exact' ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <input
              type="time"
              value={tripExactTime}
              onChange={e => setTripExactTime(e.target.value)}
              aria-label="Exact trip time"
              autoFocus
              style={{
                flex: 1, padding: '6px 12px', borderRadius: '20px',
                border: '1.5px solid var(--brand)',
                fontFamily: FONT, fontSize: FONT_SM, color: 'var(--text)',
                height: '36px', boxSizing: 'border-box' as const,
              }}
            />
            <button
              onClick={() => setTripTimeLabel('')}
              style={{
                padding: '6px 12px', borderRadius: '20px', border: '1.5px solid var(--border)',
                backgroundColor: 'var(--surface)', color: 'var(--text-secondary)',
                fontFamily: FONT, fontSize: FONT_SM, cursor: 'pointer', whiteSpace: 'nowrap', flexShrink: 0,
              }}
            >Clear</button>
          </div>
        ) : (
          <div style={{ display: 'flex', gap: '6px', flexWrap: 'nowrap' }}>
            {['Morning', 'Afternoon', 'Evening', 'Exact'].map(t => (
              <button
                key={t}
                onClick={() => setTripTimeLabel(tripTimeLabel === t.toLowerCase() ? '' : t.toLowerCase())}
                style={{
                  flex: 1, padding: '6px 4px', borderRadius: '20px',
                  border: `1.5px solid ${tripTimeLabel === t.toLowerCase() ? 'var(--brand)' : 'var(--border)'}`,
                  backgroundColor: tripTimeLabel === t.toLowerCase() ? 'var(--brand)' : 'var(--surface)',
                  color: tripTimeLabel === t.toLowerCase() ? '#fff' : 'var(--text)',
                  fontFamily: FONT, fontSize: FONT_XS, fontWeight: 500, cursor: 'pointer',
                }}
              >{t}</button>
            ))}
          </div>
        )}

        <button
          onClick={() => onSaveTrip(tripDay, tripTimeLabel, tripExactTime)}
          disabled={!tripDay}
          style={{ ...listStyles.sheetButton, padding: '12px', marginTop: '2px', opacity: tripDay ? 1 : 0.4 }}
        >Save</button>
        <button
          onClick={onClose}
          style={{
            background: 'none', border: 'none', cursor: 'pointer',
            fontFamily: FONT, fontSize: FONT_BASE, color: 'var(--text-secondary)',
            padding: '4px 0', textAlign: 'center' as const, width: '100%',
          }}
        >Cancel</button>
      </div>
    </ActionSheet>
  )
}
