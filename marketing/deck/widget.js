// Thriftly Universal Feedback Widget
// Embed on any page: <script src="https://deck.thrift.ly/widget.js" async></script>
// Phone + SMS code auth via Twilio Verify. Creates GitHub issues automatically.
(function () {
  'use strict';

  var WIDGET_ID = 'tly-fb-widget';
  if (document.getElementById(WIDGET_ID)) return;

  // Allow API URL override: <script src="widget.js" data-api="http://localhost:3000"></script>
  var scripts = document.querySelectorAll('script[src*="widget.js"]');
  var scriptEl = scripts[scripts.length - 1];
  var API_BASE = (scriptEl && scriptEl.dataset.api) || 'https://deck.thrift.ly';

  // State
  var isOpen = false;
  var screen = 'phone'; // phone | code | form | success
  var phone = '';
  var code = '';
  var token = localStorage.getItem('tly_widget_token') || '';
  var urgency = 'normal';
  var feedbackText = '';
  var error = '';
  var sending = false;

  // ─── Styles ───
  function injectStyles() {
    var s = document.createElement('style');
    s.textContent = [
      '#tly-fb-fab{position:fixed;bottom:24px;right:24px;z-index:99999;width:44px;height:44px;border-radius:50%;background:#35644F;color:#fff;border:none;cursor:pointer;box-shadow:0 4px 14px rgba(53,100,79,.4);display:flex;align-items:center;justify-content:center;transition:transform .15s,box-shadow .15s;font-size:0;line-height:1}',
      '#tly-fb-fab:hover{transform:scale(1.08);box-shadow:0 6px 20px rgba(53,100,79,.5)}',
      '#tly-fb-fab:active{transform:scale(.95)}',
      '#tly-fb-fab svg{width:22px;height:22px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}',
      '#tly-fb-overlay{position:fixed;inset:0;z-index:100000;background:rgba(0,0,0,.5);display:none;align-items:flex-end;justify-content:center}',
      '#tly-fb-overlay.open{display:flex}',
      '@media(min-width:640px){#tly-fb-overlay{align-items:center}}',
      '#tly-fb-modal{background:#fff;width:100%;max-width:420px;border-radius:16px 16px 0 0;max-height:90vh;display:flex;flex-direction:column;overflow:hidden;font-family:Inter,-apple-system,system-ui,sans-serif}',
      '@media(min-width:640px){#tly-fb-modal{border-radius:16px;margin:24px}}',
      '.tly-fb-header{display:flex;align-items:center;padding:14px 16px;border-bottom:1px solid #e5e7eb;gap:8px}',
      '.tly-fb-header h3{margin:0;font-size:15px;font-weight:600;color:#1a1a1a;flex:1}',
      '.tly-fb-close{background:none;border:none;cursor:pointer;font-size:22px;color:#999;padding:2px 6px;border-radius:6px;line-height:1}',
      '.tly-fb-close:hover{background:#f3f4f6;color:#333}',
      '.tly-fb-body{padding:16px;flex:1;overflow-y:auto}',
      '.tly-fb-input{width:100%;padding:12px;border:1.5px solid #e5e7eb;border-radius:10px;font-size:16px;font-family:inherit;outline:none;box-sizing:border-box;transition:border-color .15s}',
      '.tly-fb-input:focus{border-color:#35644F}',
      '.tly-fb-textarea{width:100%;padding:12px;border:1.5px solid #e5e7eb;border-radius:10px;font-size:16px;font-family:inherit;outline:none;box-sizing:border-box;resize:none;min-height:120px;transition:border-color .15s}',
      '.tly-fb-textarea:focus{border-color:#35644F}',
      '.tly-fb-btn{width:100%;padding:12px;border:none;border-radius:10px;background:#35644F;color:#fff;font-size:15px;font-weight:600;font-family:inherit;cursor:pointer;transition:background .12s;margin-top:12px}',
      '.tly-fb-btn:hover{background:#2a503f}',
      '.tly-fb-btn:disabled{background:#8db5a0;cursor:not-allowed}',
      '.tly-fb-error{color:#ef4444;font-size:13px;margin-top:8px}',
      '.tly-fb-hint{color:#6b7280;font-size:13px;margin-top:8px}',
      '.tly-fb-priority{display:flex;gap:6px;margin-top:12px;align-items:center}',
      '.tly-fb-priority span{font-size:12px;color:#6b7280;margin-right:4px}',
      '.tly-fb-pill{padding:4px 12px;border-radius:100px;border:1.5px solid #e5e7eb;font-size:12px;font-weight:600;cursor:pointer;font-family:inherit;background:#fff;color:#6b7280;transition:all .12s}',
      '.tly-fb-pill.active{border-color:#35644F;background:rgba(53,100,79,.08);color:#35644F}',
      '.tly-fb-pill.critical.active{border-color:#ef4444;background:rgba(239,68,68,.08);color:#ef4444}',
      '.tly-fb-success{text-align:center;padding:40px 24px}',
      '.tly-fb-success-icon{width:56px;height:56px;border-radius:50%;background:#d1fae5;display:flex;align-items:center;justify-content:center;margin:0 auto 14px}',
      '.tly-fb-success-icon svg{width:28px;height:28px;fill:#059669}',
      '.tly-fb-success-msg{font-size:15px;color:#374151;line-height:1.5}',
      '.tly-fb-success-id{font-size:12px;color:#9ca3af;margin-top:6px}',
    ].join('\n');
    document.head.appendChild(s);
  }

  // ─── Helpers ───
  function esc(s) { return String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }

  function api(path, body) {
    var headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = 'Bearer ' + token;
    return fetch(API_BASE + path, {
      method: 'POST',
      headers: headers,
      body: JSON.stringify(body),
    }).then(function (r) { return r.json(); });
  }

  // ─── Render ───
  function render() {
    var overlay = document.getElementById('tly-fb-overlay');
    if (!overlay) return;
    if (!isOpen) { overlay.classList.remove('open'); overlay.innerHTML = ''; return; }
    overlay.classList.add('open');

    var html = '<div id="tly-fb-modal">';

    // Header
    html += '<div class="tly-fb-header">';
    html += '<h3>' + (screen === 'form' ? 'Send Feedback' : screen === 'success' ? 'Thank You' : 'Verify Your Phone') + '</h3>';
    html += '<button class="tly-fb-close" id="tly-fb-close-btn">&times;</button>';
    html += '</div>';

    html += '<div class="tly-fb-body">';

    if (screen === 'phone') {
      html += '<label style="font-size:13px;font-weight:500;color:#374151;display:block;margin-bottom:6px">Phone number</label>';
      html += '<input class="tly-fb-input" type="tel" id="tly-fb-phone" placeholder="(555) 123-4567" value="' + esc(phone) + '" />';
      html += '<button class="tly-fb-btn" id="tly-fb-send-code"' + (sending ? ' disabled' : '') + '>' + (sending ? 'Sending...' : 'Send Code') + '</button>';
      if (error) html += '<div class="tly-fb-error">' + esc(error) + '</div>';
      html += '<div class="tly-fb-hint">We\'ll text you a verification code.</div>';
    }

    if (screen === 'code') {
      html += '<label style="font-size:13px;font-weight:500;color:#374151;display:block;margin-bottom:6px">Enter the code we sent</label>';
      html += '<input class="tly-fb-input" type="text" inputmode="numeric" id="tly-fb-code" placeholder="123456" maxlength="6" value="' + esc(code) + '" />';
      html += '<button class="tly-fb-btn" id="tly-fb-verify"' + (sending ? ' disabled' : '') + '>' + (sending ? 'Verifying...' : 'Verify') + '</button>';
      if (error) html += '<div class="tly-fb-error">' + esc(error) + '</div>';
      html += '<div class="tly-fb-hint"><button style="background:none;border:none;color:#35644F;cursor:pointer;font-size:13px;font-family:inherit;padding:0;text-decoration:underline" id="tly-fb-back">Use a different number</button></div>';
    }

    if (screen === 'form') {
      html += '<textarea class="tly-fb-textarea" id="tly-fb-text" placeholder="Bug, feature idea, or anything else..." maxlength="5000">' + esc(feedbackText) + '</textarea>';
      html += '<div class="tly-fb-priority"><span>Priority:</span>';
      var pills = ['normal', 'critical'];
      var labels = { normal: 'Normal', critical: 'I insist' };
      for (var i = 0; i < pills.length; i++) {
        var p = pills[i];
        html += '<button class="tly-fb-pill' + (p === urgency ? ' active' : '') + (p === 'critical' ? ' critical' : '') + '" data-urg="' + p + '">' + labels[p] + '</button>';
      }
      html += '</div>';
      html += '<button class="tly-fb-btn" id="tly-fb-submit"' + (sending ? ' disabled' : '') + '>' + (sending ? 'Sending...' : 'Send') + '</button>';
      if (error) html += '<div class="tly-fb-error">' + esc(error) + '</div>';
      html += '<div class="tly-fb-hint">Cmd+Enter to send</div>';
    }

    if (screen === 'success') {
      html += '<div class="tly-fb-success">';
      html += '<div class="tly-fb-success-icon"><svg viewBox="0 0 24 24"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg></div>';
      html += '<div class="tly-fb-success-msg">' + (urgency === 'critical' ? 'Escalated to the product team.' : 'Submitted! The product team will review this.') + '</div>';
      html += '</div>';
    }

    html += '</div></div>';
    overlay.innerHTML = html;

    // Bind events
    var closeBtn = document.getElementById('tly-fb-close-btn');
    if (closeBtn) closeBtn.onclick = close;

    overlay.onclick = function (e) { if (e.target === overlay) close(); };

    if (screen === 'phone') {
      var phoneInput = document.getElementById('tly-fb-phone');
      if (phoneInput) {
        phoneInput.oninput = function () { phone = this.value; error = ''; };
        phoneInput.onkeydown = function (e) { if (e.key === 'Enter') sendCode(); };
        phoneInput.focus();
      }
      var sendCodeBtn = document.getElementById('tly-fb-send-code');
      if (sendCodeBtn) sendCodeBtn.onclick = sendCode;
    }

    if (screen === 'code') {
      var codeInput = document.getElementById('tly-fb-code');
      if (codeInput) {
        codeInput.oninput = function () { code = this.value; error = ''; };
        codeInput.onkeydown = function (e) { if (e.key === 'Enter') verifyCode(); };
        codeInput.focus();
      }
      var verifyBtn = document.getElementById('tly-fb-verify');
      if (verifyBtn) verifyBtn.onclick = verifyCode;
      var backBtn = document.getElementById('tly-fb-back');
      if (backBtn) backBtn.onclick = function () { screen = 'phone'; code = ''; error = ''; render(); };
    }

    if (screen === 'form') {
      var textarea = document.getElementById('tly-fb-text');
      if (textarea) {
        textarea.oninput = function () { feedbackText = this.value; error = ''; };
        textarea.onkeydown = function (e) { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); submitFeedback(); } };
        textarea.focus();
      }
      var submitBtn = document.getElementById('tly-fb-submit');
      if (submitBtn) submitBtn.onclick = submitFeedback;
      var pillBtns = overlay.querySelectorAll('.tly-fb-pill');
      for (var j = 0; j < pillBtns.length; j++) {
        pillBtns[j].onclick = (function (btn) {
          return function () { urgency = btn.getAttribute('data-urg'); render(); };
        })(pillBtns[j]);
      }
    }
  }

  // ─── Auth ───
  function sendCode() {
    var clean = phone.replace(/\D/g, '');
    if (clean.length < 10) { error = 'Enter a valid phone number.'; render(); return; }
    sending = true; error = ''; render();
    api('/api/widget/send-code', { phone: clean })
      .then(function (data) {
        sending = false;
        if (data.success) { screen = 'code'; error = ''; }
        else { error = data.error || 'Failed to send code.'; }
        render();
      })
      .catch(function () { sending = false; error = 'Network error.'; render(); });
  }

  function verifyCode() {
    if (!code || code.length < 4) { error = 'Enter the code we sent.'; render(); return; }
    var clean = phone.replace(/\D/g, '');
    sending = true; error = ''; render();
    api('/api/widget/verify', { phone: clean, code: code })
      .then(function (data) {
        sending = false;
        if (data.success && data.token) {
          token = data.token;
          localStorage.setItem('tly_widget_token', token);
          screen = 'form';
          error = '';
        } else {
          error = data.error || 'Invalid code.';
        }
        render();
      })
      .catch(function () { sending = false; error = 'Network error.'; render(); });
  }

  function validateSession() {
    if (!token) return;
    fetch(API_BASE + '/api/widget/validate', {
      method: 'GET',
      headers: { 'Authorization': 'Bearer ' + token },
    })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data.valid) { screen = 'form'; }
        else { token = ''; localStorage.removeItem('tly_widget_token'); screen = 'phone'; }
      })
      .catch(function () {});
  }

  // ─── Submit ───
  function submitFeedback() {
    var text = feedbackText.trim();
    if (!text) { error = 'Tell us what\'s on your mind.'; render(); return; }
    sending = true; error = ''; render();
    api('/api/widget/feedback', {
      body: text,
      urgency: urgency,
      source_url: window.location.href,
      page_title: document.title,
    })
      .then(function (data) {
        sending = false;
        if (data.success) {
          screen = 'success';
          feedbackText = '';
          urgency = 'normal';
          render();
          setTimeout(close, 3000);
        } else if (data.error === 'Not authenticated') {
          token = ''; localStorage.removeItem('tly_widget_token');
          screen = 'phone'; error = 'Session expired. Please verify again.';
          render();
        } else {
          error = data.error || 'Something went wrong.';
          render();
        }
      })
      .catch(function () { sending = false; error = 'Network error.'; render(); });
  }

  // ─── Open / Close ───
  function open() {
    isOpen = true;
    error = '';
    if (token) { screen = 'form'; validateSession(); }
    else { screen = 'phone'; }
    render();
  }

  function close() {
    if (sending) return;
    isOpen = false;
    screen = token ? 'form' : 'phone';
    error = '';
    render();
  }

  // ─── Init ───
  injectStyles();

  var container = document.createElement('div');
  container.id = WIDGET_ID;

  var fab = document.createElement('button');
  fab.id = 'tly-fb-fab';
  fab.title = 'Send feedback';
  fab.innerHTML = '<svg viewBox="0 0 24 24"><path d="M9 18h6"/><path d="M10 22h4"/><path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/></svg>';
  fab.onclick = open;

  var overlay = document.createElement('div');
  overlay.id = 'tly-fb-overlay';

  container.appendChild(fab);
  container.appendChild(overlay);
  document.body.appendChild(container);

  // Keyboard shortcuts
  document.addEventListener('keydown', function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key === '.') { e.preventDefault(); isOpen ? close() : open(); }
    if (e.key === 'Escape' && isOpen) close();
  });

  // Validate existing session on load
  if (token) validateSession();
})();
