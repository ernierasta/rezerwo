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

  // niezapisane zmiany trzymamy jako szkic w przeglądarce, żeby dało się
  // wyjść na inną stronę i wrócić do tych samych wartości
  var drafts = RZ.draft('rezerwo.draft:' + url);

  function stash() { drafts.save('page', values()); }

  function applyDraft(v) {
    if (!v) return false;
    page.querySelectorAll('[data-field]').forEach(function (el) {
      if (v[el.dataset.field] === undefined) return;
      if (el.type === 'checkbox') el.checked = !!v[el.dataset.field];
      else el.value = v[el.dataset.field];
    });
    if (editor && v.text !== undefined) editor.clipboard.dangerouslyPasteHTML(v.text);
    return true;
  }

  page.addEventListener('input', stash);
  page.addEventListener('change', stash);
  if (editor) editor.on('text-change', stash);

  RZ.ready.then(function () {
    if (applyDraft(drafts.get('page'))) {
      RZ.draftNotice(function () {
        drafts.clearAll();
        location.reload();
      });
    }
  });

  document.getElementById('save').addEventListener('click', function () {
    var btn = this;
    var msg = page.querySelector('.sec-f .msg');
    btn.disabled = true;
    if (msg) { msg.textContent = RZ.t('common.loading'); msg.classList.remove('bad'); }

    RZ.postJSON(url, values())
      .then(function (resp) {
        drafts.clearAll();
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
})();
