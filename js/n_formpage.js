// n_formpage.js - formularz dla gościa (tmpl/n_formpage.html).
// Pola renderuje serwer; tu jest tylko: mnożnik i suma do zapłaty, opcja
// "Inne", walidacja z komunikatami w języku formularza, szkic w przeglądarce
// i wysyłka. Format wysyłanych danych jak w starym formRender:
// [{name: "pole", userData: ["wartość", ...]}].

(function () {
  'use strict';

  var root = document.getElementById('gform');
  var form = document.getElementById('gform-form');
  if (!root || !form) return;

  var t = RZ.t;
  var fields = Array.prototype.slice.call(form.querySelectorAll('.ff[data-name]'));
  var submit = document.getElementById('gform-submit');
  var msgEl = document.getElementById('gform-msg');
  var totalEl = document.getElementById('gform-total');
  var currency = root.dataset.currency || '';
  var draftKey = root.dataset.draft;
  var sent = false;

  // --- pomocnicze --------------------------------------------------------

  function inputs(ff) {
    return Array.prototype.slice.call(ff.querySelectorAll('input:not(.other-text), select, textarea'));
  }

  function money(n) {
    var s = (Math.round(n * 100) / 100).toLocaleString(root.dataset.lang || 'pl');
    return currency ? s + ' ' + currencySign(currency) : s;
  }

  function currencySign(c) {
    return {CZK: 'Kč', EUR: '€', PLN: 'zł'}[c] || c;
  }

  // --- mnożnik i suma ----------------------------------------------------

  function multValue(inp) {
    var n = Number(inp.value) || 0;
    var price = Number(inp.dataset.price) || 1;
    return {n: n, price: price, total: n * price};
  }

  function updateMult(ff) {
    var inp = ff.querySelector('input');
    var out = ff.querySelector('.mult-res b');
    if (inp && out) out.textContent = String(multValue(inp).total);
  }

  function updateTotal() {
    if (!totalEl) return;
    var sum = 0;
    fields.forEach(function (ff) {
      if (!ff.classList.contains('money')) return;
      var inp = ff.querySelector('input');
      if (!inp) return;
      if (ff.dataset.type === 'multiplierField') sum += multValue(inp).total;
      else sum += Number(inp.value) || 0;
    });
    totalEl.textContent = money(sum);
    totalEl.closest('.gform-sum').classList.toggle('zero', sum === 0);
  }

  // --- opcja "Inne" ------------------------------------------------------

  // fromText: zmiana przyszła z pola tekstowego - wpisanie czegoś zaznacza "Inne",
  // bo goście klikają od razu w pole, a nie w kwadracik obok
  function syncOther(ff, fromText) {
    var other = ff.querySelector('[data-other]');
    if (!other) return;
    var txt = ff.querySelector('.other-text');
    if (fromText && txt.value.trim() !== '' && !other.checked) other.checked = true;
    txt.classList.toggle('off', !other.checked);
    other.value = txt.value.trim();
    if (other.checked && document.activeElement === other) txt.focus();
  }

  // --- zbieranie odpowiedzi ----------------------------------------------

  function valueOf(ff) {
    var type = ff.dataset.type;
    if (type === 'checkbox-group' || type === 'radio-group') {
      return inputs(ff).filter(function (i) { return i.checked && i.value !== ''; })
        .map(function (i) { return i.value; });
    }
    var el = ff.querySelector('input:not(.other-text), select, textarea');
    if (!el) return [];
    if (type === 'multiplierField') {
      if (el.value === '') return [''];
      var m = multValue(el);
      return [m.n + ' * ' + m.price + ' = ' + m.total];
    }
    if (el.multiple) {
      return Array.prototype.slice.call(el.selectedOptions).map(function (o) { return o.value; });
    }
    if (el.hasAttribute('data-date')) {
      // zapis zawsze jako dd.mm.rrrr, niezależnie od tego, jak gość wpisał
      var dt = RZ.parseDate(el.value);
      return [dt ? RZ.fmtDate(dt) : el.value.trim()];
    }
    return [el.value.trim()];
  }

  function collect() {
    return fields.map(function (ff) {
      return {name: ff.dataset.name, userData: valueOf(ff)};
    });
  }

  // --- walidacja ---------------------------------------------------------

  function errorOf(ff) {
    var type = ff.dataset.type;
    var required = ff.hasAttribute('data-required');

    if (type === 'checkbox-group' || type === 'radio-group') {
      var other = ff.querySelector('[data-other]');
      if (other && other.checked && other.value === '') return t('gform.err.other');
      if (required && valueOf(ff).length === 0) return t('gform.err.choose');
      return '';
    }

    var el = ff.querySelector('input, select, textarea');
    if (!el) return '';
    if (required && el.value.trim() === '') {
      return el.tagName === 'SELECT' ? t('gform.err.choose') : t('gform.err.required');
    }
    return RZ.inputError(el);
  }

  function showError(ff, text) {
    var err = ff.querySelector('.ff-err');
    ff.classList.toggle('bad', !!text);
    if (!err) return;
    err.textContent = text;
    err.hidden = !text;
    inputs(ff).forEach(function (i) {
      if (text) i.setAttribute('aria-invalid', 'true');
      else i.removeAttribute('aria-invalid');
    });
  }

  function validate() {
    var firstBad = null;
    fields.forEach(function (ff) {
      var e = errorOf(ff);
      showError(ff, e);
      if (e && !firstBad) firstBad = ff;
    });
    if (firstBad) {
      firstBad.scrollIntoView({behavior: 'smooth', block: 'center'});
      var focusEl = firstBad.querySelector('input, select, textarea');
      if (focusEl) setTimeout(function () { focusEl.focus({preventScroll: true}); }, 250);
    }
    return !firstBad;
  }

  // --- szkic: wpisane dane przeżywają przeładowanie strony ----------------

  function saveDraft() {
    if (!draftKey || sent) return;
    var d = {};
    fields.forEach(function (ff) {
      var o = ff.querySelector('.other-text');
      d[ff.dataset.name] = {v: valueOf(ff), o: o ? o.value : undefined};
    });
    try { localStorage.setItem(draftKey, JSON.stringify(d)); } catch (e) { /* prywatne okno */ }
  }

  function clearDraft() {
    try { localStorage.removeItem(draftKey); } catch (e) { /* nic */ }
  }

  function restoreDraft() {
    var d;
    try { d = JSON.parse(localStorage.getItem(draftKey) || 'null'); } catch (e) { d = null; }
    if (!d) return;
    fields.forEach(function (ff) {
      var saved = d[ff.dataset.name];
      if (!saved) return;
      var vals = saved.v || [];
      var type = ff.dataset.type;
      var other = ff.querySelector('.other-text');
      if (other && saved.o !== undefined) other.value = saved.o;
      if (type === 'checkbox-group' || type === 'radio-group') {
        inputs(ff).forEach(function (i) {
          i.checked = i.hasAttribute('data-other') ? (saved.o && vals.indexOf(saved.o) >= 0) : vals.indexOf(i.value) >= 0;
        });
        syncOther(ff);
        return;
      }
      var el = ff.querySelector('input, select, textarea');
      if (!el) return;
      if (type === 'multiplierField') {
        var m = /^(\S+) \*/.exec(vals[0] || '');
        el.value = m ? m[1] : '';
        updateMult(ff);
        return;
      }
      if (el.multiple) {
        Array.prototype.forEach.call(el.options, function (o) { o.selected = vals.indexOf(o.value) >= 0; });
        return;
      }
      if (vals[0] !== undefined) el.value = vals[0];
    });
  }

  // --- zdarzenia ---------------------------------------------------------

  form.addEventListener('input', function (e) {
    var ff = e.target.closest('.ff');
    if (!ff) return;
    if (ff.dataset.type === 'multiplierField') updateMult(ff);
    if (e.target.classList.contains('other-text')) syncOther(ff, true);
    if (ff.classList.contains('bad')) showError(ff, errorOf(ff));
    if (!form.querySelector('.ff.bad')) msgEl.hidden = true;
    updateTotal();
    saveDraft();
  });

  form.addEventListener('change', function (e) {
    var ff = e.target.closest('.ff');
    if (!ff) return;
    syncOther(ff);
    if (ff.classList.contains('bad')) showError(ff, errorOf(ff));
    saveDraft();
  });

  // błąd pokazujemy po wyjściu z pola, nie w trakcie pisania
  form.addEventListener('focusout', function (e) {
    var ff = e.target.closest('.ff');
    if (!ff || ff.contains(e.relatedTarget)) return;
    var inp = e.target;
    if (inp.value !== '' || ff.classList.contains('bad')) showError(ff, errorOf(ff));
  });

  // domyślne "0" w polu liczbowym: wpisana liczba zastępuje zero, a nie daje "025"
  form.addEventListener('focusin', function (e) {
    var el = e.target;
    if (el.type === 'number' && el.value === '0') setTimeout(function () { el.select(); }, 0);
  });

  form.addEventListener('submit', function (e) {
    e.preventDefault();
    msgEl.hidden = true;
    fields.forEach(syncOther);
    if (!validate()) {
      msgEl.textContent = t('gform.err.fix');
      msgEl.hidden = false;
      return;
    }

    var uniq = document.getElementById('uniqid');
    submit.disabled = true;
    var label = submit.textContent;
    submit.textContent = t('gform.sending');

    RZ.postJSON(root.dataset.send, {uniqid: uniq ? uniq.textContent : '', data: collect()})
      .then(function (resp) {
        sent = true;
        clearDraft();
        location.href = resp.done;
      })
      .catch(function (err) {
        msgEl.textContent = err.message || t('gform.save_error');
        msgEl.hidden = false;
        submit.disabled = false;
        submit.textContent = label;
      });
  });

  // Bez ostrzeżenia "Opuścić stronę?" (beforeunload): ten dialog rysuje
  // przeglądarka w swoim języku, a wpisane dane i tak przeżywają w szkicu.
  RZ.noNativeTips(form);

  // panel informacyjny na telefonie zwinięty, żeby formularz był od razu widać
  var info = root.querySelector('.gform-info');
  if (info && window.matchMedia('(max-width: 860px)').matches) info.open = false;

  RZ.loadLocale(root.dataset.lang || 'pl').then(function () {
    restoreDraft();
    fields.forEach(function (ff) {
      if (ff.dataset.type === 'multiplierField') updateMult(ff);
    });
    updateTotal();
  });
})();
