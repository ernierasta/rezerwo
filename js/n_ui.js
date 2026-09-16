// n_ui.js - drobne narzędzia nowego panelu: teksty, powiadomienia, harmonijka.
// Bez jQuery i bez Bootstrapa.

(function (global) {
  'use strict';

  var dict = {};

  // t("klucz") - tekst interfejsu z katalogu wczytanego z /locales/{lang}.json.
  function t(key, args) {
    var s = dict[key];
    if (s === undefined) return key;
    if (args === undefined) return s;
    if (!Array.isArray(args)) args = [args];
    var i = 0;
    return s.replace(/%[ds]/g, function () { return args[i++]; });
  }

  function loadLocale(lang) {
    return fetch('/locales/' + lang + '.json')
      .then(function (r) { return r.ok ? r.json() : {}; })
      .then(function (d) { dict = d; return d; })
      .catch(function () { return {}; });
  }

  // toast("Zapisano") albo toast("Oznaczono 3", {label: "Cofnij", action: fn})
  var toastEl = null, toastTimer = null;
  function toast(text, undo) {
    if (toastEl) toastEl.remove();
    toastEl = document.createElement('div');
    toastEl.className = 'toast';
    var span = document.createElement('span');
    span.textContent = text;
    toastEl.appendChild(span);
    if (undo && undo.action) {
      var b = document.createElement('button');
      b.textContent = undo.label || 'Cofnij';
      b.addEventListener('click', function () { undo.action(); hideToast(); });
      toastEl.appendChild(b);
    }
    document.body.appendChild(toastEl);
    clearTimeout(toastTimer);
    toastTimer = setTimeout(hideToast, undo ? 8000 : 3000);
  }

  function hideToast() {
    if (toastEl) { toastEl.remove(); toastEl = null; }
  }

  // postJSON zwraca obiekt odpowiedzi; rzuca błąd z komunikatem serwera.
  function postJSON(url, data, method) {
    return fetch(url, {
      method: method || 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(data)
    }).then(function (r) {
      return r.text().then(function (txt) {
        var body = {};
        try { body = txt ? JSON.parse(txt) : {}; } catch (e) { body = {msg: txt}; }
        if (!r.ok) throw new Error(body.msg || ('HTTP ' + r.status));
        return body;
      });
    });
  }

  // Harmonijka: rozwijanie sekcji, zapamiętany stan, adres z #sekcją.
  function accordion(root, storageKey) {
    var open = {};
    try { open = JSON.parse(localStorage.getItem(storageKey) || '{}') || {}; } catch (e) { open = {}; }

    function store() {
      try { localStorage.setItem(storageKey, JSON.stringify(open)); } catch (e) { /* prywatne okno */ }
    }

    function setOpen(sec, isOpen) {
      sec.classList.toggle('open', isOpen);
      open[sec.dataset.sec] = isOpen;
      var act = sec.querySelector('.sec-h .act');
      if (act) act.textContent = isOpen ? t('common.collapse') : (act.dataset.label || act.textContent);
      var body = sec.querySelector('.sec-b');
      if (body) body.hidden = !isOpen;
      if (isOpen) sec.dispatchEvent(new CustomEvent('sec:open', {bubbles: true}));
      store();
    }

    root.querySelectorAll('.sec').forEach(function (sec) {
      var head = sec.querySelector('.sec-h');
      var act = sec.querySelector('.sec-h .act');
      if (act && !act.dataset.label) act.dataset.label = act.textContent;
      if (!sec.querySelector('.sec-b')) return; // sekcja tylko z linkiem
      head.addEventListener('click', function (e) {
        if (e.target.tagName === 'A' && e.target.getAttribute('href') && !e.target.classList.contains('act')) return;
        e.preventDefault();
        setOpen(sec, !sec.classList.contains('open'));
      });
      setOpen(sec, !!open[sec.dataset.sec]);
    });

    var hash = location.hash.replace('#', '');
    if (hash) {
      var target = root.querySelector('.sec[data-sec="' + hash + '"]');
      if (target) {
        setOpen(target, true);
        target.scrollIntoView({block: 'start'});
      }
    }
  }

  global.RZ = {
    t: t,
    loadLocale: loadLocale,
    toast: toast,
    postJSON: postJSON,
    accordion: accordion
  };
})(window);
