// n_formpage.js - formularz dla gościa (tmpl/n_formpage.html).
// Pola renderuje serwer; tu jest tylko: mnożnik i suma do zapłaty, opcja
// "Inne", pola warunkowe (n_formconds.js), walidacja z komunikatami w języku
// formularza, szkic w przeglądarce i wysyłka. Ukryte pola idą puste. Format wysyłanych danych jak w starym formRender:
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
  var conds = null;

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
      if (!ff.classList.contains('money') || ff.hidden) return;
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
      return {name: ff.dataset.name, userData: ff.hidden ? [] : valueOf(ff)};
    });
  }

  // --- walidacja ---------------------------------------------------------

  function errorOf(ff) {
    if (ff.hidden) return '';
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

  // setField wpisuje zapisaną wartość {v: [...], o: "tekst Inne"} do pola
  function setField(ff, saved) {
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
  }

  // fillAll wpisuje zapisane wartości {nazwa: {v, o}}. Opcje pola zależnego
  // pojawiają się dopiero po wyborze rodzica, więc wpisujemy kilka razy.
  function fillAll(d) {
    for (var k = 0; k < 3; k++) {
      fields.forEach(function (ff) {
        if (d[ff.dataset.name]) setField(ff, d[ff.dataset.name]);
      });
      if (!conds.apply()) break;
    }
  }

  // restoreDraft oddaje true, jeśli był szkic tego formularza
  function restoreDraft() {
    var d;
    try { d = JSON.parse(localStorage.getItem(draftKey) || 'null'); } catch (e) { d = null; }
    if (!d) return false;
    fillAll(d);
    return true;
  }

  // --- zapamiętane dane gościa ---------------------------------------------
  // Formularz idzie fetch-em, więc przeglądarka nie zapamiętuje wpisów sama.
  // Po wysłaniu zapisujemy powtarzające się pola (imię, nazwisko, e-mail,
  // telefon, najstarsze dziecko, klasa) i wpisujemy je w następny formularz
  // tej samej organizacji. Pola rozpoznajemy po nazwie i etykiecie, bo
  // w różnych formularzach nazywają się różnie ("dziecko", "imieucznia", ...).

  var rememberKey = 'rezerwo.gform.remember.' + (location.pathname.split('/')[2] || '');

  function fieldKind(ff) {
    var name = ff.dataset.name, type = ff.dataset.type;
    if (name === 'name-REQUIRED') return 'name';
    if (name === 'surname-REQUIRED') return 'surname';
    if (name === 'email-REQUIRED') return 'email';
    var l = ff.querySelector('.ff-l');
    var label = l ? l.textContent.replace('*', '').trim().toLowerCase() : '';
    if (/^klas/.test(label)) return 'class';
    if (type !== 'text') return '';
    var el = ff.querySelector('input');
    if (el && el.type === 'tel' || /telefon/.test(label)) return 'phone';
    if (el && el.type === 'email') return 'email';
    if (/najst(arsz|\.)/.test(label) && /(dzieck|ucznia)/.test(label)) return 'child';
    return '';
  }

  function remember() {
    var m;
    try { m = JSON.parse(localStorage.getItem(rememberKey) || '{}') || {}; } catch (e) { return; }
    fields.forEach(function (ff) {
      var k = fieldKind(ff);
      if (!k) return;
      var v = valueOf(ff);
      if (v.length === 0 || v[0] === '') return;
      var o = ff.querySelector('.other-text');
      m[k] = {v: v, o: o ? o.value : undefined};
    });
    try { localStorage.setItem(rememberKey, JSON.stringify(m)); } catch (e) { /* prywatne okno */ }
  }

  function recall() {
    var m;
    try { m = JSON.parse(localStorage.getItem(rememberKey) || 'null'); } catch (e) { m = null; }
    if (!m) return;
    var filled = [];
    fields.forEach(function (ff) {
      var k = fieldKind(ff);
      if (!k || !m[k]) return;
      var sel = ff.querySelector('select');
      if (sel) {
        // lista często ma wybraną z góry opcję "--Wybierz--", więc nie pytamy
        // o pustość, tylko czy zapamiętana wartość jest wśród opcji
        var vals = m[k].v || [];
        var has = Array.prototype.some.call(sel.options, function (o) { return o.value !== '' && vals.indexOf(o.value) >= 0; });
        if (!has) return;
      } else if (valueOf(ff).some(function (v) { return v !== ''; })) {
        return;
      }
      setField(ff, m[k]);
      if (valueOf(ff).some(function (v) { return v !== ''; })) filled.push(ff);
    });
    conds.apply();
    var note = document.getElementById('gform-remembered');
    if (!filled.length || !note) return;
    note.hidden = false;
    document.getElementById('gform-remembered-clear').addEventListener('click', function () {
      filled.forEach(function (ff) { setField(ff, {v: [''], o: ''}); });
      conds.apply();
      updateTotal();
      try { localStorage.removeItem(rememberKey); } catch (e) { /* nic */ }
      note.hidden = true;
      saveDraft();
    });
  }

  // --- zdarzenia ---------------------------------------------------------

  // applyConds przelicza pola warunkowe; ukryte pole nie pokazuje błędu
  function applyConds() {
    var changed = conds.apply();
    if (changed) {
      fields.forEach(function (ff) { if (ff.hidden && ff.classList.contains('bad')) showError(ff, ''); });
    }
    return changed;
  }

  form.addEventListener('input', function (e) {
    var ff = e.target.closest('.ff');
    if (!ff) return;
    if (ff.dataset.type === 'multiplierField') updateMult(ff);
    if (e.target.classList.contains('other-text')) syncOther(ff, true);
    applyConds();
    if (ff.classList.contains('bad')) showError(ff, errorOf(ff));
    if (!form.querySelector('.ff.bad')) msgEl.hidden = true;
    updateTotal();
    saveDraft();
  });

  form.addEventListener('change', function (e) {
    var ff = e.target.closest('.ff');
    if (!ff) return;
    syncOther(ff);
    if (applyConds()) updateTotal();
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

    send(false);
  });

  // --- wysyłka; ten sam e-mail = poprawka własnego zgłoszenia ---------------

  var dlgDiff = document.getElementById('gform-resubmit');
  var dlgSame = document.getElementById('gform-same');

  function send(confirm) {
    var uniq = document.getElementById('uniqid');
    submit.disabled = true;
    var label = submit.textContent;
    submit.textContent = t('gform.sending');
    var done = function () {
      submit.disabled = false;
      submit.textContent = label;
    };

    RZ.postJSON(root.dataset.send, {uniqid: uniq ? uniq.textContent : '', data: collect(), confirm: confirm})
      .then(function (resp) {
        if (resp.confirm) {
          done();
          showDiff(resp.changes || []);
          return;
        }
        if (resp.same) {
          done();
          document.getElementById('gform-same-show').href = resp.done;
          dlgSame.hidden = false;
          return;
        }
        sent = true;
        clearDraft();
        remember();
        location.href = resp.done;
      })
      .catch(function (err) {
        done();
        var ff = err.field && form.querySelector('.ff[data-name="' + err.field + '"]');
        if (ff) {
          showError(ff, err.message);
          ff.scrollIntoView({behavior: 'smooth', block: 'center'});
        }
        msgEl.textContent = err.message || t('gform.save_error');
        msgEl.hidden = false;
      });
  }

  function cell(text, cls) {
    var td = document.createElement('td');
    if (text === '') {
      td.textContent = t('gform.resubmit_empty');
      td.className = 'empty';
    } else {
      td.textContent = text;
      if (cls) td.className = cls;
    }
    return td;
  }

  function showDiff(changes) {
    var body = document.getElementById('gform-diff-rows');
    body.textContent = '';
    changes.forEach(function (c) {
      var tr = document.createElement('tr');
      tr.appendChild(cell(c.label));
      tr.appendChild(cell(c.old, 'old'));
      tr.appendChild(cell(c.new, 'new'));
      body.appendChild(tr);
    });
    dlgDiff.hidden = false;
    document.getElementById('gform-resubmit-save').focus();
  }

  if (dlgDiff) {
    document.getElementById('gform-resubmit-save').addEventListener('click', function () {
      dlgDiff.hidden = true;
      send(true);
    });
    document.getElementById('gform-resubmit-back').addEventListener('click', function () { dlgDiff.hidden = true; });
    document.getElementById('gform-same-back').addEventListener('click', function () { dlgSame.hidden = true; });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { dlgDiff.hidden = true; dlgSame.hidden = true; }
    });
  }

  // pola warunkowe: od razu, żeby ukryte pola nie mignęły przed wczytaniem tekstów
  conds = RZ.formConds(form, valueOf);

  // Bez ostrzeżenia "Opuścić stronę?" (beforeunload): ten dialog rysuje
  // przeglądarka w swoim języku, a wpisane dane i tak przeżywają w szkicu.
  RZ.noNativeTips(form);

  // panel informacyjny na telefonie zwinięty, żeby formularz był od razu widać
  var info = root.querySelector('.gform-info');
  if (info && window.matchMedia('(max-width: 860px)').matches) info.open = false;

  RZ.loadLocale(root.dataset.lang || 'pl').then(function () {
    if (!restoreDraft()) recall();
    fields.forEach(function (ff) {
      if (ff.dataset.type === 'multiplierField') updateMult(ff);
    });
    updateTotal();
  });
})();
