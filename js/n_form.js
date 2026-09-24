// n_form.js - strona formularza w panelu (tmpl/n_form.html): harmonijka,
// status otwarty/zamknięty, kopiowanie linku, generator ✨ panelu
// informacyjnego i kasowanie formularza.

document.addEventListener('DOMContentLoaded', function () {
  'use strict';

  var page = document.getElementById('form-page');
  var formID = page.dataset.formId;
  var t = RZ.t;
  var secs = RZ.sections(page, '/api/forms/' + formID + '/section',
    {draftKey: 'rezerwo.form.' + formID + '.draft'});

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    var acc = RZ.accordion(page, 'rezerwo.form.' + formID + '.sections');
    secs.restore(acc); // niezapisane zmiany sprzed wyjścia ze strony
    syncStatus();
  });

  // przełącznik Otwarty / Zamknięty zapisuje się w ukrytym polu sekcji
  var status = document.getElementById('f-status');
  var seg = document.getElementById('status-seg');
  function syncStatus() {
    seg.querySelectorAll('button').forEach(function (b) {
      b.classList.toggle('on', b.dataset.v === (status.value || 'open'));
    });
  }
  seg.addEventListener('click', function (e) {
    var b = e.target.closest('button');
    if (!b) return;
    status.value = b.dataset.v;
    syncStatus();
    status.dispatchEvent(new Event('input', {bubbles: true}));
  });

  document.getElementById('copy-link').addEventListener('click', function () {
    var url = location.origin + document.getElementById('form-link').textContent.trim();
    if (navigator.clipboard) navigator.clipboard.writeText(url);
    RZ.toast(t('fields.copied'));
  });

  // ✨ - tabela z sumami odpowiedzi na każde pole (to samo API co stary edytor)
  var gen = document.getElementById('gen-info');
  if (gen) {
    gen.addEventListener('click', function () {
      var q = secs.editors.infopanel;
      if (!q) return;
      RZ.postJSON('/api/formdefs', {type: 'infopanel', id: parseInt(formID, 10)})
        .then(function (resp) {
          if (!resp.msg || !Array.isArray(resp.msg.ops)) throw new Error('bad response');
          var range = q.getSelection(true);
          var Delta = Quill.import('delta');
          q.updateContents(new Delta().retain(range ? range.index : q.getLength()).concat(new Delta(resp.msg.ops)), 'user');
        })
        .catch(function (err) { RZ.toast(t('common.save_error') + ': ' + err.message); });
    });
  }

  // kasowanie: trzeba przepisać nazwę formularza
  var dlg = document.getElementById('del-dlg');
  var name = document.getElementById('del-name');
  var ok = document.getElementById('del-ok');
  var formName = document.querySelector('.dh h3').textContent.trim();
  document.getElementById('form-delete').addEventListener('click', function () {
    dlg.hidden = false;
    name.value = '';
    ok.disabled = true;
    name.focus();
  });
  name.addEventListener('input', function () {
    ok.disabled = name.value.trim() !== formName;
  });
  document.getElementById('del-cancel').addEventListener('click', function () { dlg.hidden = true; });
  dlg.addEventListener('click', function (e) { if (e.target === dlg) dlg.hidden = true; });
  ok.addEventListener('click', function () {
    ok.disabled = true;
    RZ.postJSON('/api/forms/' + formID, {}, 'DELETE')
      .then(function (resp) {
        try { localStorage.removeItem('rezerwo.form.' + formID + '.draft'); } catch (e) { /* nic */ }
        location.href = resp.next || '/admin/start';
      })
      .catch(function (err) {
        dlg.querySelector('.msg').textContent = err.message;
        ok.disabled = false;
      });
  });
});
