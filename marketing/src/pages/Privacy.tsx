import { useDocTitle } from '../hooks/useDocTitle'

export default function Privacy() {
  useDocTitle('Privacy Policy')
  return (
    <section className="legal-page">
      <div className="section-inner" style={{ maxWidth: 720, padding: '8rem 2rem 4rem' }}>
        <img src="/logo.svg" alt="Groflo" style={{ height: 24, marginBottom: '1.5rem' }} />
        <h1>Privacy Policy</h1>
        <p className="legal-updated">Effective: April 7, 2026</p>

        <p>
          Groflo, Inc. ("Groflo", "we") makes a shared shopping list app you control by texting Penny, our AI assistant. This policy tells you what we collect, how we use it, and what you control. We wrote it in plain English and we mean it.
        </p>

        <h2>1. Account and the Rally ecosystem</h2>
        <p>
          Your Groflo account is part of the <strong>Rally ecosystem</strong>, a shared identity layer that lets one Rally login work across Groflo and other Rally apps. <strong>Your account is shared. Your shopping list is not.</strong> Other Rally apps cannot read your list, your dietary preferences, your household, or your messages with Penny.
        </p>
        <p>
          When you create a Groflo account we record your phone number, optional email, optional display name, and the timestamp of your consent. If you sign in with Google, we also receive your Google email and name.
        </p>

        <h2>2. SMS messages and your consent</h2>
        <p>
          Penny is an SMS-native assistant. If you give us your phone number, you are choosing to receive text messages from Penny: replies to messages you send her, list updates from your household, and occasional reminders about items you have asked her to track.
        </p>
        <p>
          <strong>Message frequency varies</strong> based on your usage and your household's activity. <strong>Message and data rates may apply</strong> from your wireless carrier.
        </p>
        <p>
          You can change how often Penny texts you in Settings. You can stop texts at any time by replying <strong>STOP</strong> to any Penny message. After you reply STOP we send one confirmation text and no more. Reply <strong>HELP</strong> to any Penny message for support.
        </p>
        <p>
          <strong>We will not share your mobile information with third parties or affiliates for marketing or promotional purposes.</strong> Information you share with Penny stays inside Groflo and the service providers listed in section 4.
        </p>

        <h2>3. Information we collect</h2>
        <ul>
          <li><strong>Phone number</strong> — required for SMS, used to verify your account and to send and receive texts with Penny.</li>
          <li><strong>Email and name</strong> — optional, used to recover access and to address you in messages.</li>
          <li><strong>Shopping list data</strong> — items you add by app, text, or email, your edits, and your check-offs.</li>
          <li><strong>Household relationships</strong> — who you share lists with.</li>
          <li><strong>Brand preferences and dietary attributes</strong> — saved when you correct Penny or set them in Settings.</li>
          <li><strong>Receipts you upload</strong> — to verify swap rebates. We extract the line items and store the parsed data; the original image is deleted within 30 days.</li>
          <li><strong>Usage data</strong> — page views, feature usage, and error reports, used to keep Penny working.</li>
        </ul>

        <h2>4. Service providers</h2>
        <p>
          We share the minimum data needed with the providers who run the service. Each provider sees only the slice of data they need.
        </p>
        <ul>
          <li><strong>Twilio</strong> — sends and receives your SMS messages, and sends one-time verification codes.</li>
          <li><strong>Anthropic</strong> — runs the AI that turns your texts into list items. Your texts are sent for processing. Anthropic does not train its models on your data.</li>
          <li><strong>Stripe</strong> — processes brand swap rebate payouts to you, when applicable. Stripe is also the payments backbone for the Rally ecosystem.</li>
          <li><strong>Railway</strong> — hosts the Groflo backend.</li>
          <li><strong>Google</strong> — only if you sign in with Google.</li>
          <li><strong>SendGrid</strong> — sends transactional emails (password resets, magic links, optional digests).</li>
        </ul>

        <h2>5. How we use your data</h2>
        <ul>
          <li>To verify your identity and keep your account secure.</li>
          <li>To send and receive SMS messages with Penny and your household.</li>
          <li>To organize your list, learn your brand preferences, and respect your dietary attributes.</li>
          <li>To pay you brand swap rebates you have earned.</li>
          <li>To improve Penny — but only with anonymized, aggregated data, never your individual list.</li>
        </ul>

        <h2>6. Brand partners and rebates</h2>
        <p>
          Groflo is free for shoppers because brands pay to suggest products on your list. Suggestions are always labeled. Brand partners <strong>never</strong> see your name, your phone number, your email, your address, or your individual list. They see aggregated, anonymized statistics about how often their products are chosen across the whole Groflo population.
        </p>

        <h2>7. Data retention</h2>
        <p>
          We keep your account data while your account is active. You can delete individual list items at any time in the app. You can delete your account by emailing <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>. After we receive your request we delete your personal data within 30 days, except records we are legally required to keep (for example, financial records of rebate payouts).
        </p>

        <h2>8. Security</h2>
        <p>
          We protect your data with encrypted connections (TLS), HttpOnly session cookies, hashed passwords (bcrypt), short-lived authentication codes, and industry-standard server hardening. We do not store payment card numbers; Stripe does.
        </p>

        <h2>9. Your rights</h2>
        <p>
          You have the right to access, correct, export, or delete your personal data at any time. California residents have additional rights under the CCPA, including the right not to be discriminated against for exercising those rights. We do not sell or share personal information as those terms are defined under the CCPA.
        </p>
        <p>
          To exercise any of these rights, email <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>.
        </p>

        <h2>10. Children</h2>
        <p>
          Groflo is not for children under 13 and we do not knowingly collect data from children under 13.
        </p>

        <h2>11. Changes</h2>
        <p>
          When we make material changes to this policy we will notify you by email or by an in-app notice before the changes take effect. The "Effective" date at the top of this page always reflects the current version.
        </p>

        <h2>12. Contact</h2>
        <p>
          Questions or requests: <a href="mailto:privacy@groflo.io">privacy@groflo.io</a>.<br />
          Mailing address available on request.
        </p>
      </div>
    </section>
  )
}
