// n_newform.js - okno "Nowy formularz" na pulpicie (tmpl/n_home.html).
// Kopia istniejącego formularza jest domyślna: formularze powtarzają się co
// roku. Adres /admin#newform=ID otwiera okno z kopią formularza ID
// (przycisk "Kopiuj formularz" na stronie formularza).

(function () {
  'use strict';

  var dlg = document.getElementById('nf-dlg');
  if (!dlg) return;
  var t = RZ.t;
  var name = document.getElementById('nf-name');
  var url = document.getElementById('nf-url');
  var urlFull = document.getElementById('nf-url-full');
  var from = document.getElementById('nf-from');
  var lang = document.getElementById('nf-lang');
  var msg = document.getElementById('nf-msg');
  var ok = document.getElementById('nf-ok');
  var urlEdited = false, base = '', timer = null;

  function nextName(s) {
    // "Bal 2026" -> "Bal 2027"
    var n = String(s).replace(/(\d{4})/, function (y) { return String(Number(y) + 1); });
    return n === s ? t('formpage.copy_name', s) : n;
  }

  function start() {
    var r = dlg.querySelector('input[name="nf-start"]:checked');
    return r ? r.value : 'empty';
  }

  function suggest() {
    clearTimeout(timer);
    timer = setTimeout(function () {
      fetch('/api/forms/url?name=' + encodeURIComponent(name.value))
        .then(function (r) { return r.json(); })
        .then(function (d) {
          base = d.base;
          if (!urlEdited) url.value = d.url;
          showURL();
        })
        .catch(function () { /* podpowiedź nie jest konieczna */ });
    }, 250);
  }

  function showURL() { urlFull.textContent = base + url.value; }

  function open(copyID) {
    msg.textContent = '';
    urlEdited = false;
    if (from && copyID) {
      from.value = String(copyID);
      var r = dlg.querySelector('input[value="copy"]');
      if (r) r.checked = true;
    }
    name.value = start() === 'copy' && from ? nextName(from.options[from.selectedIndex].text) : '';
    dlg.hidden = false;
    name.focus();
    name.select();
    suggest();
  }

  function close() { dlg.hidden = true; }

  document.getElementById('new-form').addEventListener('click', function () { open(0); });
  document.getElementById('nf-cancel').addEventListener('click', close);
  dlg.addEventListener('click', function (e) { if (e.target === dlg) close(); });
  dlg.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter' && e.target.tagName === 'INPUT' && e.target.type === 'text') ok.click();
  });
  name.addEventListener('input', suggest);
  url.addEventListener('input', function () { urlEdited = true; showURL(); });

  dlg.addEventListener('change', function (e) {
    if (e.target === from || (e.target.name === 'nf-start' && e.target.value === 'copy')) {
      if (from) {
        name.value = nextName(from.options[from.selectedIndex].text);
        suggest();
      }
    }
    if (from) from.disabled = start() !== 'copy';
  });

  ok.addEventListener('click', function () {
    ok.disabled = true;
    msg.textContent = '';
    RZ.postJSON('/api/forms', {
      name: name.value,
      url: url.value.trim(),
      language: lang.value,
      from_id: start() === 'copy' && from ? parseInt(from.value, 10) : 0,
      preset: start() === 'person' ? 'person' : ''
    }).then(function (resp) {
      location.href = resp.next;
    }).catch(function (err) {
      msg.textContent = err.message;
      msg.classList.add('bad');
      ok.disabled = false;
    });
  });

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    var m = /newform=(\d+)/.exec(location.hash);
    if (m) {
      history.replaceState(null, '', location.pathname);
      open(+m[1]);
    }
  });
})();
