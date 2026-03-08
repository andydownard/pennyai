// Thriftly Feedback Widget
// Full-screen modal. Priority buttons: "I insist" vs "Normal".
// Submit in header so it's always visible above mobile keyboard.
(function () {
  'use strict';

  var WIDGET_ID = 'thriftly-feedback-widget';
  if (document.getElementById(WIDGET_ID)) return;

  var isOpen = false;
  var canSubmit = false;

  var checkInterval = null;
  function checkAuth() {
    fetch('/auth/me', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data.can_submit_feedback && !canSubmit) {
          canSubmit = true;
          injectWidget();
          if (checkInterval) { clearInterval(checkInterval); checkInterval = null; }
        }
      })
      .catch(function () {});
  }
  checkAuth();
  checkInterval = setInterval(function () {
    if (canSubmit) { clearInterval(checkInterval); return; }
    checkAuth();
  }, 3000);

  var statusLabels = {
    'new': 'Submitted',
    'reviewed': 'Under Review',
    'roadmapped': 'In Progress',
    'done': 'Shipped'
  };

  var statusDescriptions = {
    'new': 'The product team will review this soon.',
    'reviewed': 'The product team is evaluating this.',
    'roadmapped': 'This is being built.',
    'done': 'This has been shipped.'
  };

  function injectStyles() {
    var css = document.createElement('style');
    css.textContent = [
      '#thriftly-fb-fab {',
      '  position: fixed; bottom: 160px; left: 16px; z-index: 99999;',
      '  width: 40px; height: 40px; border-radius: 50%;',
      '  background: #35644F; color: #fff; border: none; cursor: pointer;',
      '  box-shadow: 0 4px 14px rgba(53,100,79,0.4);',
      '  display: flex; align-items: center; justify-content: center;',
      '  transition: transform 0.15s ease, box-shadow 0.15s ease;',
      '  font-size: 24px; line-height: 1;',
      '}',
      '#thriftly-fb-fab:hover { transform: scale(1.08); box-shadow: 0 6px 20px rgba(53,100,79,0.5); }',
      '#thriftly-fb-fab:active { transform: scale(0.95); }',
      '#thriftly-fb-fab svg { width: 20px; height: 20px; fill: currentColor; }',
      '',
      '#thriftly-fb-overlay {',
      '  position: fixed; inset: 0; z-index: 100000;',
      '  background: #fff; display: none;',
      '  flex-direction: column;',
      '}',
      '#thriftly-fb-overlay.open { display: flex; }',
      '',
      '.thriftly-fb-header {',
      '  display: flex; align-items: center; justify-content: space-between;',
      '  padding: 12px 16px; border-bottom: 1px solid #e5e7eb; flex-shrink: 0;',
      '  gap: 8px;',
      '}',
      '.thriftly-fb-header h2 { margin: 0; font-size: 16px; font-weight: 600; color: #1a1a1a; flex: 1; }',
      '.thriftly-fb-close {',
      '  background: none; border: none; cursor: pointer; font-size: 22px;',
      '  color: #999; padding: 4px 8px; border-radius: 6px; line-height: 1; flex-shrink: 0;',
      '}',
      '.thriftly-fb-close:hover { background: #f3f4f6; color: #333; }',
      '.thriftly-fb-send {',
      '  background: #35644F; color: #fff; border: none; border-radius: 8px;',
      '  padding: 6px 16px; font-size: 14px; font-weight: 600; cursor: pointer;',
      '  flex-shrink: 0; transition: background 0.12s;',
      '}',
      '.thriftly-fb-send:hover { background: #2a503f; }',
      '.thriftly-fb-send:disabled { background: #8db5a0; cursor: not-allowed; }',
      '',
      '.thriftly-fb-priority-row {',
      '  display: flex; align-items: center; gap: 10px; padding: 10px 16px; flex-shrink: 0;',
      '  border-bottom: 1px solid #f3f4f6;',
      '}',
      '.thriftly-fb-toggle-track {',
      '  width: 36px; height: 20px; border-radius: 10px; cursor: pointer;',
      '  position: relative; transition: background-color 0.2s; flex-shrink: 0;',
      '}',
      '.thriftly-fb-toggle-thumb {',
      '  width: 16px; height: 16px; border-radius: 50%; background: #fff;',
      '  position: absolute; top: 2px; transition: left 0.2s;',
      '  box-shadow: 0 1px 3px rgba(0,0,0,0.2);',
      '}',
      '.thriftly-fb-toggle-label {',
      '  font-size: 13px; font-weight: 600; cursor: pointer; user-select: none;',
      '}',
      '',
      '.thriftly-fb-body {',
      '  padding: 12px 16px; flex: 1; display: flex; flex-direction: column;',
      '  max-width: 600px; width: 100%; margin: 0 auto; box-sizing: border-box;',
      '}',
      '',
      '.thriftly-fb-textarea {',
      '  width: 100%; flex: 1; min-height: 80px; padding: 12px;',
      '  border: 1.5px solid #e5e7eb; border-radius: 12px; font-size: 16px;',
      '  font-family: inherit; resize: none; outline: none;',
      '  transition: border-color 0.15s; box-sizing: border-box;',
      '}',
      '.thriftly-fb-textarea:focus { border-color: #35644F; }',
      '.thriftly-fb-textarea::placeholder { color: #9ca3af; }',
      '.thriftly-fb-textarea.error { border-color: #ef4444; }',
      '',
      '.thriftly-fb-error-msg {',
      '  color: #ef4444; font-size: 13px; margin-top: 6px; display: none;',
      '}',
      '.thriftly-fb-error-msg.show { display: block; }',
      '',
      '.thriftly-fb-success { text-align: center; padding: 60px 24px; flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; }',
      '.thriftly-fb-success-icon {',
      '  width: 64px; height: 64px; margin: 0 auto 16px;',
      '  background: #d1fae5; border-radius: 50%; display: flex;',
      '  align-items: center; justify-content: center;',
      '}',
      '.thriftly-fb-success-icon svg { width: 32px; height: 32px; fill: #059669; }',
      '.thriftly-fb-success-msg { font-size: 17px; color: #374151; line-height: 1.5; }',
      '.thriftly-fb-success-id { font-size: 13px; color: #9ca3af; margin-top: 6px; }',
      '',
      '.thriftly-fb-footer {',
      '  padding: 10px 16px; text-align: center; flex-shrink: 0;',
      '  border-top: 1px solid #e5e7eb;',
      '}',
      '.thriftly-fb-toggle-history {',
      '  background: none; border: none; color: #6b7280; font-size: 13px;',
      '  cursor: pointer; text-decoration: underline; padding: 4px;',
      '}',
      '.thriftly-fb-toggle-history:hover { color: #35644F; }',
      '',
      '.thriftly-fb-history-item {',
      '  border: 1px solid #e5e7eb; border-radius: 12px; padding: 14px;',
      '  margin-bottom: 12px;',
      '}',
      '.thriftly-fb-history-body { font-size: 15px; color: #374151; white-space: pre-wrap; word-break: break-word; line-height: 1.5; }',
      '.thriftly-fb-history-meta {',
      '  font-size: 12px; color: #9ca3af; margin-top: 8px;',
      '  display: flex; gap: 10px; flex-wrap: wrap; align-items: center;',
      '}',
      '.thriftly-fb-history-status {',
      '  display: inline-block; padding: 3px 10px; border-radius: 10px;',
      '  font-size: 12px; font-weight: 600;',
      '}',
      '.thriftly-fb-status-new { background: #e8f0ec; color: #35644F; }',
      '.thriftly-fb-status-reviewed { background: #fef3c7; color: #92400e; }',
      '.thriftly-fb-status-roadmapped { background: #d1fae5; color: #065f46; }',
      '.thriftly-fb-status-done { background: #e5e7eb; color: #374151; }',
      '.thriftly-fb-history-outcome {',
      '  font-size: 13px; color: #374151; margin-top: 8px; padding: 8px 10px;',
      '  background: #f9fafb; border-radius: 8px; line-height: 1.4;',
      '}',
      '.thriftly-fb-history-outcome strong { color: #1a1a1a; }',
      '.thriftly-fb-gh-link { font-size: 12px; color: #35644F; text-decoration: none; }',
      '.thriftly-fb-gh-link:hover { text-decoration: underline; }',
      '.thriftly-fb-empty { text-align: center; color: #9ca3af; padding: 40px 24px; font-size: 15px; }',
      '',
      '.thriftly-fb-insist-btn {',
      '  background: none; border: 1px solid #ef4444; color: #ef4444;',
      '  padding: 4px 12px; border-radius: 8px; font-size: 12px; font-weight: 600;',
      '  cursor: pointer; transition: all 0.12s; margin-top: 8px;',
      '}',
      '.thriftly-fb-insist-btn:hover { background: #ef4444; color: #fff; }',
      '.thriftly-fb-audit-trail { font-size: 12px; color: #6b7280; margin-top: 6px; line-height: 1.5; }',
    ].join('\n');
    document.head.appendChild(css);
  }

  function injectWidget() {
    injectStyles();
    var container = document.createElement('div');
    container.id = WIDGET_ID;

    var fab = document.createElement('button');
    fab.id = 'thriftly-fb-fab';
    fab.title = 'Instructions for Product Team';
    fab.innerHTML = '<svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12zM7 9h2v2H7zm4 0h2v2h-2zm4 0h2v2h-2z"/></svg>';
    fab.onclick = function () { openModal(); };

    var overlay = document.createElement('div');
    overlay.id = 'thriftly-fb-overlay';

    container.appendChild(fab);
    container.appendChild(overlay);
    document.body.appendChild(container);

    document.addEventListener('keydown', function (e) {
      if ((e.metaKey || e.ctrlKey) && e.key === '.') {
        e.preventDefault();
        if (isOpen) closeModal(); else openModal();
      }
      if (e.key === 'Escape' && isOpen) closeModal();
    });
  }

  function openModal() {
    isOpen = true;
    var overlay = document.getElementById('thriftly-fb-overlay');
    overlay.classList.add('open');
    overlay.innerHTML = '';
    overlay.appendChild(buildFormModal());
    var ta = overlay.querySelector('.thriftly-fb-textarea');
    if (ta) setTimeout(function () { ta.focus(); }, 50);
  }

  function closeModal() {
    isOpen = false;
    var overlay = document.getElementById('thriftly-fb-overlay');
    overlay.classList.remove('open');
    overlay.innerHTML = '';
  }

  function buildFormModal() {
    var modal = document.createElement('div');
    modal.style.cssText = 'display:flex;flex-direction:column;height:100%;';

    var selectedPriority = 'critical';

    var header = document.createElement('div');
    header.className = 'thriftly-fb-header';

    var closeBtn = document.createElement('button');
    closeBtn.className = 'thriftly-fb-close';
    closeBtn.innerHTML = '&times;';
    closeBtn.onclick = closeModal;

    var title = document.createElement('h2');
    title.textContent = 'Instructions for Product Team';

    var historyBtn = document.createElement('button');
    historyBtn.className = 'thriftly-fb-close';
    historyBtn.title = 'Past submissions';
    historyBtn.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>';
    historyBtn.onclick = function () { showHistory(); };

    var sendBtn = document.createElement('button');
    sendBtn.className = 'thriftly-fb-send';
    sendBtn.textContent = 'Send';

    header.appendChild(closeBtn);
    header.appendChild(title);
    header.appendChild(historyBtn);
    header.appendChild(sendBtn);

    var priorityRow = document.createElement('div');
    priorityRow.className = 'thriftly-fb-priority-row';

    var track = document.createElement('div');
    track.className = 'thriftly-fb-toggle-track';
    var thumb = document.createElement('div');
    thumb.className = 'thriftly-fb-toggle-thumb';
    track.appendChild(thumb);

    var label = document.createElement('span');
    label.className = 'thriftly-fb-toggle-label';

    function updateToggleUI() {
      var isInsist = selectedPriority === 'critical';
      track.style.backgroundColor = isInsist ? '#35644F' : '#e5e7eb';
      thumb.style.left = isInsist ? '18px' : '2px';
      label.textContent = isInsist ? 'I insist' : 'Normal';
      label.style.color = isInsist ? '#35644F' : '#9ca3af';
    }
    updateToggleUI();

    function togglePriority() {
      selectedPriority = selectedPriority === 'critical' ? 'normal' : 'critical';
      updateToggleUI();
    }
    track.onclick = togglePriority;
    label.onclick = togglePriority;

    priorityRow.appendChild(track);
    priorityRow.appendChild(label);

    var body = document.createElement('div');
    body.className = 'thriftly-fb-body';

    var textarea = document.createElement('textarea');
    textarea.className = 'thriftly-fb-textarea';
    textarea.placeholder = "What's on your mind? Bug, feature idea, or anything else...";
    textarea.setAttribute('maxlength', '10000');

    var errorMsg = document.createElement('div');
    errorMsg.className = 'thriftly-fb-error-msg';

    textarea.addEventListener('input', function () {
      if (this.value.length > 0) {
        textarea.classList.remove('error');
        errorMsg.classList.remove('show');
      }
    });

    textarea.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        sendBtn.click();
      }
    });

    body.appendChild(textarea);
    body.appendChild(errorMsg);

    sendBtn.onclick = function () {
      var text = textarea.value.trim();
      if (!text) {
        textarea.classList.add('error');
        errorMsg.textContent = "Tell us what's on your mind.";
        errorMsg.classList.add('show');
        textarea.focus();
        return;
      }
      sendBtn.disabled = true;
      sendBtn.textContent = '...';

      fetch('/api/feedback', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ body: text, type: 'feedback', urgency: selectedPriority }),
      })
        .then(function (r) { return r.json(); })
        .then(function (data) {
          if (data.success) {
            showSuccess(data.id, data.message, selectedPriority);
          } else {
            sendBtn.disabled = false;
            sendBtn.textContent = 'Send';
            errorMsg.textContent = data.error || 'Something went wrong';
            errorMsg.classList.add('show');
          }
        })
        .catch(function () {
          sendBtn.disabled = false;
          sendBtn.textContent = 'Send';
          errorMsg.textContent = 'Network error — please try again.';
          errorMsg.classList.add('show');
        });
    };

    modal.appendChild(header);
    modal.appendChild(priorityRow);
    modal.appendChild(body);
    return modal;
  }

  function showSuccess(feedbackId, message, priority) {
    var overlay = document.getElementById('thriftly-fb-overlay');
    overlay.innerHTML = '';

    var modal = document.createElement('div');
    modal.style.cssText = 'display:flex;flex-direction:column;height:100%;';

    var header = document.createElement('div');
    header.className = 'thriftly-fb-header';
    var successTitle = document.createElement('h2');
    successTitle.style.flex = '1';
    successTitle.textContent = 'Instructions for Product Team';
    header.appendChild(successTitle);
    var closeBtn = document.createElement('button');
    closeBtn.className = 'thriftly-fb-close';
    closeBtn.textContent = '\u00D7';
    closeBtn.onclick = closeModal;
    header.appendChild(closeBtn);

    var success = document.createElement('div');
    success.className = 'thriftly-fb-success';

    var icon = document.createElement('div');
    icon.className = 'thriftly-fb-success-icon';
    icon.innerHTML = '<svg viewBox="0 0 24 24"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/></svg>';

    var msg = document.createElement('div');
    msg.className = 'thriftly-fb-success-msg';
    if (priority === 'critical') {
      msg.textContent = 'Escalated to the CEO. The product team will act on this immediately.';
    } else {
      msg.textContent = message || 'Submitted to the product team for prioritization.';
    }

    var idLine = document.createElement('div');
    idLine.className = 'thriftly-fb-success-id';
    idLine.textContent = feedbackId ? 'Reference: #' + feedbackId : '';

    success.appendChild(icon);
    success.appendChild(msg);
    success.appendChild(idLine);

    modal.appendChild(header);
    modal.appendChild(success);
    overlay.appendChild(modal);

    setTimeout(function () { closeModal(); }, 3000);
  }

  function showHistory() {
    var overlay = document.getElementById('thriftly-fb-overlay');
    overlay.innerHTML = '';

    var modal = document.createElement('div');
    modal.style.cssText = 'display:flex;flex-direction:column;height:100%;';

    var header = document.createElement('div');
    header.className = 'thriftly-fb-header';
    var histTitle = document.createElement('h2');
    histTitle.style.flex = '1';
    histTitle.textContent = 'My Submissions';
    header.appendChild(histTitle);
    var closeBtn = document.createElement('button');
    closeBtn.className = 'thriftly-fb-close';
    closeBtn.textContent = '\u00D7';
    closeBtn.onclick = closeModal;
    header.appendChild(closeBtn);

    var body = document.createElement('div');
    body.className = 'thriftly-fb-body';
    body.style.overflowY = 'auto';
    body.innerHTML = '<div class="thriftly-fb-empty">Loading...</div>';

    var footer = document.createElement('div');
    footer.className = 'thriftly-fb-footer';
    var backBtn = document.createElement('button');
    backBtn.className = 'thriftly-fb-toggle-history';
    backBtn.textContent = 'New feedback';
    backBtn.onclick = function () { openModal(); };
    footer.appendChild(backBtn);

    modal.appendChild(header);
    modal.appendChild(body);
    modal.appendChild(footer);
    overlay.appendChild(modal);

    fetch('/api/feedback', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (items) {
        body.innerHTML = '';
        if (!items || items.length === 0) {
          body.innerHTML = '<div class="thriftly-fb-empty">No submissions yet.</div>';
          return;
        }
        var urgencyLabels = { critical: 'I insist', high: 'High', normal: 'Normal', low: 'Backlog' };
        var urgencyColors = { critical: '#ef4444', high: '#f59e0b', normal: '#35644F', low: '#9ca3af' };

        items.forEach(function (item) {
          var el = document.createElement('div');
          el.className = 'thriftly-fb-history-item';

          var bodyText = document.createElement('div');
          bodyText.className = 'thriftly-fb-history-body';
          bodyText.textContent = item.body;

          var meta = document.createElement('div');
          meta.className = 'thriftly-fb-history-meta';

          if (item.urgency) {
            var urgSpan = document.createElement('span');
            urgSpan.style.cssText = 'padding:2px 8px;border-radius:8px;font-size:11px;font-weight:600;color:#fff;background:' + (urgencyColors[item.urgency] || '#9ca3af');
            urgSpan.textContent = urgencyLabels[item.urgency] || item.urgency;
            meta.appendChild(urgSpan);
          }

          var statusSpan = document.createElement('span');
          statusSpan.className = 'thriftly-fb-history-status thriftly-fb-status-' + item.status;
          statusSpan.textContent = statusLabels[item.status] || item.status;
          meta.appendChild(statusSpan);

          var dateSpan = document.createElement('span');
          var d = new Date(item.created_at + 'Z');
          dateSpan.textContent = d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          meta.appendChild(dateSpan);

          el.appendChild(bodyText);
          el.appendChild(meta);

          var trailParts = [];
          if (item.github_issue_url) {
            trailParts.push({ type: 'link', url: item.github_issue_url, text: 'GitHub issue created' });
          }
          if (item.updated_at) {
            var updatedDate = new Date(item.updated_at + 'Z');
            trailParts.push({ type: 'text', text: 'Last updated ' + updatedDate.toLocaleDateString() });
          }
          if (item.status !== 'new') {
            trailParts.push({ type: 'text', text: 'Status: ' + (statusLabels[item.status] || item.status) });
          }
          if (trailParts.length > 0) {
            var trail = document.createElement('div');
            trail.className = 'thriftly-fb-history-outcome';
            var trailLabel = document.createElement('strong');
            trailLabel.textContent = 'Trail:';
            trail.appendChild(trailLabel);
            trail.appendChild(document.createTextNode(' '));
            trailParts.forEach(function (part, idx) {
              if (idx > 0) trail.appendChild(document.createTextNode(' \u00B7 '));
              if (part.type === 'link') {
                var a = document.createElement('a');
                a.className = 'thriftly-fb-gh-link';
                a.href = part.url;
                a.target = '_blank';
                a.rel = 'noopener';
                a.textContent = part.text;
                if (a.protocol !== 'https:' && a.protocol !== 'http:') a.removeAttribute('href');
                trail.appendChild(a);
              } else {
                trail.appendChild(document.createTextNode(part.text));
              }
            });
            el.appendChild(trail);
          }

          var outcomeText = '';
          if (item.admin_notes) {
            outcomeText = item.admin_notes;
          } else if (item.status !== 'new') {
            outcomeText = statusDescriptions[item.status] || '';
          }
          if (outcomeText) {
            var outcome = document.createElement('div');
            outcome.className = 'thriftly-fb-history-outcome';
            var outcomeLabel = document.createElement('strong');
            outcomeLabel.textContent = 'Product team:';
            outcome.appendChild(outcomeLabel);
            outcome.appendChild(document.createTextNode(' ' + outcomeText));
            el.appendChild(outcome);
          }

          if (item.status !== 'done' && item.status !== 'roadmapped' && item.status !== 'reviewed' && item.urgency !== 'critical') {
            var insistBtn = document.createElement('button');
            insistBtn.className = 'thriftly-fb-insist-btn';
            insistBtn.textContent = 'I insist — escalate this';
            insistBtn.onclick = function (e) {
              e.stopPropagation();
              insistBtn.disabled = true;
              insistBtn.textContent = 'Escalating...';
              fetch('/api/feedback', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  body: '[ESCALATION] Re: #' + item.id + ' — ' + item.body.substring(0, 200),
                  type: 'feedback',
                  urgency: 'critical',
                }),
              })
                .then(function (r) { return r.json(); })
                .then(function (data) {
                  if (data.success) {
                    insistBtn.textContent = 'Escalated to CEO';
                    insistBtn.style.borderColor = '#059669';
                    insistBtn.style.color = '#059669';
                  } else {
                    insistBtn.disabled = false;
                    insistBtn.textContent = 'I insist — escalate this';
                  }
                })
                .catch(function () {
                  insistBtn.disabled = false;
                  insistBtn.textContent = 'I insist — escalate this';
                });
            };
            el.appendChild(insistBtn);
          }

          body.appendChild(el);
        });
      })
      .catch(function () {
        body.innerHTML = '<div class="thriftly-fb-empty">Failed to load. Try again.</div>';
      });
  }
})();
