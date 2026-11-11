// Zapamiętane osoby (contacts.go, tmpl/n_contacts.html): kliknięcie w wiersz
// otwiera okno z imieniem, nazwiskiem i e-mailem; usuwanie przez wspólne
// okno potwierdzenia (data-delete w n_ui.js).
(function () {
  'use strict';

  var dlg = document.getElementById('ct-dlg');
  if (!dlg) return;
  var form = document.getElementById('ct-form');
  var msg = form.querySelector('.msg');
  var del = document.getElementById('ct-delete');
  var save = form.querySelector('[type=submit]');
  var id = 0;

  function open(row) {
    id = Number(row.dataset.id);
    form.elements.name.value = row.dataset.name;
    form.elements.surname.value = row.dataset.surname;
    form.elements.email.value = row.dataset.email;
    msg.textContent = '';
    msg.classList.remove('bad');
    save.disabled = false;
    del.dataset.delete = '/api/contacts/' + id;
    del.dataset.text = RZ.t('contacts.del_text', [row.dataset.name + ' ' + row.dataset.surname, row.dataset.email]);
    dlg.hidden = false;
    form.elements.name.focus();
  }

  function close() {
    dlg.hidden = true;
    id = 0;
  }

  document.querySelectorAll('.contacts a.row').forEach(function (row) {
    row.addEventListener('click', function (e) { e.preventDefault(); open(row); });
  });
  document.getElementById('ct-cancel').addEventListener('click', close);
  dlg.addEventListener('click', function (e) { if (e.target === dlg) close(); });
  dlg.addEventListener('keydown', function (e) { if (e.key === 'Escape') close(); });

  form.addEventListener('submit', function (e) {
    e.preventDefault();
    save.disabled = true;
    msg.classList.remove('bad');
    msg.textContent = '';
    RZ.postJSON('/api/contacts/' + id, {
      name: form.elements.name.value,
      surname: form.elements.surname.value,
      email: form.elements.email.value
    }).then(function (resp) {
      var text = resp.forms ? RZ.tN('contacts.saved', resp.forms) : RZ.t('common.saved');
      if (resp.skipped && resp.skipped.length) {
        text += ' ' + RZ.t('contacts.skipped', resp.skipped.join(', '));
      }
      try { sessionStorage.setItem('rezerwo.contacts.msg', text); } catch (err) { /* nic */ }
      location.reload();
    }).catch(function (err) {
      save.disabled = false;
      msg.textContent = err.message;
      msg.classList.add('bad');
      var f = form.elements[err.field];
      if (f) f.focus();
    });
  });

  // komunikat po przeładowaniu (zmiana mogła poprawić zgłoszenia)
  var after = null;
  try {
    after = sessionStorage.getItem('rezerwo.contacts.msg');
    sessionStorage.removeItem('rezerwo.contacts.msg');
  } catch (err) { /* nic */ }
  if (after) {
    RZ.ready.then(function () {
      RZ.toast(after);
      var h = document.getElementById('contacts');
      if (h) h.scrollIntoView({block: 'start'});
    });
  }
})();
