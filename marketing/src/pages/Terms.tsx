import { useDocTitle } from '../hooks/useDocTitle'

export default function Terms() {
  useDocTitle('Terms of Service')
  return (
    <section className="legal-page">
      <div className="section-inner" style={{ maxWidth: 720, padding: '8rem 2rem 4rem' }}>
        <img src="/logo.svg" alt="Groflo" style={{ height: 24, marginBottom: '1.5rem' }} />
        <h1>Terms of Service</h1>
        <p className="legal-updated">Effective: April 7, 2026</p>

        <p>
          These terms cover your use of <strong>Groflo</strong>, the shared shopping list service operated by Groflo, Inc., and your conversations with <strong>Penny</strong>, our AI assistant. Read them. By using Groflo you accept them.
        </p>

        <h2>1. The service</h2>
        <p>
          Groflo lets you and your household share a single shopping list. You can add items by typing in the app, by texting Penny, or by emailing her. Penny uses AI to organize your list, learn your brand preferences, and surface savings.
        </p>

        <h2>2. Your account and the Rally ecosystem</h2>
        <p>
          Your Groflo account is part of the Rally ecosystem. One Rally login works across Groflo and other Rally apps. Your shopping list, household, dietary preferences, and messages with Penny are private to Groflo and are not shared with other Rally apps.
        </p>
        <p>
          You are responsible for keeping your account credentials secure. Tell us right away if you think someone else has access to your account.
        </p>

        <h2>3. SMS messaging program</h2>
        <p>
          When you give Groflo your phone number you are enrolling in Penny's SMS program. By enrolling, you agree to receive recurring text messages from Penny: replies, list updates from your household, and reminders.
        </p>
        <ul>
          <li><strong>Message frequency varies</strong> based on your usage and your household's activity.</li>
          <li><strong>Message and data rates may apply.</strong></li>
          <li>To stop messages, reply <strong>STOP</strong> to any Penny message. To get help, reply <strong>HELP</strong> to any Penny message or email <a href="mailto:support@groflo.io">support@groflo.io</a>.</li>
          <li>Carriers are not liable for delayed or undelivered messages.</li>
          <li>You can change how often Penny texts you at any time in Settings.</li>
        </ul>
        <p>
          For details on what we collect and how we use it, see our <a href="/privacy">Privacy Policy</a>.
        </p>

        <h2>4. Your content</h2>
        <p>
          You own your shopping list, your household, and the messages you send Penny. You give Groflo a limited license to store and display that content as needed to run the service. We will never sell your individual content.
        </p>

        <h2>5. Brand suggestions and swap rebates</h2>
        <p>
          Groflo is free because brands pay to suggest products on your list. Brand suggestions are always labeled. You are never required to accept a suggestion.
        </p>
        <p>
          When you accept a swap that includes a rebate, the rebate is paid to your in-app wallet. You can cash out your wallet balance through Stripe. Cash-outs are subject to Stripe's terms and may require identity verification. Rebate amounts and availability may change at any time.
        </p>

        <h2>6. Acceptable use</h2>
        <p>
          Use Groflo to manage your household's shopping. Do not use it to harass other users, to send unsolicited commercial messages, to scrape brand data, or to break the law. We may suspend accounts that abuse the service.
        </p>

        <h2>7. Eligibility</h2>
        <p>
          You must be at least 13 years old and have a valid US mobile phone number to use Groflo.
        </p>

        <h2>8. Termination</h2>
        <p>
          You can delete your account at any time by emailing <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>. We may suspend or terminate your account if you violate these terms or if we are required to by law. After termination, your data is deleted as described in the Privacy Policy.
        </p>

        <h2>9. Disclaimers</h2>
        <p>
          Groflo and Penny are provided "as is" and "as available." We do not guarantee that the service will be uninterrupted, error-free, or that brand suggestions will be accurate or available. Penny is an AI assistant and may make mistakes. Always check labels for allergens.
        </p>

        <h2>10. Limitation of liability</h2>
        <p>
          To the fullest extent permitted by law, Groflo, Inc., its officers, employees, and partners are not liable for any indirect, incidental, special, consequential, or punitive damages, or any loss of profits or revenues, arising from your use of the service. Our total liability for any claim is limited to one hundred US dollars ($100).
        </p>

        <h2>11. Governing law and disputes</h2>
        <p>
          These terms are governed by the laws of the State of Delaware, without regard to conflict-of-laws rules. Disputes will be resolved in the state or federal courts located in Delaware, except where you have a non-waivable right to bring claims in your local jurisdiction.
        </p>

        <h2>12. Changes</h2>
        <p>
          We may update these terms. When we do, we will post the updated terms here and update the Effective date. If the changes are material, we will notify you in the app or by email before they take effect. Continued use of Groflo after the changes take effect means you accept them.
        </p>

        <h2>13. Contact</h2>
        <p>
          Questions about these terms: <a href="mailto:legal@groflo.io">legal@groflo.io</a>. Support: <a href="mailto:support@groflo.io">support@groflo.io</a>.
        </p>
      </div>
    </section>
  )
}
