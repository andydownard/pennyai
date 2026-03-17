import { Link } from 'react-router-dom'

export default function Footer() {
  return (
    <footer className="footer" role="contentinfo">
      <div className="footer-inner">
        <div className="footer-top">
          <div className="footer-brand">
            <div className="logo"><img src="/logo-white.svg" alt="thriftly" className="logo-img" /></div>
            <p>
              The AI shopping list for families. Text Penny what you need — she handles the rest.
            </p>
          </div>
          <div className="footer-col">
            <h4>Product</h4>
            <Link to="/#features">Features</Link>
            <Link to="/#how-it-works">How It Works</Link>
            <Link to="/#penny">Meet Penny</Link>
            <a href="https://app.thrift.ly" rel="noopener" target="_blank">Open Web App</a>
          </div>
          <div className="footer-col">
            <h4>Company</h4>
            <Link to="/for-brands">For Brands</Link>
            <Link to="/pricing">Pricing</Link>
            <Link to="/about">About</Link>
            <a href="mailto:hello@thrift.ly">Contact</a>
          </div>
          <div className="footer-col">
            <h4>Brands</h4>
            <Link to="/campaign/signup">Create a Free Deal</Link>
            <Link to="/for-brands">How Swaps Work</Link>
            <a href="https://brands.thrift.ly" rel="noopener" target="_blank">Brand Portal Login</a>
          </div>
          <div className="footer-col">
            <h4>Support</h4>
            <a href="mailto:support@thrift.ly">Email Support</a>
            <a href="mailto:penny@list.thrift.ly">Email Penny</a>
            <Link to="/about">Contact Us</Link>
          </div>
        </div>
        <div className="footer-bottom">
          <p>&copy; {new Date().getFullYear()} Thriftly, Inc. All rights reserved.</p>
          <div className="footer-bottom-links">
            <Link to="/privacy">Privacy</Link>
            <Link to="/terms">Terms</Link>
          </div>
        </div>
      </div>
    </footer>
  )
}
