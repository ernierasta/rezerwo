// Loginy menedżerów (/admin/managers, managers_ui.go): jeden formularz do
// dodawania i zmiany, otwierany przyciskiem "Dodaj" albo kliknięciem w wiersz.
(function () {
  'use strict';

  var form = document.getElementById('manager-form');
  if (!form) return;
  var email = form.elements.email;
  var notes = form.elements.notes;
  var pass = form.elements.password;
  var msg = form.querySelector('.msg');
  var del = document.getElementById('m-delete');
  var id = 0;

  function setMode(edit) {
    form.querySelectorAll('[data-new]').forEach(function (el) {
      el.textContent = edit ? el.dataset.edit : el.dataset.new;
    });
    del.hidden = !edit;
  }

  function open(row) {
    id = row ? Number(row.dataset.id) : 0;
    email.value = row ? row.dataset.email : '';
    notes.value = row ? row.dataset.notes : '';
    pass.value = '';
    msg.textContent = '';
    msg.classList.remove('bad');
    setMode(!!row);
    if (row) {
      del.dataset.delete = '/api/managers/' + id;
      del.dataset.text = RZ.t('managers.del_text', row.dataset.email);
    }
    document.querySelectorAll('.managers .row.on').forEach(function (r) { r.classList.remove('on'); });
    if (row) row.classList.add('on');
    form.hidden = false;
    form.scrollIntoView({block: 'nearest', behavior: 'smooth'});
    email.focus();
  }

  function close() {
    form.hidden = true;
    id = 0;
    document.querySelectorAll('.managers .row.on').forEach(function (r) { r.classList.remove('on'); });
  }

  // hasło do przekazania menedżerowi: 12 znaków bez mylących (0/O, 1/l/I)
  function generate() {
    var chars = 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    var buf = new Uint32Array(12);
    crypto.getRandomValues(buf);
    var s = '';
    for (var i = 0; i < buf.length; i++) s += chars[buf[i] % chars.length];
    pass.value = s;
    pass.select();
  }

  var add = document.getElementById('new-manager');
  if (add) add.addEventListener('click', function () { open(null); });
  document.querySelectorAll('.managers a.row').forEach(function (row) {
    row.addEventListener('click', function (e) { e.preventDefault(); open(row); });
  });
  document.getElementById('m-cancel').addEventListener('click', close);
  document.getElementById('m-generate').addEventListener('click', generate);

  form.addEventListener('submit', function (e) {
    e.preventDefault();
    var btn = form.querySelector('[type=submit]');
    btn.disabled = true;
    msg.classList.remove('bad');
    msg.textContent = '';
    var url = id ? '/api/managers/' + id : '/api/managers';
    RZ.postJSON(url, {email: email.value, notes: notes.value, password: pass.value})
      .then(function () {
        location.reload();
      })
      .catch(function (err) {
        btn.disabled = false;
        msg.textContent = err.message;
        msg.classList.add('bad');
        if (err.field && form.elements[err.field]) form.elements[err.field].focus();
      });
  });
})();
