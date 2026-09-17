// n_mail.js - zapis edytora maila i konta bankowego.
// Obie strony mają ten sam wzór: pola z data-field, jeden przycisk "Zapisz",
// treść maila w Quillu.

(function () {
  'use strict';

  var mailPage = document.getElementById('mail-page');
  var accountPage = document.getElementById('account-page');
  var page = mailPage || accountPage;
  if (!page) return;

  var url = mailPage
    ? '/api/mails/' + mailPage.dataset.mailId
    : '/api/accounts/' + accountPage.dataset.accountId;

  var editor = null;
  var editorEl = document.getElementById('mail-editor');
  if (editorEl && typeof Quill !== 'undefined') {
    editor = new Quill(editorEl, {
      modules: {
        toolbar: [
          [{header: [1, 2, 3, false]}],
          ['bold', 'italic', 'underline'],
          ['link'],
          [{list: 'ordered'}, {list: 'bullet'}],
          [{indent: '-1'}, {indent: '+1'}],
          [{color: ['#28a745', '#ffc107', '#fd7e14', '#dc3545', '#403734', '#000000']}],
          [{align: []}],
          ['clean']
        ],
        clipboard: {matchVisual: false}
      },
      theme: 'snow'
    });
  }

  function values() {
    var out = {};
    page.querySelectorAll('[data-field]').forEach(function (el) {
      out[el.dataset.field] = el.type === 'checkbox' ? el.checked : el.value;
    });
    if (editor) out.text = editor.root.innerHTML;
    return out;
  }

  var dirty = false;
  page.addEventListener('input', function () { dirty = true; });
  page.addEventListener('change', function () { dirty = true; });
  if (editor) editor.on('text-change', function () { dirty = true; });

  document.getElementById('save').addEventListener('click', function () {
    var btn = this;
    var msg = page.querySelector('.sec-f .msg');
    btn.disabled = true;
    if (msg) { msg.textContent = RZ.t('common.loading'); msg.classList.remove('bad'); }

    RZ.postJSON(url, values())
      .then(function (resp) {
        dirty = false;
        if (msg) msg.textContent = resp.warning || RZ.t('common.saved');
        if (msg && resp.warning) msg.classList.add('bad');
        RZ.toast(RZ.t('common.saved'));
        if (resp.title) {
          var h = document.querySelector('.dh h3');
          if (h) h.textContent = resp.title;
        }
      })
      .catch(function (err) {
        if (msg) { msg.textContent = RZ.t('common.save_error') + ': ' + err.message; msg.classList.add('bad'); }
      })
      .finally(function () { btn.disabled = false; });
  });

  window.addEventListener('beforeunload', function (e) {
    if (!dirty) return;
    e.preventDefault();
    e.returnValue = '';
  });
})();
