import { useState } from 'react'
import { useOnScreen } from '../hooks/useOnScreen'

export default function SavingsCalculator() {
  const calculator = useOnScreen()
  const [householdSize, setHouseholdSize] = useState(3)
  const [tripsPerWeek, setTripsPerWeek] = useState(2)

  return (
    <section className={`savings-calc fade-section${calculator.visible ? ' visible' : ''}`} ref={calculator.ref}>
      <div className="section-inner" style={{ textAlign: 'center' }}>
        <p className="section-label">Savings Calculator</p>
        <h2 className="section-title" style={{ textAlign: 'center' }}>How much could you save?</h2>
        <p className="section-subtitle" style={{ textAlign: 'center', margin: '0 auto 3rem', maxWidth: '520px' }}>
          Adjust the sliders to see your estimated monthly savings from Penny's deals and free products.
        </p>
        <div className="calc-card">
          <img src="/logo.svg" alt="thriftly" className="calc-logo" loading="lazy" />
          <div className="calc-inputs">
            <div className="calc-input-group">
              <label className="calc-label" htmlFor="calc-household">
                People in your household: <strong>{householdSize}</strong>
              </label>
              <input
                id="calc-household"
                type="range"
                min={1}
                max={6}
                value={householdSize}
                onChange={(e) => setHouseholdSize(Number(e.target.value))}
                className="calc-slider"
              />
            </div>
            <div className="calc-input-group">
              <label className="calc-label" htmlFor="calc-trips">
                Grocery trips per week: <strong>{tripsPerWeek}</strong>
              </label>
              <input
                id="calc-trips"
                type="range"
                min={1}
                max={5}
                value={tripsPerWeek}
                onChange={(e) => setTripsPerWeek(Number(e.target.value))}
                className="calc-slider"
              />
            </div>
          </div>
          <div className="calc-results">
            <div className="calc-result">
              <div className="calc-result-value">${Math.round(householdSize * tripsPerWeek * 8)}</div>
              <div className="calc-result-label">Estimated monthly savings</div>
            </div>
            <div className="calc-result">
              <div className="calc-result-value">{Math.round(householdSize * tripsPerWeek * 1.5)}</div>
              <div className="calc-result-label">Free products per month</div>
            </div>
            <div className="calc-result">
              <div className="calc-result-value">{Math.round(tripsPerWeek * 2.5)}</div>
              <div className="calc-result-label">Deals per week</div>
            </div>
          </div>
          <a href="https://app.thrift.ly" rel="noopener" target="_blank" className="btn-primary" style={{ marginTop: '1.5rem' }}>
            Start saving →
          </a>
        </div>
      </div>
    </section>
  )
}
