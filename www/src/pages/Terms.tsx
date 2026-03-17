import { useDocTitle } from '../hooks/useDocTitle'

export default function Terms() {
  useDocTitle('Terms of Service')
  return (
    <section className="legal-page">
      <div className="section-inner" style={{ maxWidth: 720, padding: '8rem 2rem 4rem' }}>
        <img src="/logo.svg" alt="thriftly" style={{ height: 24, marginBottom: '1.5rem' }} />
        <h1>Terms of Service</h1>
        <p className="legal-updated">Last updated: March 2026</p>

        <h2>For shoppers</h2>
        <p>
          Thriftly is free to use. By using the service, you agree to provide accurate information and to use the platform for personal, non-commercial grocery shopping purposes. We may display clearly labeled product suggestions from brand partners within your lists.
        </p>

        <h2>For advertisers</h2>
        <p>
          Brand advertisers agree to provide truthful product information and to comply with all applicable advertising laws and regulations. All campaigns are subject to review to ensure they align with our values and are genuinely useful to shoppers. We reserve the right to reject or remove campaigns that do not meet our standards.
        </p>

        <h2>Advertising guidelines</h2>
        <p>
          All sponsored content on Thriftly must be clearly labeled. Products must be accurately described. We do not accept campaigns for products that are harmful, misleading, or inappropriate for a grocery shopping context.
        </p>

        <h2>Payment terms</h2>
        <p>
          Advertisers are billed based on their selected pricing model (CPM or CPC). Your payment method is authorized at campaign submission and charged as your ad spend accrues. You can pause or cancel your campaign at any time.
        </p>

        <h2>Limitation of liability</h2>
        <p>
          Thriftly is provided as-is. We make no guarantees about specific savings, time reductions, or advertising performance. Results vary based on individual usage and campaign configuration.
        </p>

        <h2>Contact</h2>
        <p>
          Questions about these terms? Email us at <a href="mailto:legal@thrift.ly">legal@thrift.ly</a>.
        </p>
      </div>
    </section>
  )
}
