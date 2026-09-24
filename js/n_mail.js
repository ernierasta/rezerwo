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

  var relatedEl = document.getElementById('f-related');
  function isEvents() { return !relatedEl || relatedEl.value === 'events'; }

  // --- pastylka zmiennej -----------------------------------------------------

  // {{.Name}}, {{ .Surname }}, {{.QRPayMail "Konto"}}, {{.PDF "opis"}}
  var ACTION = /\{\{\s*(\.[A-Za-z]+)(?:\s+("(?:[^"\\]|\\.)*"))?\s*\}\}/g;

  function simpleVar(code) {
    for (var i = 0; i < meta.vars.length; i++) {
      if (meta.vars[i].code === code) return meta.vars[i];
    }
    return null;
  }

  function hasAccount(name) { return meta.accounts.indexOf(name) >= 0; }

  function chipLabel(code, arg) {
    if (code === '.QRPayMail') {
      return RZ.t('mail.qr') + ': ' + (hasAccount(arg) ? arg : RZ.t('mail.qr_choose'));
    }
    var v = simpleVar(code);
    return v ? RZ.t(v.label) : code;
  }

  function actionText(code, arg) {
    // JSON.stringify daje literał, który Go przyjmuje jako napis w szablonie
    return '{{' + code + (arg !== undefined && arg !== null ? ' ' + JSON.stringify(arg) : '') + '}}';
  }

  var editor = null;
  var editorEl = document.getElementById('mail-editor');

  if (editorEl && typeof Quill !== 'undefined') {
    var Embed = Quill.import('blots/embed');

    class MailVar extends Embed {
      static create(value) {
        var node = super.create();
        node.dataset.code = value.code;
        if (value.arg !== undefined && value.arg !== null) node.dataset.arg = value.arg;
        node.textContent = chipLabel(value.code, value.arg);
        if (value.code === '.QRPayMail') {
          node.classList.add('qr');
          if (!hasAccount(value.arg)) node.classList.add('bad');
        }
        return node;
      }
      static value(node) {
        return {code: node.dataset.code, arg: node.dataset.arg};
      }
    }
    MailVar.blotName = 'mailvar';
    MailVar.tagName = 'span';
    MailVar.className = 'mvar';
    Quill.register(MailVar);

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
        handlers: {
          mailvar: function () {
            var range = editor.getSelection(true);
            openVarMenu(varButton, function (code, arg) { insertChip(range.index, code, arg); });
          }
        }
      }),
      theme: 'snow'
    });
  }

  var varButton = page.querySelector('.ql-mailvar');

  function insertChip(index, code, arg) {
    editor.insertEmbed(index, 'mailvar', {code: code, arg: arg}, 'user');
    editor.setSelection(index + 1, 0, 'user');
  }

  // Kliknięcie w pastylkę QR pozwala zmienić konto.
  if (editor) {
    editor.root.addEventListener('click', function (e) {
      var node = e.target.closest('.mvar.qr');
      if (!node) return;
      var blot = Quill.find(node);
      if (!blot || blot === node) return;
      var index = editor.getIndex(blot);
      openAccountMenu(node, function (name) {
        editor.deleteText(index, 1, 'user');
        insertChip(index, '.QRPayMail', name);
      });
    });
  }

  // --- menu wstawiania -------------------------------------------------------

  var menu = null;

  function closeMenu() {
    if (menu) menu.remove();
    menu = null;
    document.removeEventListener('mousedown', outside, true);
    document.removeEventListener('keydown', onEsc, true);
  }
  function outside(e) { if (menu && !menu.contains(e.target)) closeMenu(); }
  function onEsc(e) { if (e.key === 'Escape') closeMenu(); }

  // items: [{label, action}] albo {sep: true} albo {note: 'tekst'}
  function openMenu(anchor, items) {
    closeMenu();
    menu = document.createElement('div');
    menu.className = 'mv-menu';
    menu.setAttribute('role', 'menu');
    items.forEach(function (it) {
      if (it.sep) { menu.appendChild(document.createElement('hr')); return; }
      if (it.note) {
        var p = document.createElement('p');
        p.className = 'muted';
        p.textContent = it.note;
        menu.appendChild(p);
        return;
      }
      var b = document.createElement('button');
      b.type = 'button';
      b.setAttribute('role', 'menuitem');
      b.textContent = it.label;
      b.addEventListener('mousedown', function (e) { e.preventDefault(); });
      b.addEventListener('click', function () {
        if (it.keepOpen) { it.action(); return; }
        closeMenu();
        it.action();
      });
      menu.appendChild(b);
    });
    document.body.appendChild(menu);

    var r = anchor.getBoundingClientRect();
    var left = Math.min(r.left + window.scrollX, window.scrollX + document.documentElement.clientWidth - menu.offsetWidth - 8);
    menu.style.left = Math.max(8, left) + 'px';
    menu.style.top = (r.bottom + window.scrollY + 4) + 'px';

    setTimeout(function () {
      document.addEventListener('mousedown', outside, true);
      document.addEventListener('keydown', onEsc, true);
    }, 0);
    var first = menu.querySelector('button');
    if (first) first.focus();
  }

  function accountItems(pick) {
    if (!meta.accounts.length) return [{note: RZ.t('mail.no_accounts')}];
    return meta.accounts.map(function (name) {
      return {label: name, action: function () { pick(name); }};
    });
  }

  function openAccountMenu(anchor, pick) { openMenu(anchor, accountItems(pick)); }

  function openVarMenu(anchor, pick, subjectOnly) {
    var items = meta.vars
      .filter(function (v) { return !subjectOnly || v.subject; })
      .map(function (v) {
        return {label: RZ.t(v.label), action: function () { pick(v.code); }};
      });
    if (!subjectOnly) {
      items.push({sep: true});
      items.push({
        label: RZ.t('mail.qr') + ' ›',
        keepOpen: true,
        action: function () {
          openAccountMenu(anchor, function (name) { pick('.QRPayMail', name); });
        }
      });
    }
    openMenu(anchor, items);
  }

  // --- temat: [Etykieta] <-> {{.Code}} --------------------------------------

  var subjectEl = document.getElementById('f-subject');

  function subjectToEditor(s) {
    if (!isEvents()) return s;
    return s.replace(ACTION, function (all, code, arg) {
      var v = arg ? null : simpleVar(code);
      return v ? '[' + RZ.t(v.label) + ']' : all;
    });
  }

  function subjectFromEditor(s) {
    meta.vars.forEach(function (v) {
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
      openVarMenu(subjectVar, function (code) {
        var tag = '[' + RZ.t(simpleVar(code).label) + ']';
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

  function unquote(arg) {
    try { return JSON.parse(arg); } catch (e) { return arg.slice(1, -1); }
  }

  // Zamienia znane akcje w tekście na pastylki. Działa na węzłach tekstu, więc
  // akcje w atrybutach (np. w linku) i nieznane akcje zostają jak były.
  function toEditor(html) {
    var tpl = document.createElement('template');
    tpl.innerHTML = html || '';
    var pdf = null;
    if (!isEvents()) return {html: tpl.innerHTML, pdf: pdf};

    var walker = document.createTreeWalker(tpl.content, NodeFilter.SHOW_TEXT);
    var texts = [];
    while (walker.nextNode()) texts.push(walker.currentNode);

    texts.forEach(function (node) {
      var s = node.data;
      ACTION.lastIndex = 0;
      if (!ACTION.test(s)) return;
      ACTION.lastIndex = 0;

      var frag = document.createDocumentFragment();
      var last = 0, m;
      while ((m = ACTION.exec(s)) !== null) {
        var code = m[1], arg = m[2] ? unquote(m[2]) : null, chip = null;
        if (code === '.PDF' && arg !== null) {
          if (pdf === null) pdf = arg;
          chip = '';
        } else if ((code === '.QRPayMail' || code === '.QRPay') && arg !== null) {
          chip = {code: '.QRPayMail', arg: arg};
        } else if (arg === null && simpleVar(code)) {
          chip = {code: code};
        }
        if (chip === null) continue;

        frag.appendChild(document.createTextNode(s.slice(last, m.index)));
        if (chip) {
          var span = document.createElement('span');
          span.className = 'mvar';
          span.dataset.code = chip.code;
          if (chip.arg !== undefined) span.dataset.arg = chip.arg;
          span.textContent = chipLabel(chip.code, chip.arg);
          frag.appendChild(span);
        }
        last = m.index + m[0].length;
      }
      frag.appendChild(document.createTextNode(s.slice(last)));
      node.parentNode.replaceChild(frag, node);
    });
    return {html: tpl.innerHTML, pdf: pdf};
  }

  // Pastylki na akcje. Znaczniki podmieniamy dopiero po innerHTML, żeby nazwa
  // konta z "&" czy "<" nie została zamieniona na encję wewnątrz akcji.
  function fromEditor() {
    var root = RZ.editorDOM(editor.root.cloneNode(true));
    var actions = [];
    root.querySelectorAll('.mvar').forEach(function (node) {
      actions.push(actionText(node.dataset.code, node.dataset.arg));
      node.replaceWith(document.createTextNode('⁣MV' + (actions.length - 1) + '⁣'));
    });
    var html = root.innerHTML.replace(/⁣MV(\d+)⁣/g, function (all, i) { return actions[+i]; });
    if (pdfBox && pdfBox.checked && isEvents()) {
      html += actionText('.PDF', pdfInfo.value.trim());
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
