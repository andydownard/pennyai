import { Link } from 'react-router-dom'

const APP_URL = import.meta.env.VITE_APP_URL || 'https://app.groflo.io'

export default function Footer() {
  return (
    <footer className="footer" role="contentinfo">
      <div className="footer-inner">
        <div className="footer-top">
          <div className="footer-brand">
            <div className="logo"><img src="/logo-white.svg" alt="Groflo" className="logo-img" /></div>
            <p>
              The AI shopping list for families. Text Penny what you need — she handles the rest.
            </p>
          </div>
          <div className="footer-col">
            <h4>Product</h4>
            <Link to="/#features">Features</Link>
            <Link to="/#how-it-works">How It Works</Link>
            <Link to="/#penny">Meet Penny</Link>
            <a href={APP_URL} rel="noopener" target="_blank">Open Web App</a>
          </div>
          <div className="footer-col">
            <h4>Company</h4>
            <Link to="/for-brands">For Brands</Link>
            <Link to="/pricing">Pricing</Link>
            <Link to="/about">About</Link>
            <a href="mailto:hello@groflo.io">Contact</a>
          </div>
          <div className="footer-col">
            <h4>Brands</h4>
            <Link to="/campaign/signup">Create a Free Deal</Link>
            <Link to="/for-brands">How Swaps Work</Link>
            <a href="https://brands.groflo.io" rel="noopener" target="_blank">Brand Portal Login</a>
          </div>
          <div className="footer-col">
            <h4>Support</h4>
            <a href="mailto:support@groflo.io">Email Support</a>
            <a href="mailto:penny@list.groflo.io">Email Penny</a>
            <Link to="/about">Contact Us</Link>
          </div>
        </div>
        <div className="footer-bottom">
          <p>&copy; {new Date().getFullYear()} Groflo, Inc. All rights reserved.</p>
          <div className="footer-bottom-links">
            <Link to="/privacy">Privacy</Link>
            <Link to="/terms">Terms</Link>
          </div>
        </div>
      </div>
    </footer>
  )
}
