// n_vars.js - dane zamówienia jako "pastylki" w edytorze Quill.
//
// Używa go edytor maila imprezy (n_mail.js) i tekst "Po zamówieniu" na
// stronie imprezy (n_event.js). W edytorze admin widzi pastylkę "Imię" albo
// "Kod QR do płatności: Konto", a w bazie zostaje akcja szablonu Go
// ({{.Name}}, {{.QRPayMail "Konto"}}), więc istniejące teksty działają bez
// zmian. Zamiana w obie strony: toEditor() przy wczytaniu, fromEditor() przy
// zapisie. Wymaga n_ui.js (RZ.t, RZ.editorDOM) i Quilla.
//
// Lista zmiennych i kont przychodzi z serwera (mailEditorVars w catalog_ui.go):
// {vars: [{code, label, subject, account}], accounts: ["Konto", ...]}.
// account: akcja z nazwą konta, np. {{.PayInfo "Konto"}} (dane do przelewu,
// pay.go) - wstawia się i zmienia jak kod QR. Maile formularzy mają też
// zmienne ze stałym argumentem: {code: '.Field', arg: 'klasa', text: 'Klasa'}
// - pole formularza, text to jego etykieta.

(function (global) {
  'use strict';

  var RZ = global.RZ;

  // {{.Name}}, {{ .Surname }}, {{.QRPayMail "Konto"}}, {{.PDF "opis"}}
  var ACTION = /\{\{\s*(\.[A-Za-z]+)(?:\s+("(?:[^"\\]|\\.)*"))?\s*\}\}/g;

  // qr: akcja kodu QR w tym miejscu - w mailu obrazek w załączniku
  // (.QRPayMail), na stronie zwykły obrazek (.QRPay)
  var meta = {vars: [], accounts: [], qr: '.QRPayMail'};

  function isQR(code) { return code === '.QRPayMail' || code === '.QRPay'; }

  function findVar(code, account) {
    for (var i = 0; i < meta.vars.length; i++) {
      var v = meta.vars[i];
      if (v.code === code && !v.account === !account && v.arg === undefined) return v;
    }
    return null;
  }

  // simpleVar - zmienna bez argumentu ({{.Name}}), accountVar - z kontem,
  // fieldVar - ze stałym argumentem ({{.Field "klasa"}})
  function simpleVar(code) { return findVar(code, false); }
  function accountVar(code) { return findVar(code, true); }
  function fieldVar(code, arg) {
    for (var i = 0; i < meta.vars.length; i++) {
      var v = meta.vars[i];
      if (v.code === code && v.arg !== undefined && v.arg === arg) return v;
    }
    return null;
  }

  // varLabel - etykieta zmiennej: pole formularza albo klucz tłumaczenia
  function varLabel(v) { return v.text || RZ.t(v.label); }

  // findAny - zmienna z argumentem albo bez (bez kont)
  function findAny(code, arg) {
    return arg !== undefined && arg !== null ? fieldVar(code, arg) : simpleVar(code);
  }

  // pastylka z kontem: kod QR albo dane do przelewu
  function withAccount(code) { return isQR(code) || !!accountVar(code); }

  function hasAccount(name) { return meta.accounts.indexOf(name) >= 0; }

  function chipLabel(code, arg) {
    if (withAccount(code)) {
      var name = isQR(code) ? RZ.t('mail.qr') : RZ.t(accountVar(code).label);
      return name + ': ' + (hasAccount(arg) ? arg : RZ.t('mail.qr_choose'));
    }
    var v = findAny(code, arg);
    return v ? varLabel(v) : code;
  }

  // shortLabel - etykieta pastylki; pola formularza bywają całymi zdaniami
  function shortLabel(code, arg) {
    var l = chipLabel(code, arg);
    return l.length > 48 ? l.slice(0, 45).trim() + '…' : l;
  }

  function actionText(code, arg) {
    // JSON.stringify daje literał, który Go przyjmuje jako napis w szablonie
    return '{{' + code + (arg !== undefined && arg !== null ? ' ' + JSON.stringify(arg) : '') + '}}';
  }

  function unquote(arg) {
    try { return JSON.parse(arg); } catch (e) { return arg.slice(1, -1); }
  }

  var registered = false;
  function register() {
    if (registered || typeof Quill === 'undefined') return;
    registered = true;
    var Embed = Quill.import('blots/embed');

    class MailVar extends Embed {
      static create(value) {
        var node = super.create();
        node.dataset.code = value.code;
        if (value.arg !== undefined && value.arg !== null) node.dataset.arg = value.arg;
        node.textContent = shortLabel(value.code, value.arg);
        node.title = chipLabel(value.code, value.arg);
        if (withAccount(value.code)) {
          node.classList.add('acct');
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
  }

  // setup ustawia zmienne i konta; wołać przed utworzeniem edytora.
  function setup(data, qr) {
    meta.vars = (data && data.vars) || [];
    meta.accounts = (data && data.accounts) || [];
    meta.qr = qr || '.QRPayMail';
    register();
  }

  // toEditor zamienia znane akcje w HTML na pastylki. Działa na węzłach
  // tekstu, więc akcje w atrybutach (np. w linku) i nieznane akcje zostają
  // jak były. withPDF: akcję {{.PDF "opis"}} wycina i oddaje w .pdf.
  function toEditor(html, withPDF) {
    var tpl = document.createElement('template');
    tpl.innerHTML = html || '';
    var pdf = null;

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
        if (withPDF && code === '.PDF' && arg !== null) {
          if (pdf === null) pdf = arg;
          chip = '';
        } else if (isQR(code) && arg !== null) {
          chip = {code: meta.qr, arg: arg};
        } else if (arg !== null && accountVar(code)) {
          chip = {code: code, arg: arg};
        } else if (arg === null && simpleVar(code)) {
          chip = {code: code};
        } else if (arg !== null && fieldVar(code, arg)) {
          chip = {code: code, arg: arg};
        }
        if (chip === null) continue;

        frag.appendChild(document.createTextNode(s.slice(last, m.index)));
        if (chip) {
          var span = document.createElement('span');
          span.className = 'mvar';
          span.dataset.code = chip.code;
          if (chip.arg !== undefined) span.dataset.arg = chip.arg;
          span.textContent = shortLabel(chip.code, chip.arg);
          span.title = chipLabel(chip.code, chip.arg);
          frag.appendChild(span);
        }
        last = m.index + m[0].length;
      }
      frag.appendChild(document.createTextNode(s.slice(last)));
      node.parentNode.replaceChild(frag, node);
    });
    return {html: tpl.innerHTML, pdf: pdf};
  }

  // fromEditor: HTML do zapisu, pastylki z powrotem jako akcje. Znaczniki
  // podmieniamy dopiero po innerHTML, żeby nazwa konta z "&" czy "<" nie
  // została zamieniona na encję wewnątrz akcji.
  function fromEditor(quill) {
    var root = RZ.editorDOM(quill.root.cloneNode(true));
    var actions = [];
    root.querySelectorAll('.mvar').forEach(function (node) {
      actions.push(actionText(node.dataset.code, node.dataset.arg));
      node.replaceWith(document.createTextNode('⁣MV' + (actions.length - 1) + '⁣'));
    });
    return root.innerHTML.replace(/⁣MV(\d+)⁣/g, function (all, i) { return actions[+i]; });
  }

  function insertChip(quill, index, code, arg) {
    quill.insertEmbed(index, 'mailvar', {code: code, arg: arg}, 'user');
    quill.setSelection(index + 1, 0, 'user');
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

  function openAccountMenu(anchor, pick) {
    if (!meta.accounts.length) { openMenu(anchor, [{note: RZ.t('mail.no_accounts')}]); return; }
    openMenu(anchor, meta.accounts.map(function (name) {
      return {label: name, action: function () { pick(name); }};
    }));
  }

  // openVarMenu: pick(code) albo pick(kod, argument) dla pól formularza,
  // kodu QR i danych do przelewu (argument to nazwa pola albo konta).
  // subjectOnly - tylko zmienne, które mają sens w temacie maila (bez tych
  // z kontem). Pola formularza oddziela kreska od danych gościa.
  function openVarMenu(anchor, pick, subjectOnly) {
    var items = [], fields = false;
    meta.vars
      .filter(function (v) { return !v.account && !v.hidden && (!subjectOnly || v.subject); })
      .forEach(function (v) {
        if (v.arg !== undefined && !fields && items.length) items.push({sep: true});
        if (v.arg !== undefined) fields = true;
        items.push({label: varLabel(v), action: function () { pick(v.code, v.arg); }});
      });
    if (!subjectOnly) {
      var withAcc = function (label, code) {
        return {
          label: label + ' ›',
          keepOpen: true,
          action: function () {
            openAccountMenu(anchor, function (name) { pick(code, name); });
          }
        };
      };
      items.push({sep: true});
      items.push(withAcc(RZ.t('mail.qr'), meta.qr));
      meta.vars.forEach(function (v) {
        if (v.account) items.push(withAcc(RZ.t(v.label), v.code));
      });
    }
    openMenu(anchor, items);
  }

  // attach dopina do edytora przycisk "+ Dane zamówienia" (ql-mailvar
  // z paska narzędzi) i zmianę konta po kliknięciu w pastylkę z kontem.
  function attach(quill) {
    var button = quill.getModule('toolbar').container.querySelector('.ql-mailvar');
    if (button) button.textContent = RZ.t('mail.insert');
    quill.root.addEventListener('click', function (e) {
      var node = e.target.closest('.mvar.acct');
      if (!node) return;
      var blot = Quill.find(node);
      if (!blot || blot === node) return;
      var index = quill.getIndex(blot);
      openAccountMenu(node, function (name) {
        quill.deleteText(index, 1, 'user');
        insertChip(quill, index, node.dataset.code, name);
      });
    });
    return button;
  }

  // handler przycisku ql-mailvar w konfiguracji paska Quilla
  function toolbarHandler(getQuill) {
    return function () {
      var quill = getQuill();
      var range = quill.getSelection(true);
      var button = quill.getModule('toolbar').container.querySelector('.ql-mailvar');
      openVarMenu(button, function (code, arg) { insertChip(quill, range.index, code, arg); });
    };
  }

  RZ.vars = {
    setup: setup,
    simpleVar: simpleVar,
    findVar: findAny,
    varLabel: varLabel,
    vars: function () { return meta.vars; },
    toEditor: toEditor,
    fromEditor: fromEditor,
    actionText: actionText,
    openVarMenu: openVarMenu,
    attach: attach,
    toolbarHandler: toolbarHandler
  };
})(window);
