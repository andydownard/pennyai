import { useDocTitle } from '../hooks/useDocTitle'

export default function Privacy() {
  useDocTitle('Privacy Policy')
  return (
    <section className="legal-page">
      <div className="section-inner" style={{ maxWidth: 720, padding: '8rem 2rem 4rem' }}>
        <img src="/logo.svg" alt="thriftly" style={{ height: 24, marginBottom: '1.5rem' }} />
        <h1>Privacy Policy</h1>
        <p className="legal-updated">Last updated: March 2026</p>

        <h2>What we collect</h2>
        <p>
          When you use Thriftly, we collect the information you provide directly — such as your shopping list items, brand preferences, and account details. For brand advertisers, we also collect company and payment information during campaign signup.
        </p>

        <h2>How we use your data</h2>
        <p>
          We use your data to power the Thriftly experience: building your lists, learning your preferences, and keeping your household in sync. We never sell your individual shopping data to third parties.
        </p>

        <h2>Brand advertising</h2>
        <p>
          Thriftly is free for shoppers because brands pay to suggest products. When brand suggestions appear on your list, they are always clearly labeled. Brand partners receive only anonymized, aggregated insights — never your personal shopping data.
        </p>

        <h2>Data storage and security</h2>
        <p>
          Your data is stored securely and encrypted in transit and at rest. We follow industry-standard practices to protect your information.
        </p>

        <h2>Your rights</h2>
        <p>
          You can request access to, correction of, or deletion of your personal data at any time by contacting us at <a href="mailto:privacy@thrift.ly">privacy@thrift.ly</a>. California residents have additional rights under the CCPA.
        </p>

        <h2>Contact</h2>
        <p>
          Questions about this policy? Email us at <a href="mailto:privacy@thrift.ly">privacy@thrift.ly</a>.
        </p>
      </div>
    </section>
  )
}
