// n_mail.js - zapis edytora maila i konta bankowego.
// Obie strony mają ten sam wzór: pola z data-field, jeden przycisk "Zapisz",
// treść maila w Quillu.
//
// Maile imprez: dane zamówienia są w edytorze "pastylkami" (blot mailvar),
// w temacie jako [Etykieta], a w bazie zostają akcje szablonu Go ({{.Name}},
// {{.QRPayMail "Konto"}}, {{.PDF "opis"}}), więc stare maile działają dalej.
// Zamiana w obie strony: toEditor() przy wczytaniu, fromEditor() przy zapisie.

(function () {
  'use strict';

  var mailPage = document.getElementById('mail-page');
  var accountPage = document.getElementById('account-page');
  var page = mailPage || accountPage;
  if (!page) return;

  var url = mailPage
    ? '/api/mails/' + mailPage.dataset.mailId
    : '/api/accounts/' + accountPage.dataset.accountId;

  var meta = {vars: [], accounts: []};
  if (mailPage && mailPage.dataset.vars) {
    try { meta = JSON.parse(mailPage.dataset.vars); } catch (e) { /* zostaje puste */ }
  }
  var V = RZ.vars; // pastylki danych zamówienia (n_vars.js)

  var relatedEl = document.getElementById('f-related');
  function isEvents() { return !relatedEl || relatedEl.value === 'events'; }

  var editor = null;
  var editorEl = document.getElementById('mail-editor');

  if (editorEl && typeof Quill !== 'undefined') {
    V.setup(meta, '.QRPayMail');
    editor = new Quill(editorEl, {
      modules: RZ.quillModules({
        container: [
          [{header: [1, 2, 3, false]}],
          ['bold', 'italic', 'underline'],
          ['link'],
          [{list: 'ordered'}, {list: 'bullet'}],
          [{indent: '-1'}, {indent: '+1'}],
          [{color: ['#28a745', '#ffc107', '#fd7e14', '#dc3545', '#403734', '#000000']}],
          [{align: []}],
          ['clean'],
          ['mailvar']
        ],
        handlers: {mailvar: V.toolbarHandler(function () { return editor; })}
      }),
      theme: 'snow'
    });
  }

  var varButton = editor ? V.attach(editor) : null;

  // --- temat: [Etykieta] <-> {{.Code}} --------------------------------------

  var subjectEl = document.getElementById('f-subject');

  function subjectToEditor(s) {
    if (!isEvents()) return s;
    return s.replace(/\{\{\s*(\.[A-Za-z]+)\s*\}\}/g, function (all, code) {
      var v = V.simpleVar(code);
      return v ? '[' + RZ.t(v.label) + ']' : all;
    });
  }

  function subjectFromEditor(s) {
    meta.vars.forEach(function (v) {
      if (v.account) return;
      s = s.split('[' + RZ.t(v.label) + ']').join('{{' + v.code + '}}');
    });
    return s;
  }

  // Kursor w temacie liczy się tylko wtedy, gdy admin w temat klikał;
  // inaczej znacznik dopisujemy na końcu.
  var subjectCaret = false;
  var subjectVar = document.getElementById('subject-var');
  if (subjectVar) {
    subjectEl.addEventListener('focus', function () { subjectCaret = true; });
    subjectVar.addEventListener('click', function () {
      var val0 = subjectEl.value;
      var start = subjectCaret ? subjectEl.selectionStart : val0.length;
      var end = subjectCaret ? subjectEl.selectionEnd : val0.length;
      V.openVarMenu(subjectVar, function (code) {
        var tag = '[' + RZ.t(V.simpleVar(code).label) + ']';
        var val = subjectEl.value;
        if (start > 0 && !/\s/.test(val.charAt(start - 1))) tag = ' ' + tag;
        if (end < val.length && !/\s/.test(val.charAt(end))) tag = tag + ' ';
        subjectEl.value = val.slice(0, start) + tag + val.slice(end);
        subjectEl.focus();
        subjectEl.setSelectionRange(start + tag.length, start + tag.length);
        stash();
      }, true);
    });
  }

  // --- treść: akcje szablonu <-> pastylki ------------------------------------

  var pdfBox = document.getElementById('f-pdf');
  var pdfInfo = document.getElementById('f-pdf-info');
  var pdfInfoF = document.getElementById('pdf-info-f');

  function setPDF(on, info) {
    if (!pdfBox) return;
    pdfBox.checked = on;
    pdfInfo.value = info || '';
    pdfInfoF.hidden = !on;
  }

  function toEditor(html) {
    if (!isEvents()) return {html: html || '', pdf: null};
    return V.toEditor(html, true);
  }

  function fromEditor() {
    var html = V.fromEditor(editor);
    if (pdfBox && pdfBox.checked && isEvents()) {
      html += V.actionText('.PDF', pdfInfo.value.trim());
    }
    return html;
  }

  function setText(html) {
    var r = toEditor(html);
    setPDF(r.pdf !== null, r.pdf);
    editor.setContents([]);
    editor.clipboard.dangerouslyPasteHTML(r.html);
    editor.history.clear();
  }

  function setRelatedMode() {
    page.classList.toggle('mail-forms', !isEvents());
    if (varButton) varButton.hidden = !isEvents();
  }

  // --- wartości, szkic, zapis -----------------------------------------------

  function values() {
    var out = {};
    page.querySelectorAll('[data-field]').forEach(function (el) {
      out[el.dataset.field] = el.type === 'checkbox' ? el.checked : el.value;
    });
    if (editor) out.text = fromEditor();
    if (mailPage && isEvents()) out.subject = subjectFromEditor(out.subject);
    return out;
  }

  // niezapisane zmiany trzymamy jako szkic w przeglądarce, żeby dało się
  // wyjść na inną stronę i wrócić do tych samych wartości
  var drafts = RZ.draft('rezerwo.draft:' + url);
  var loaded = !mailPage;

  function stash() { if (loaded) drafts.save('page', values()); }

  function applyValues(v) {
    page.querySelectorAll('[data-field]').forEach(function (el) {
      if (v[el.dataset.field] === undefined) return;
      if (el.type === 'checkbox') el.checked = !!v[el.dataset.field];
      else el.value = v[el.dataset.field];
    });
    setRelatedMode();
    if (subjectEl && mailPage) subjectEl.value = subjectToEditor(subjectEl.value);
    if (editor && v.text !== undefined) setText(v.text);
  }

  page.addEventListener('input', stash);
  page.addEventListener('change', function (e) {
    if (e.target === relatedEl) setRelatedMode();
    if (e.target === pdfBox) pdfInfoF.hidden = !pdfBox.checked;
    stash();
  });
  if (editor) editor.on('text-change', stash);

  RZ.ready.then(function () {
    // etykiety pastylek i tematu są w tłumaczeniach, więc treść wczytujemy
    // dopiero po nich
    if (mailPage) {
      if (varButton) varButton.textContent = RZ.t('mail.insert');
      applyValues({
        subject: subjectEl.value,
        text: document.getElementById('mail-text').innerHTML
      });
    }
    var d = drafts.get('page');
    if (d) {
      applyValues(d);
      RZ.draftNotice(function () {
        drafts.clearAll();
        location.reload();
      });
    }
    loaded = true;
  });

  document.getElementById('save').addEventListener('click', function () {
    var btn = this;
    var msg = page.querySelector('.sec-f .msg');
    btn.disabled = true;
    if (msg) { msg.textContent = RZ.t('common.loading'); msg.classList.remove('bad'); }

    RZ.postJSON(url, values())
      .then(function (resp) {
        drafts.clearAll();
        // wracamy tam, skąd admin przyszedł (impreza, formularz, katalogi);
        // ostrzeżenie zostawia go na stronie, żeby je przeczytał
        if (!resp.warning && page.dataset.back) {
          location.href = page.dataset.back;
          return;
        }
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

  // --- podgląd i mail próbny -------------------------------------------------

  var previewBtn = document.getElementById('preview');
  if (previewBtn) previewBtn.addEventListener('click', function () {
    var v = values();
    var body = {subject: v.subject, text: v.text};
    var msg = page.querySelector('.sec-f .msg');
    RZ.postJSON(url + '/preview', body)
      .then(function (resp) { if (msg) msg.textContent = ''; showPreview(resp, body); })
      .catch(function (err) {
        if (msg) { msg.textContent = err.message; msg.classList.add('bad'); }
      });
  });

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  function showPreview(resp, body) {
    var back = el('div', 'dlg-back');
    var dlg = el('div', 'dlg dlg-wide mail-preview');
    dlg.setAttribute('role', 'dialog');
    dlg.setAttribute('aria-modal', 'true');
    back.appendChild(dlg);

    dlg.appendChild(el('h3', '', RZ.t('mail.preview_title')));
    dlg.appendChild(el('p', 'muted', RZ.t('mail.preview_hint')));

    var head = el('dl', 'mp-head');
    function row(label, value) {
      head.appendChild(el('dt', '', label));
      head.appendChild(el('dd', '', value));
    }
    row(RZ.t('mail.to'), resp.to);
    row(RZ.t('mail.subject'), resp.subject);
    if (resp.files && resp.files.length) row(RZ.t('mail.files'), resp.files.join(', '));
    dlg.appendChild(head);

    var frame = document.createElement('iframe');
    frame.setAttribute('sandbox', '');
    frame.title = RZ.t('mail.preview_title');
    frame.srcdoc = '<html><head><base href="' + location.origin + '/"><style>body{font:15px/1.5 sans-serif;color:#222;margin:12px}p{margin:0}img{max-width:100%}</style></head><body>' + resp.html + '</body></html>';
    dlg.appendChild(frame);

    var foot = el('div', 'sec-f');
    var send = el('button', 'btn', RZ.t('mail.test_send', [resp.to]));
    var close = el('button', 'btn quiet', RZ.t('common.close'));
    var note = el('small', 'msg');
    foot.appendChild(send);
    foot.appendChild(close);
    foot.appendChild(note);
    dlg.appendChild(foot);

    function done() {
      back.remove();
      document.removeEventListener('keydown', esc);
      previewBtn.focus();
    }
    function esc(e) { if (e.key === 'Escape') done(); }
    close.addEventListener('click', done);
    back.addEventListener('click', function (e) { if (e.target === back) done(); });
    document.addEventListener('keydown', esc);

    send.addEventListener('click', function () {
      send.disabled = true;
      note.classList.remove('bad');
      note.textContent = RZ.t('common.loading');
      RZ.postJSON(url + '/test', body)
        .then(function (r) {
          note.textContent = RZ.t('mail.test_sent', [r.to]);
          RZ.toast(RZ.t('mail.test_sent', [r.to]));
        })
        .catch(function (err) { note.textContent = err.message; note.classList.add('bad'); })
        .finally(function () { send.disabled = false; });
    });

    document.body.appendChild(back);
    close.focus();
  }
})();

// Konto bankowe: czeski numer konta (předčíslí-číslo) i kod banku składają
// się w IBAN, a wpisany IBAN rozkłada się z powrotem na numer i kod banku.
// Zapisywany jest tylko IBAN (z ewentualnym "+BIC" na końcu).
(function () {
  'use strict';

  var ibanEl = document.getElementById('f-iban');
  var numEl = document.getElementById('f-cz-number');
  var bankEl = document.getElementById('f-cz-bank');
  var hint = document.getElementById('cz-hint');
  if (!ibanEl || !numEl || !bankEl) return;

  function digits(s) { return /^\d+$/.test(s); }

  // mod 97 na długim ciągu cyfr, kawałkami (nie mieści się w liczbie JS)
  function mod97(s) {
    var r = 0;
    for (var i = 0; i < s.length; i += 7) r = parseInt(r + s.substr(i, 7), 10) % 97;
    return r;
  }

  // suma kontrolna czeskiego numeru konta: wagi od prawej 1,2,4,8,5,10,9,7,3,6
  function czMod11(s) {
    var w = [1, 2, 4, 8, 5, 10, 9, 7, 3, 6], sum = 0;
    for (var i = 0; i < s.length; i++) sum += parseInt(s.charAt(s.length - 1 - i), 10) * w[i];
    return sum % 11 === 0;
  }

  function pad(s, n) { while (s.length < n) s = '0' + s; return s; }

  // "19-2000145399", "0800" -> "CZ6508000000192000145399", albo "" i powód
  function toIBAN(acc, bank) {
    acc = acc.replace(/\s+/g, '');
    var parts = acc.split('-');
    var prefix = parts.length === 2 ? parts[0] : '';
    var number = parts.length === 2 ? parts[1] : parts[0];
    if (parts.length > 2 || (prefix && (!digits(prefix) || prefix.length > 6)) ||
        !digits(number) || number.length < 2 || number.length > 10) return {err: 'account.cz_bad_number'};
    if (!/^\d{4}$/.test(bank)) return {err: 'account.cz_bad_bank'};
    if ((prefix && !czMod11(prefix)) || !czMod11(number)) return {err: 'account.cz_bad_checksum'};
    var bban = bank + pad(prefix, 6) + pad(number, 10);
    var check = 98 - mod97(bban + '123500');
    return {iban: 'CZ' + pad(String(check), 2) + bban};
  }

  // IBAN (bez spacji, bez +BIC) -> numer i kod banku; null dla nie-czeskich
  function fromIBAN(iban) {
    if (!/^CZ\d{22}$/.test(iban)) return null;
    var bank = iban.substr(4, 4);
    var prefix = iban.substr(8, 6).replace(/^0+/, '');
    var number = iban.substr(14).replace(/^0+/, '');
    return {number: (prefix ? prefix + '-' : '') + number, bank: bank};
  }

  function splitIBAN() {
    var v = ibanEl.value.replace(/\s+/g, '').toUpperCase();
    var i = v.indexOf('+');
    return i < 0 ? {iban: v, bic: ''} : {iban: v.slice(0, i), bic: v.slice(i)};
  }

  function setHint(key) {
    if (!hint) return;
    hint.textContent = RZ.t(key || 'account.cz_hint');
    hint.classList.toggle('bad', !!key);
  }

  function fromCZ() {
    var acc = numEl.value.trim(), bank = bankEl.value.trim();
    if (!acc && !bank) { setHint(); return; }
    if (!acc || bank.length < 4) { setHint(); return; }
    var r = toIBAN(acc, bank);
    if (r.err) { setHint(r.err); return; }
    setHint();
    ibanEl.value = r.iban + splitIBAN().bic;
    ibanEl.dispatchEvent(new Event('input', {bubbles: true})); // szkic
  }

  function fromIBANField() {
    var cz = fromIBAN(splitIBAN().iban);
    if (!cz) return;
    numEl.value = cz.number;
    bankEl.value = cz.bank;
    setHint();
  }

  numEl.addEventListener('input', fromCZ);
  bankEl.addEventListener('input', fromCZ);
  ibanEl.addEventListener('input', function (e) { if (e.isTrusted) fromIBANField(); });
  fromIBANField();
  // szkic z przeglądarki podmienia IBAN po wczytaniu tłumaczeń
  RZ.ready.then(fromIBANField);
})();
