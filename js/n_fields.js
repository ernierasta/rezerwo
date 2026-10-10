// n_fields.js - designer pól formularza (tmpl/n_fields.html, forms_fields.go).
//
// Model to lista pól w formacie formBuildera (tak jak w formtemplates.content)
// plus pola pomocnicze z podkreślnikiem, których nie zapisujemy:
//   _id    numer pola w tej sesji
//   _orig  nazwa pola w bazie (do zmian nazw bez utraty odpowiedzi)
//   _new   pole jeszcze niezapisane - nazwa idzie za etykietą
//   _w     szerokość 1-12 (col-md-N), _nr - zacznij nowy wiersz
// Przy zapisie szerokości zamieniają się z powrotem na klasy row-N col-md-N.

(function () {
  'use strict';

  var root = document.getElementById('fd');
  if (!root) return;

  var t = RZ.t;
  var formID = root.dataset.formId;
  var isAddons = root.dataset.addons === '1';
  var currency = root.dataset.currency || 'CZK';
  var canvas = document.getElementById('fd-canvas');
  var props = document.getElementById('fd-props');
  var btnSave = document.getElementById('fd-save');
  var btnUndo = document.getElementById('fd-undo');
  var btnRedo = document.getElementById('fd-redo');
  var stateEl = document.getElementById('fd-state');
  var draftKey = 'rezerwo.fields.' + formID;
  var panelMode = root.dataset.panelMode || 'text';
  var SUMMABLE = ['number', 'multiplierField', 'select', 'radio-group', 'checkbox-group'];

  var ROLE = {name: 'name-REQUIRED', surname: 'surname-REQUIRED', email: 'email-REQUIRED'};
  var PERSEAT = '-perseat';
  var INPUTS = ['text', 'textarea', 'number', 'date', 'select', 'radio-group', 'checkbox-group', 'multiplierField'];
  var CHOICE = ['select', 'radio-group', 'checkbox-group'];
  var PLAIN = ['text', 'textarea', 'number', 'date'];
  var WIDTHS = [[12, '1'], [9, '¾'], [8, '⅔'], [6, '½'], [4, '⅓'], [3, '¼']];

  var fields = [];
  var moneyId = 0;
  var sel = 0;
  var uid = 0;
  var counts = (window.FD_DATA && FD_DATA.counts) || {};
  var stats = (window.FD_DATA && FD_DATA.stats) || {};
  var savedSig = '';
  var undoStack = [], redoStack = [], current = null, lastKey = '', lastTime = 0;
  var dragSrc = null;

  // --- pomocnicze --------------------------------------------------------

  function esc(s) {
    return String(s === undefined || s === null ? '' : s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function isInput(type) { return INPUTS.indexOf(type) >= 0; }
  function byId(id) { for (var i = 0; i < fields.length; i++) if (fields[i]._id === id) return fields[i]; return null; }
  function indexOfId(id) { for (var i = 0; i < fields.length; i++) if (fields[i]._id === id) return i; return -1; }
  function roleOf(f) {
    for (var k in ROLE) if (f.name === ROLE[k]) return k;
    return '';
  }
  // inPanel - czy pole trafi do panelu informacyjnego (fieldInPanel w
  // form_stats.go): w trybie automatycznym domyślnie pola z liczbami i opcjami,
  // w pozostałych tylko te z włączonym przełącznikiem.
  function inPanel(f) {
    if (!isInput(f.type) || roleOf(f)) return false;
    if (f.panel !== undefined) return !!f.panel;
    return panelMode === 'auto' && SUMMABLE.indexOf(f.type) >= 0;
  }

  // atField - stan pola pokazany przy polu zamiast w panelu (panel_place,
  // AttachFieldStats w form_stats.go)
  function atField(f) {
    return !isAddons && (panelMode === 'auto' || panelMode === 'fields') && inPanel(f) && f.panel_place === 'field';
  }

  // optStat - liczba przy opcji (pole ze stanem przy polu), albo ''
  function optStat(f, label, inSelect) {
    if (!atField(f) || (f.panel_only && f.panel_only !== label)) return '';
    var s = stats[f._orig] || {};
    var n = (s.o && s.o[label]) || 0;
    var g = Number(f.panel_goal) || 0;
    var txt = n + (g ? ' / ' + g : '');
    if (inSelect) return ' (' + esc(txt) + ')';
    return ' <small class="opt-stat' + (g && n >= g ? ' done' : '') + '">' + esc(txt) + '</small>';
  }

  // statHTML - wiersz stanu pod polem bez opcji (szablon "ffstat")
  function statHTML(f) {
    if (!atField(f) || CHOICE.indexOf(f.type) >= 0) return '';
    var v = (stats[f._orig] || {}).v || '0';
    var g = Number(f.panel_goal) || 0;
    var done = g > 0 && parseFloat(v) >= g;
    var p = g ? Math.min(100, Math.floor(parseFloat(v) * 100 / g)) : 0;
    return '<div class="ff-stat' + (done ? ' done' : '') + '"><span>' + esc(g ? t('gform.stat_goal', [v, g]) : t('gform.stat', v)) + '</span>' +
      (g ? '<i class="ip-bar' + (done ? ' done' : '') + '"><i style="width:' + p + '%"></i></i>' : '') + '</div>';
  }

  function answersOf(f) { return f._orig ? (counts[f._orig] || 0) : 0; }

  // cleanHTML zostawia w etykiecie to samo co serwer (CleanLabel w formdef.go):
  // pogrubienie, kursywę, podkreślenie, nowy wiersz i link.
  var ALLOWED = ['b', 'strong', 'i', 'em', 'u', 'br', 'a'];
  function cleanHTML(html) {
    html = html || '';
    // stare akapity mają HTML zapisany jako encje (&lt;b&gt;) - jak w CleanLabel
    if (html.indexOf('<') < 0 && html.indexOf('&lt;') >= 0) {
      var ta = document.createElement('textarea');
      ta.innerHTML = html;
      html = ta.value;
    }
    var tpl = document.createElement('template');
    tpl.innerHTML = html;
    (function walk(node) {
      Array.prototype.slice.call(node.childNodes).forEach(function (ch) {
        if (ch.nodeType === 3) return;
        if (ch.nodeType !== 1) { ch.remove(); return; }
        var tag = ch.tagName.toLowerCase();
        if (tag === 'script' || tag === 'style') { ch.remove(); return; }
        walk(ch);
        if (ALLOWED.indexOf(tag) < 0) {
          if ((tag === 'div' || tag === 'p') && ch.previousSibling) ch.parentNode.insertBefore(document.createElement('br'), ch);
          while (ch.firstChild) ch.parentNode.insertBefore(ch.firstChild, ch);
          ch.remove();
          return;
        }
        Array.prototype.slice.call(ch.attributes).forEach(function (a) {
          if (!(tag === 'a' && a.name === 'href')) ch.removeAttribute(a.name);
        });
        if (tag === 'a' && !/^(https?:|mailto:|tel:)/i.test(ch.getAttribute('href') || '')) ch.removeAttribute('href');
      });
    })(tpl.content);
    return tpl.innerHTML.replace(/&nbsp;/g, ' ').replace(/(\s|<br\s*\/?>)+$/i, '').trim();
  }

  function plain(html) {
    var d = document.createElement('div');
    d.innerHTML = html || '';
    return (d.textContent || '').trim();
  }

  function slug(s) {
    s = plain(s).toLowerCase().replace(/ł/g, 'l');
    if (s.normalize) s = s.normalize('NFD').replace(/[\u0300-\u036f]/g, '');
    s = s.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
    if (s.length > 30) s = s.slice(0, 30).replace(/-+$/, '');
    return s;
  }

  function nameTaken(name, exceptId) {
    return fields.some(function (f) { return f._id !== exceptId && f.name === name; });
  }

  function uniqueName(base, exceptId) {
    base = base || 'pole';
    var n = base, i = 2;
    while (nameTaken(n, exceptId) || (counts[n] && !fields.some(function (f) { return f._orig === n; }))) {
      n = base + '-' + i++;
    }
    return n;
  }

  function currencySign(c) { return {CZK: 'Kč', EUR: '€', PLN: 'zł'}[c] || c; }

  // --- model: wczytanie i zapis ------------------------------------------

  var rowRe = /\brow-(\S+)/, colRe = /\bcol-(?:md-|sm-|lg-)?(\d{1,2})\b/;

  // load czyta definicję; pola z tym samym row-N stają obok siebie tak jak
  // w formRender (w miejscu pierwszego z nich).
  function load(content, money) {
    var groups = [], pos = {};
    (content || []).forEach(function (raw) {
      var f = JSON.parse(JSON.stringify(raw));
      f._id = ++uid;
      f._orig = f.name || '';
      var cls = f.className || '';
      var m = colRe.exec(cls);
      f._w = isInput(f.type) && m ? Math.min(12, Math.max(1, +m[1])) : 12;
      var row = isInput(f.type) ? (rowRe.exec(cls) || [])[1] : '';
      if (row && pos[row] !== undefined) {
        groups[pos[row]].push(f);
        return;
      }
      if (row) pos[row] = groups.length;
      groups.push([f]);
    });
    // _nr tylko tam, gdzie bez niego wiersz skleiłby się z poprzednim -
    // inaczej pole zapisane osobno nie dałoby się postawić obok innego
    fields = [];
    var prevSum = 12;
    groups.forEach(function (g) {
      var sum = 0;
      g.forEach(function (f, i) {
        var w = isInput(f.type) ? f._w : 12;
        f._nr = i === 0 && isInput(f.type) && prevSum + w <= 12;
        sum += w;
        fields.push(f);
      });
      prevSum = isInput(g[0].type) ? sum : 12;
    });
    moneyId = 0;
    fields.forEach(function (f) { if (money && f.name === money) moneyId = f._id; });
  }

  // layoutRows dzieli listę na wiersze: pola obok siebie, dopóki mieszczą
  // się w 12 kolumnach i żadne nie każe zacząć nowego wiersza.
  function layoutRows() {
    var rows = [], cur = null, sum = 0;
    fields.forEach(function (f) {
      var inp = isInput(f.type), w = inp ? (f._w || 12) : 12;
      if (!cur || !inp || f._nr || sum + w > 12) {
        cur = {fields: [], inp: inp};
        rows.push(cur);
        sum = 0;
      }
      cur.fields.push(f);
      sum += w;
      if (!inp) cur = null;
    });
    return rows;
  }

  function serialize() {
    var out = [], k = 0;
    layoutRows().forEach(function (r) {
      var multi = r.inp && (r.fields.length > 1 || (r.fields[0]._w || 12) < 12);
      if (multi) k++;
      r.fields.forEach(function (f) {
        var o = {};
        Object.keys(f).forEach(function (key) { if (key.charAt(0) !== '_') o[key] = f[key]; });
        if (isInput(f.type)) {
          var cls = (o.className || '').split(/\s+/).filter(function (c) {
            return c && !/^row-/.test(c) && !/^col-/.test(c);
          });
          var formControl = CHOICE.indexOf(f.type) < 0 || f.type === 'select';
          if (formControl && cls.indexOf('form-control') < 0) cls.unshift('form-control');
          if (multi) cls.push('row-' + k, 'col-md-' + (f._w || 12));
          if (cls.length) o.className = cls.join(' ');
          else delete o.className;
        }
        out.push(o);
      });
    });
    return out;
  }

  function moneyName() {
    var f = byId(moneyId);
    return f ? f.name : '';
  }

  function renames() {
    var r = {};
    fields.forEach(function (f) {
      if (f._orig && f.name && f._orig !== f.name) r[f._orig] = f.name;
    });
    return r;
  }

  function signature() {
    return JSON.stringify({c: serialize(), m: moneyName()});
  }

  // --- historia (cofnij / ponów) -----------------------------------------

  function snapshot() {
    return JSON.stringify({fields: fields, moneyId: moneyId, sel: sel});
  }

  function restore(s) {
    var o = JSON.parse(s);
    fields = o.fields;
    moneyId = o.moneyId;
    sel = byId(o.sel) ? o.sel : 0;
  }

  // commit zapisuje stan do historii. Kolejne zmiany tego samego ustawienia
  // w krótkim czasie (pisanie w polu) to jeden krok cofania.
  function commit(key) {
    var now = Date.now();
    var coalesce = key && key === lastKey && now - lastTime < 1200;
    lastKey = key || '';
    lastTime = now;
    if (!coalesce && current !== null) {
      undoStack.push(current);
      if (undoStack.length > 150) undoStack.shift();
      redoStack = [];
    }
    current = snapshot();
    afterChange();
  }

  function undo() {
    if (!undoStack.length) return;
    redoStack.push(current);
    current = undoStack.pop();
    restore(current);
    lastKey = '';
    render();
    renderProps();
    afterChange();
  }

  function redo() {
    if (!redoStack.length) return;
    undoStack.push(current);
    current = redoStack.pop();
    restore(current);
    lastKey = '';
    render();
    renderProps();
    afterChange();
  }

  function isDirty() { return signature() !== savedSig; }

  function afterChange() {
    btnUndo.disabled = !undoStack.length;
    btnRedo.disabled = !redoStack.length;
    var dirty = isDirty();
    root.classList.toggle('dirty', dirty);
    stateEl.textContent = dirty ? t('fields.unsaved') : '';
    try {
      if (dirty) localStorage.setItem(draftKey, JSON.stringify({base: savedSig, snap: snapshot()}));
      else localStorage.removeItem(draftKey);
    } catch (e) { /* prywatne okno */ }
  }

  // --- nowe pola ---------------------------------------------------------

  function newField(kind) {
    var f;
    var base = {required: false, access: false};
    function mk(o) { return Object.assign({}, base, o); }

    if (kind.indexOf('role:') === 0) {
      var role = kind.slice(5);
      var existing = fields.filter(function (x) { return x.name === ROLE[role]; })[0];
      if (existing) {
        select(existing._id);
        RZ.toast(t('fields.role_exists', t('fields.role.' + role)));
        return null;
      }
      f = mk({type: 'text', subtype: role === 'email' ? 'email' : 'text', required: true,
        label: t('fields.role.' + role), className: 'form-control', name: ROLE[role], maxlength: 60});
      f._w = role === 'email' ? 12 : 6;
    } else {
      switch (kind) {
        case 'tel':
          f = mk({type: 'text', subtype: 'tel', label: t('fields.type.tel'), className: 'form-control', maxlength: 30});
          break;
        case 'text':
          f = mk({type: 'text', subtype: 'text', label: t('fields.new.text'), className: 'form-control'});
          break;
        case 'textarea':
          f = mk({type: 'textarea', subtype: 'textarea', label: t('fields.new.textarea'), className: 'form-control', rows: 3});
          break;
        case 'number':
          f = mk({type: 'number', subtype: 'number', label: t('fields.new.number'), className: 'form-control', min: 0});
          break;
        case 'date':
          f = mk({type: 'date', label: t('fields.new.date'), className: 'form-control'});
          break;
        case 'select':
        case 'radio-group':
        case 'checkbox-group':
          f = mk({type: kind, label: t('fields.new.choice'), values: [
            {label: t('fields.new.option', 1), value: t('fields.new.option', 1), selected: false},
            {label: t('fields.new.option', 2), value: t('fields.new.option', 2), selected: false}
          ]});
          if (kind === 'select') { f.className = 'form-control'; f.multiple = false; }
          else { f.inline = false; f.other = false; }
          if (kind === 'checkbox-group') f.toggle = false;
          break;
        case 'multiplierField':
          f = mk({type: 'multiplierField', label: t('fields.new.multiplier'), className: 'form-control',
            multipl: '100', pre: '(', post: ' ' + currencySign(currency) + ')'});
          break;
        case 'header':
          f = {type: 'header', subtype: 'h2', label: t('fields.new.header'), access: false};
          break;
        case 'paragraph':
          f = {type: 'paragraph', subtype: 'p', label: t('fields.new.paragraph'), access: false};
          break;
        default:
          return null;
      }
      f._w = 12;
    }
    f._id = ++uid;
    f._orig = '';
    f._new = true;
    f._nr = false;
    if (isInput(f.type) && !f.name) f.name = uniqueName(slug(f.label) || f.type, f._id);
    return f;
  }

  function addField(kind, index, newRow) {
    var f = newField(kind);
    if (!f) return;
    if (index === undefined) {
      var i = indexOfId(sel);
      index = i >= 0 ? i + 1 : fields.length;
    }
    fields.splice(index, 0, f);
    if (newRow) ownRow(index);
    sel = f._id;
    commit();
    render();
    renderProps();
    var ed = props.querySelector('.fd-rich');
    if (ed) {
      ed.focus();
      document.execCommand('selectAll', false, null);
    }
    var item = canvas.querySelector('[data-id="' + f._id + '"]');
    if (item) item.scrollIntoView({block: 'nearest', behavior: 'smooth'});
  }

  // --- podgląd (te same klasy co tmpl/n_formfield.html) -------------------

  function fieldHTML(f) {
    var req = f.required ? '<i class="req">*</i>' : '';
    var desc = f.description ? '<small class="ff-d">' + esc(f.description) + '</small>' : '';
    var label = cleanHTML(f.label) || '<span class="muted">' + esc(t('fields.no_label')) + '</span>';
    var h = '';

    switch (f.type) {
      case 'header':
        var lvl = f.subtype === 'h3' ? 'h3' : (f.subtype === 'h4' || f.subtype === 'h5' || f.subtype === 'h6') ? 'h4' : 'h2';
        return '<div class="ff ff-header"><' + lvl + '>' + label + '</' + lvl + '></div>';
      case 'paragraph':
        return '<div class="ff ff-paragraph"><p>' + label + '</p></div>';
      case 'radio-group':
      case 'checkbox-group':
        var it = f.type === 'radio-group' ? 'radio' : 'checkbox';
        h = '<fieldset class="ff ff-group"><legend class="ff-l">' + label + req + '</legend>' + desc +
          '<div class="opts' + (f.inline ? ' inline' : '') + '">';
        (f.values || []).forEach(function (o) {
          h += '<label class="opt"><input type="' + it + '" tabindex="-1"' + (o.selected ? ' checked' : '') + '><span>' + esc(o.label) + '</span>' + optStat(f, o.label) + '</label>';
        });
        if (f.other) h += '<label class="opt other"><input type="' + it + '" tabindex="-1"><span>' + esc(t('gform.other')) + '</span><input class="inp other-text" disabled tabindex="-1"></label>';
        return h + '</div></fieldset>';
    }

    h = '<div class="ff ff-' + esc(f.type) + (moneyId === f._id ? ' money' : '') + '"><label class="ff-l">' + label + req + '</label>';
    switch (f.type) {
      case 'textarea':
        h += '<textarea class="inp" rows="' + esc(f.rows || 3) + '" tabindex="-1" placeholder="' + esc(f.placeholder || '') + '"></textarea>';
        break;
      case 'select':
        h += '<select class="inp" tabindex="-1"' + (f.multiple ? ' multiple' : '') + '>';
        if (!f.multiple) h += '<option>' + esc(t('field.select_hint')) + '</option>';
        (f.values || []).forEach(function (o) { h += '<option' + (o.selected && !f.multiple ? ' selected' : '') + '>' + esc(o.label) + optStat(f, o.label, true) + '</option>'; });
        h += '</select>';
        break;
      case 'number':
        h += '<input class="inp num" type="number" tabindex="-1" value="' + esc(f.value || '') + '" placeholder="' + esc(f.placeholder || '') + '">';
        break;
      case 'date':
        h += '<input class="inp" type="text" tabindex="-1" placeholder="' + esc(t('gform.date_ph')) + '">';
        break;
      case 'multiplierField':
        h += '<div class="mult"><input class="inp num" type="number" tabindex="-1" value="1"><span class="mult-x">× ' + esc(f.multipl) +
          '</span><output class="mult-res">' + esc(f.pre || '') + '<b>' + esc(Number(f.multipl) || 0) + '</b>' + esc(f.post || '') + '</output></div>';
        break;
      default:
        h += '<input class="inp" type="' + esc(f.subtype || 'text') + '" tabindex="-1" placeholder="' + esc(f.placeholder || '') + '">';
    }
    return h + statHTML(f) + desc + '</div>';
  }

  function badges(f) {
    var b = [];
    var role = roleOf(f);
    if (role) b.push('<b class="role">' + esc(t('fields.role.' + role)) + '</b>');
    if (moneyId === f._id) b.push('<b class="money">' + esc(t('fields.role.money')) + '</b>');
    if (!isAddons && (panelMode === 'auto' || panelMode === 'fields') && inPanel(f)) {
      b.push('<b class="panel">' + esc(t(atField(f) ? 'fields.panel_badge_field' : 'fields.panel_badge')) + (f.panel_goal ? ' / ' + esc(f.panel_goal) : '') + '</b>');
    }
    if (isAddons && f.name && f.name.slice(-PERSEAT.length) === PERSEAT) b.push('<b>' + esc(t('fields.perseat_badge')) + '</b>');
    var n = answersOf(f);
    if (n) b.push('<b class="cnt">' + esc(t('fields.answers_badge', n)) + '</b>');
    return b.length ? '<span class="fd-badges">' + b.join('') + '</span>' : '';
  }

  function render() {
    var scroll = canvas.parentNode.scrollTop;
    if (!fields.length) {
      canvas.innerHTML = '<div class="fd-empty"><p>' + esc(t('fields.empty')) + '</p>' +
        (isAddons ? '' : '<button class="btn" data-add="preset-person">' + esc(t('fields.add_person')) + '</button>') + '</div>';
      return;
    }
    var h = '<div class="gform-form">';
    layoutRows().forEach(function (r) {
      h += '<div class="ff-row">';
      r.fields.forEach(function (f) {
        var w = isInput(f.type) ? (f._w || 12) : 12;
        h += '<div class="fd-item w' + w + (f._id === sel ? ' sel' : '') + '" data-id="' + f._id + '" draggable="true" tabindex="0">' +
          '<div class="fd-tools">' +
          '<button type="button" data-act="up" title="' + esc(t('fields.move_up')) + ' (Alt+↑)">↑</button>' +
          '<button type="button" data-act="down" title="' + esc(t('fields.move_down')) + ' (Alt+↓)">↓</button>' +
          '<button type="button" data-act="dup" title="' + esc(t('fields.duplicate')) + '">⧉</button>' +
          '<button type="button" data-act="del" title="' + esc(t('common.delete')) + '">✕</button>' +
          '</div>' + badges(f) + '<div class="fd-inert">' + fieldHTML(f) + '</div></div>';
      });
      h += '</div>';
    });
    if (!isAddons) h += '<div class="gform-foot fd-foot"><button class="btn big" type="button" tabindex="-1">' + esc(t('gform.submit')) + '</button></div>';
    h += '</div>';
    canvas.innerHTML = h;
    canvas.parentNode.scrollTop = scroll;
  }

  function select(id) {
    sel = id;
    canvas.querySelectorAll('.fd-item').forEach(function (el) {
      el.classList.toggle('sel', +el.dataset.id === id);
    });
    renderProps();
  }

  // --- panel ustawień ----------------------------------------------------

  function row(label, control, hint) {
    return '<div class="f fd-f"><label>' + esc(label) + '</label>' + control +
      (hint ? '<small>' + esc(hint) + '</small>' : '') + '</div>';
  }

  function inp(prop, value, attrs) {
    return '<input class="inp" data-prop="' + prop + '" value="' + esc(value) + '" ' + (attrs || '') + '>';
  }

  function check(prop, checked, label, attrs) {
    return '<label class="check fd-check"><input type="checkbox" data-prop="' + prop + '"' + (checked ? ' checked' : '') + ' ' + (attrs || '') + '><span>' + esc(label) + '</span></label>';
  }

  function richEditor(value) {
    return '<div class="fd-richbar" role="toolbar">' +
      '<button type="button" data-cmd="bold" title="' + esc(t('fields.bold')) + '"><b>B</b></button>' +
      '<button type="button" data-cmd="italic" title="' + esc(t('fields.italic')) + '"><i>I</i></button>' +
      '<button type="button" data-cmd="underline" title="' + esc(t('fields.underline')) + '"><u>U</u></button>' +
      '<button type="button" data-cmd="link" title="' + esc(t('fields.link')) + '">🔗</button>' +
      '<button type="button" data-cmd="removeFormat" title="' + esc(t('fields.clear_format')) + '">⌫</button>' +
      '</div><div class="fd-rich inp" contenteditable="true" data-rich="label" role="textbox" aria-multiline="true">' + cleanHTML(value) + '</div>' +
      '<div class="fd-linkrow" hidden><input class="inp" placeholder="https://…" data-linkurl><button type="button" class="btn sm" data-linkok>OK</button></div>';
  }

  function overviewHTML() {
    var have = {};
    fields.forEach(function (f) { var r = roleOf(f); if (r) have[r] = true; });
    var inputs = fields.filter(function (f) { return isInput(f.type); }).length;
    var h = '<h4>' + esc(isAddons ? t('fields.title_addons') : t('fields.overview')) + '</h4>';
    h += '<p class="muted">' + esc(t('fields.overview_count', inputs)) + '</p>';
    if (!isAddons) {
      h += '<ul class="fd-checklist">';
      ['name', 'surname', 'email'].forEach(function (r) {
        h += '<li class="' + (have[r] ? 'ok' : '') + '"><span>' + (have[r] ? '✓' : '–') + '</span>' + esc(t('fields.role.' + r)) +
          (have[r] ? '' : ' <button class="link" data-add="role:' + r + '">' + esc(t('fields.add')) + '</button>') + '</li>';
      });
      h += '<li class="' + (moneyId ? 'ok' : '') + '"><span>' + (moneyId ? '✓' : '–') + '</span>' + esc(t('fields.role.money')) +
        (moneyId ? ': ' + esc(plain(byId(moneyId).label)) : '') + '</li></ul>';
      h += '<p class="muted small">' + esc(have.surname ? t('fields.roles_hint') : t('fields.anon_hint')) + '</p>';
    } else {
      h += '<p class="muted small">' + esc(t('fields.addons_hint')) + '</p>';
    }
    h += '<h5>' + esc(t('fields.shortcuts')) + '</h5><p class="muted small">' + esc(t('fields.shortcuts_text')) + '</p>';
    return h;
  }

  function renderProps() {
    var f = byId(sel);
    if (!f) {
      props.innerHTML = overviewHTML();
      return;
    }
    var type = f.type;
    var h = '<div class="fd-ph"><h4>' + esc(t('fields.type.' + (type === 'text' && f.subtype === 'tel' ? 'tel' : type))) + '</h4>' +
      '<button class="link" data-deselect>' + esc(t('common.close')) + '</button></div>';

    // zamiana rodzaju w obrębie zgodnej grupy (opcje / zwykłe pola)
    var group = CHOICE.indexOf(type) >= 0 ? CHOICE : (PLAIN.indexOf(type) >= 0 && !roleOf(f) ? PLAIN : null);
    if (group && !isAddons) {
      var o = '';
      group.forEach(function (g) { o += '<option value="' + g + '"' + (g === type ? ' selected' : '') + '>' + esc(t('fields.type.' + g)) + '</option>'; });
      h += row(t('fields.kind'), '<select class="inp" data-convert>' + o + '</select>');
    }

    var labelName = type === 'header' ? t('fields.header_text') : type === 'paragraph' ? t('fields.paragraph_text') : t('fields.label');
    h += row(labelName, richEditor(f.label || ''));

    if (type === 'header') {
      var lv = f.subtype === 'h3' ? 'h3' : (f.subtype === 'h4' || f.subtype === 'h5') ? 'h4' : 'h2';
      h += row(t('fields.header_size'), '<div class="seg" data-seg="subtype">' +
        [['h2', t('fields.size_l')], ['h3', t('fields.size_m')], ['h4', t('fields.size_s')]].map(function (x) {
          return '<button type="button" data-v="' + x[0] + '"' + (lv === x[0] ? ' class="on"' : '') + '>' + esc(x[1]) + '</button>';
        }).join('') + '</div>');
    }

    if (isInput(type)) {
      h += row(t('fields.description'), inp('description', f.description || ''), t('fields.description_hint'));
      var role = roleOf(f);
      h += check('required', f.required || !!role, t('fields.required'), role ? 'disabled' : '');

      if (type === 'text' && !isAddons) {
        var ro = '<option value="">' + esc(t('fields.role.none')) + '</option>';
        ['name', 'surname', 'email'].forEach(function (r) {
          ro += '<option value="' + r + '"' + (role === r ? ' selected' : '') + '>' + esc(t('fields.role.' + r)) + '</option>';
        });
        h += row(t('fields.role_label'), '<select class="inp" data-role>' + ro + '</select>', t('fields.role_hint'));
        if (!role || role !== 'email') {
          h += row(t('fields.text_kind'), '<div class="seg" data-seg="subtype">' +
            [['text', t('fields.type.text')], ['email', t('fields.role.email')], ['tel', t('fields.type.tel')]].map(function (x) {
              return '<button type="button" data-v="' + x[0] + '"' + ((f.subtype || 'text') === x[0] ? ' class="on"' : '') + '>' + esc(x[1]) + '</button>';
            }).join('') + '</div>');
        }
      }

      if (type === 'text' || type === 'textarea' || type === 'number') {
        h += row(t('fields.placeholder'), inp('placeholder', f.placeholder || ''), t('fields.placeholder_hint'));
      }
      if (type === 'text' || type === 'textarea') {
        h += row(t('fields.maxlength'), inp('maxlength', f.maxlength || '', 'type="number" min="1" inputmode="numeric"'));
      }
      if (type === 'textarea') {
        h += row(t('fields.rows'), inp('rows', f.rows || 3, 'type="number" min="1" max="30"'));
      }
      if (type === 'number' || type === 'date') {
        var it = type === 'date' ? 'type="date"' : 'type="number"';
        h += '<div class="fd-grid3">' +
          row(t('fields.min'), inp('min', f.min === undefined ? '' : f.min, it)) +
          row(t('fields.max'), inp('max', f.max === undefined ? '' : f.max, it)) +
          (type === 'number' ? row(t('fields.step'), inp('step', f.step || '', 'type="number" min="0" step="any"')) : '') +
          '</div>';
        if (type === 'number') h += row(t('fields.default_value'), inp('value', f.value || '', 'type="number"'));
      }

      if (CHOICE.indexOf(type) >= 0) h += optionsHTML(f);
      if (type === 'radio-group' || type === 'checkbox-group') {
        h += check('inline', f.inline, t('fields.inline'));
        h += check('other', f.other, t('fields.other'));
      }
      if (type === 'select') h += check('multiple', f.multiple, t('fields.multiple'));

      if (type === 'multiplierField') {
        h += '<div class="fd-box">' +
          row(t('fields.price'), inp('multipl', f.multipl || '', 'type="number" min="0" step="any" inputmode="decimal"'), t('fields.price_hint')) +
          '<div class="fd-grid2">' + row(t('fields.before'), inp('pre', f.pre || '')) + row(t('fields.after'), inp('post', f.post || '')) + '</div>' +
          '<div class="fd-cur">' + ['Kč', '€', 'zł'].map(function (c) {
            return '<button type="button" class="btn quiet sm" data-currency="' + c + '">' + c + '</button>';
          }).join('') + '</div>' +
          '<p class="fd-example muted">' + esc(t('fields.example')) + ' 2 × ' + esc(f.multipl || 0) + ' → ' +
          esc(f.pre || '') + '<b>' + esc(2 * (Number(f.multipl) || 0)) + '</b>' + esc(f.post || '') + '</p></div>';
      }

      if ((type === 'number' || type === 'multiplierField') && !isAddons) {
        h += check('_money', moneyId === f._id, t('fields.is_money'));
        h += '<small class="fd-under muted">' + esc(t('fields.is_money_hint')) + '</small>';
      }

      if (isAddons && type !== 'multiplierField') {
        var ps = f.name && f.name.slice(-PERSEAT.length) === PERSEAT;
        h += check('_perseat', ps, t('fields.perseat'));
        h += '<small class="fd-under muted">' + esc(t('fields.perseat_hint')) + '</small>';
      }

      if (!isAddons) h += panelHTML(f);

      // szerokość i wiersz
      h += row(t('fields.width'), '<div class="seg fd-widths" data-width>' + WIDTHS.map(function (w) {
        return '<button type="button" data-v="' + w[0] + '"' + ((f._w || 12) === w[0] ? ' class="on"' : '') + ' title="' + esc(t('fields.width_' + w[0])) + '">' + w[1] + '</button>';
      }).join('') + '</div>', t('fields.width_hint'));
      if ((f._w || 12) < 12) h += check('_nr', f._nr, t('fields.new_row'));

      // zaawansowane: klucz pola
      var n = answersOf(f);
      h += '<details class="fd-adv"' + (f._keyOpen ? ' open' : '') + '><summary>' + esc(t('fields.advanced')) + '</summary>' +
        row(t('fields.key'), inp('_name', f.name, 'spellcheck="false" autocomplete="off"' + (role ? ' disabled' : '')),
          role ? t('fields.key_role') : t('fields.key_hint')) +
        '<p class="fd-snip" data-fn="Field"><code>{{.Field "' + esc(f.name) + '"}}</code> <button class="link" data-copy="{{.Field &quot;' + esc(f.name) + '&quot;}}">' + esc(t('fields.copy')) + '</button></p>' +
        (type === 'multiplierField' ? '<p class="fd-snip" data-fn="MTotal"><code>{{.MTotal "' + esc(f.name) + '"}}</code> <button class="link" data-copy="{{.MTotal &quot;' + esc(f.name) + '&quot;}}">' + esc(t('fields.copy')) + '</button></p>' : '') +
        (n ? '<p class="muted small">' + esc(t('fields.key_answers', n)) + '</p>' : '') +
        '<small class="fd-keyerr miss" hidden></small></details>';
    }

    h += '<div class="fd-actions"><button class="btn quiet sm" data-act-sel="dup">' + esc(t('fields.duplicate')) + '</button>' +
      '<button class="btn danger sm" data-act-sel="del">' + esc(t('fields.delete_field')) + '</button></div>';
    props.innerHTML = h;
  }

  function panelHTML(f) {
    var on = inPanel(f);
    var h = '<div class="fd-box fd-panel"><h5>' + esc(t('fields.panel_title')) + '</h5>' +
      check('_panel', on, t('fields.panel_show'));
    if (on) {
      var place = f.panel_place === 'field' ? 'field' : 'panel';
      h += row(t('fields.panel_place'), '<div class="seg" data-place>' + ['panel', 'field'].map(function (v) {
        return '<button type="button" data-v="' + v + '"' + (place === v ? ' class="on"' : '') + '>' + esc(t('fields.panel_place_' + v)) + '</button>';
      }).join('') + '</div>', t('fields.panel_place_hint'));
      h += '<p class="fd-place-all muted">' + esc(t('fields.panel_place_all')) + ' ' +
        '<button type="button" class="link" data-place-all="panel">' + esc(t('fields.panel_place_panel')) + '</button> · ' +
        '<button type="button" class="link" data-place-all="field">' + esc(t('fields.panel_place_field')) + '</button></p>';
      h += row(t('fields.panel_goal'), inp('panel_goal', f.panel_goal || '', 'type="number" min="0" inputmode="numeric"'), t('fields.panel_goal_hint'));
      if (CHOICE.indexOf(f.type) >= 0) {
        var o = '<option value="">' + esc(t('fields.panel_all')) + '</option>';
        (f.values || []).forEach(function (v) {
          o += '<option value="' + esc(v.label) + '"' + (f.panel_only === v.label ? ' selected' : '') + '>' + esc(v.label) + '</option>';
        });
        h += row(t('fields.panel_only'), '<select class="inp" data-panel-only>' + o + '</select>', t('fields.panel_only_hint'));
      }
    }
    h += '<small class="muted">' + esc(t('fields.panel_hint_' + panelMode)) + '</small></div>';
    return h;
  }

  function optionsHTML(f) {
    var single = f.type !== 'checkbox-group' && !(f.type === 'select' && f.multiple);
    var h = '<div class="f fd-f"><label>' + esc(t('fields.options')) + '</label><div class="fd-opts">';
    (f.values || []).forEach(function (o, i) {
      h += '<div class="fd-opt" data-i="' + i + '">' +
        '<input type="' + (single ? 'radio' : 'checkbox') + '" name="fd-optsel" data-opt-sel' + (o.selected ? ' checked' : '') + ' title="' + esc(t('fields.option_default')) + '">' +
        '<input class="inp" data-opt-label value="' + esc(o.label) + '" aria-label="' + esc(t('fields.option')) + ' ' + (i + 1) + '">' +
        '<button type="button" data-opt-up title="' + esc(t('fields.move_up')) + '">↑</button>' +
        '<button type="button" data-opt-del title="' + esc(t('common.delete')) + '">✕</button></div>';
    });
    h += '</div><div class="fd-optbar"><button type="button" class="link" data-opt-add>+ ' + esc(t('fields.option_add')) + '</button>' +
      '<button type="button" class="link" data-opt-paste>' + esc(t('fields.option_paste')) + '</button>' +
      (single ? '<button type="button" class="link" data-opt-nodefault>' + esc(t('fields.option_nodefault')) + '</button>' : '') +
      '</div><small>' + esc(t('fields.option_hint')) + '</small></div>';
    return h;
  }

  // --- zmiany ustawień ---------------------------------------------------

  function changed(key, rerenderProps) {
    commit(key);
    render();
    if (rerenderProps) renderProps();
  }

  function setLabel(f, html) {
    f.label = html;
    if (f._new && !f._nameEdited && isInput(f.type) && !roleOf(f)) {
      var suffix = isAddons && f.name && f.name.slice(-PERSEAT.length) === PERSEAT ? PERSEAT : '';
      f.name = uniqueName((slug(html) || f.type) + suffix, f._id);
      var k = props.querySelector('[data-prop="_name"]');
      if (k) k.value = f.name;
    }
  }

  function setRole(f, role) {
    var old = roleOf(f);
    if (role === old) return;
    if (role) {
      // inne pole z tą rolą wraca do zwykłego
      fields.forEach(function (x) {
        if (x !== f && x.name === ROLE[role]) x.name = uniqueName(slug(x.label) || 'pole', x._id);
      });
      f.name = ROLE[role];
      f.required = true;
      if (role === 'email') f.subtype = 'email';
      else if (f.subtype === 'email') f.subtype = 'text';
      if (!plain(f.label) || f.label === t('fields.new.text')) f.label = t('fields.role.' + role);
    } else {
      f.name = uniqueName(slug(f.label) || 'pole', f._id);
    }
  }

  function setKey(f, value, errEl) {
    value = value.trim();
    var ok = /^[\p{L}0-9_.-]{1,80}$/u.test(value);
    var msg = '';
    if (!ok) msg = t('fields.key_bad');
    else if (nameTaken(value, f._id)) msg = t('fields.key_taken');
    else if (value === ROLE.name || value === ROLE.surname || value === ROLE.email) msg = t('fields.key_role_reserved');
    if (errEl) { errEl.textContent = msg; errEl.hidden = !msg; }
    if (msg) return false;
    f.name = value;
    f._nameEdited = true;
    return true;
  }

  function convert(f, type) {
    if (type === f.type) return;
    var from = f.type;
    f.type = type;
    if (CHOICE.indexOf(type) >= 0) {
      delete f.subtype;
      if (type === 'select') {
        f.className = 'form-control';
        f.multiple = false;
        delete f.inline; delete f.other; delete f.toggle;
      } else {
        delete f.multiple;
        if (from === 'select') delete f.className;
        f.inline = !!f.inline;
        f.other = !!f.other;
        if (type === 'checkbox-group') f.toggle = false; else delete f.toggle;
      }
      if (type !== 'checkbox-group') {
        var seen = false;
        (f.values || []).forEach(function (o) { if (o.selected) { if (seen) o.selected = false; seen = true; } });
      }
    } else {
      f.subtype = type === 'date' ? undefined : type;
      if (f.subtype === undefined) delete f.subtype;
      if (type !== 'number') { delete f.min; delete f.max; delete f.step; }
      if (type !== 'textarea') delete f.rows;
      if (type === 'textarea' && !f.rows) f.rows = 3;
      if (type !== 'number' && moneyId === f._id) moneyId = 0;
    }
  }

  props.addEventListener('input', function (e) {
    var f = byId(sel);
    if (!f) return;
    var el = e.target;

    if (el.matches('.fd-rich')) {
      setLabel(f, cleanHTML(el.innerHTML));
      changed('label:' + f._id);
      return;
    }
    if (el.matches('[data-opt-label]')) {
      var i = +el.closest('.fd-opt').dataset.i;
      var o = f.values[i];
      if (!o.value || o.value === o.label) o.value = el.value;
      o.label = el.value;
      changed('opt:' + f._id + ':' + i);
      return;
    }
    var prop = el.dataset.prop;
    if (!prop || el.type === 'checkbox') return;
    if (prop === '_name') {
      if (setKey(f, el.value, props.querySelector('.fd-keyerr'))) {
        props.querySelectorAll('.fd-snip').forEach(function (p) {
          var fn = p.dataset.fn;
          var snip = '{{.' + fn + ' "' + f.name + '"}}';
          p.querySelector('code').textContent = snip;
          p.querySelector('[data-copy]').dataset.copy = snip;
        });
        changed('key:' + f._id);
      }
      return;
    }
    if (el.value === '') delete f[prop];
    else f[prop] = el.type === 'number' && prop !== 'multipl' && prop !== 'value' && prop !== 'min' && prop !== 'max' && prop !== 'step' ? Number(el.value) : el.value;
    changed(prop + ':' + f._id);
    if (prop === 'multipl' || prop === 'pre' || prop === 'post') updateExample(f);
  });

  function updateExample(f) {
    var ex = props.querySelector('.fd-example');
    if (!ex) return;
    ex.innerHTML = esc(t('fields.example')) + ' 2 × ' + esc(f.multipl || 0) + ' → ' + esc(f.pre || '') +
      '<b>' + esc(2 * (Number(f.multipl) || 0)) + '</b>' + esc(f.post || '');
  }

  props.addEventListener('change', function (e) {
    var f = byId(sel);
    if (!f) return;
    var el = e.target;

    if (el.matches('[data-convert]')) { convert(f, el.value); changed('', true); return; }
    if (el.matches('[data-role]')) { setRole(f, el.value); changed('', true); return; }
    if (el.matches('[data-panel-only]')) {
      if (el.value) f.panel_only = el.value; else delete f.panel_only;
      changed('', true);
      return;
    }
    if (el.matches('[data-opt-sel]')) {
      var i = +el.closest('.fd-opt').dataset.i;
      if (el.type === 'radio') f.values.forEach(function (o, j) { o.selected = j === i; });
      else f.values[i].selected = el.checked;
      changed('');
      return;
    }
    var prop = el.dataset.prop;
    if (!prop || el.type !== 'checkbox') return;
    switch (prop) {
      case '_money':
        moneyId = el.checked ? f._id : (moneyId === f._id ? 0 : moneyId);
        break;
      case '_perseat':
        var base = f.name.slice(-PERSEAT.length) === PERSEAT ? f.name.slice(0, -PERSEAT.length) : f.name;
        f.name = el.checked ? uniqueName(base + PERSEAT, f._id) : uniqueName(base, f._id);
        break;
      case '_nr':
        f._nr = el.checked;
        break;
      case '_panel':
        f.panel = el.checked;
        break;
      case 'multiple':
        f.multiple = el.checked;
        if (!el.checked) {
          var seen = false;
          (f.values || []).forEach(function (o) { if (o.selected) { if (seen) o.selected = false; seen = true; } });
        }
        break;
      default:
        f[prop] = el.checked;
    }
    changed('', true);
  });

  var savedRange = null;
  props.addEventListener('mousedown', function (e) {
    // przyciski paska etykiety nie mogą zabierać zaznaczenia z edytora
    if (e.target.closest('.fd-richbar button')) e.preventDefault();
  });

  props.addEventListener('paste', function (e) {
    if (!e.target.closest('.fd-rich')) return;
    // wklejamy sam tekst - formatowanie z Worda czy strony www zostaje za drzwiami
    e.preventDefault();
    var text = (e.clipboardData || window.clipboardData).getData('text/plain');
    document.execCommand('insertText', false, text.replace(/\s*\n\s*/g, ' '));
  });

  props.addEventListener('keydown', function (e) {
    if (e.target.matches('.fd-rich') && e.key === 'Enter' && !e.shiftKey) {
      // Enter to nowy wiersz w etykiecie (<br>), nie nowy <div>
      e.preventDefault();
      document.execCommand('insertLineBreak');
    }
    if (e.target.matches('[data-opt-label]') && e.key === 'Enter') {
      e.preventDefault();
      addOption(byId(sel), +e.target.closest('.fd-opt').dataset.i + 1);
    }
    if (e.target.matches('[data-linkurl]') && e.key === 'Enter') {
      e.preventDefault();
      applyLink();
    }
  });

  function applyLink() {
    var row = props.querySelector('.fd-linkrow');
    var url = row.querySelector('input').value.trim();
    row.hidden = true;
    var ed = props.querySelector('.fd-rich');
    ed.focus();
    if (savedRange) {
      var s = window.getSelection();
      s.removeAllRanges();
      s.addRange(savedRange);
    }
    if (url) {
      if (!/^(https?:|mailto:|tel:)/i.test(url)) url = (url.indexOf('@') > 0 ? 'mailto:' : 'https://') + url;
      document.execCommand('createLink', false, url);
    } else {
      document.execCommand('unlink');
    }
    setLabel(byId(sel), cleanHTML(ed.innerHTML));
    changed('');
  }

  function addOption(f, at) {
    var n = (f.values || []).length + 1;
    var label = t('fields.new.option', n);
    f.values = f.values || [];
    f.values.splice(at === undefined ? f.values.length : at, 0, {label: label, value: label, selected: false});
    changed('', true);
    var inputs = props.querySelectorAll('[data-opt-label]');
    var el = inputs[at === undefined ? inputs.length - 1 : at];
    if (el) { el.focus(); el.select(); }
  }

  props.addEventListener('click', function (e) {
    var f = byId(sel);
    var b = e.target.closest('button, [data-deselect]');
    if (!b) return;

    if (b.matches('[data-add]')) { addField(b.dataset.add); return; }
    if (b.matches('[data-deselect]')) { select(0); return; }
    if (!f) return;

    if (b.matches('[data-cmd]')) {
      var ed = props.querySelector('.fd-rich');
      if (b.dataset.cmd === 'link') {
        var s = window.getSelection();
        savedRange = s.rangeCount ? s.getRangeAt(0).cloneRange() : null;
        var row = props.querySelector('.fd-linkrow');
        row.hidden = false;
        var a = s.anchorNode && s.anchorNode.parentElement && s.anchorNode.parentElement.closest('a');
        row.querySelector('input').value = a ? a.getAttribute('href') : '';
        row.querySelector('input').focus();
        return;
      }
      ed.focus();
      document.execCommand(b.dataset.cmd, false, null);
      if (b.dataset.cmd === 'removeFormat') document.execCommand('unlink');
      setLabel(f, cleanHTML(ed.innerHTML));
      changed('');
      return;
    }
    if (b.matches('[data-linkok]')) { applyLink(); return; }

    if (b.closest('[data-seg]')) {
      f[b.closest('[data-seg]').dataset.seg] = b.dataset.v;
      changed('', true);
      return;
    }
    if (b.closest('[data-place]')) {
      if (b.dataset.v === 'field') f.panel_place = 'field'; else delete f.panel_place;
      changed('', true);
      return;
    }
    if (b.matches('[data-place-all]')) {
      // wszystkie pola ze stanem naraz - admin szybko porówna oba układy
      fields.forEach(function (x) {
        if (!inPanel(x)) return;
        if (b.dataset.placeAll === 'field') x.panel_place = 'field'; else delete x.panel_place;
      });
      changed('', true);
      return;
    }
    if (b.closest('[data-width]')) {
      f._w = +b.dataset.v;
      changed('', true);
      return;
    }
    if (b.matches('[data-currency]')) {
      f.pre = '(';
      f.post = ' ' + b.dataset.currency + ')';
      changed('', true);
      return;
    }
    if (b.matches('[data-opt-add]')) { addOption(f); return; }
    if (b.matches('[data-opt-nodefault]')) {
      (f.values || []).forEach(function (o) { o.selected = false; });
      changed('', true);
      return;
    }
    if (b.matches('[data-opt-del]')) {
      f.values.splice(+b.closest('.fd-opt').dataset.i, 1);
      changed('', true);
      return;
    }
    if (b.matches('[data-opt-up]')) {
      var i = +b.closest('.fd-opt').dataset.i;
      if (i > 0) {
        var tmp = f.values[i - 1];
        f.values[i - 1] = f.values[i];
        f.values[i] = tmp;
        changed('', true);
      }
      return;
    }
    if (b.matches('[data-opt-paste]')) {
      dialog(t('fields.option_paste'), '<p class="muted">' + esc(t('fields.option_paste_hint')) + '</p><textarea class="inp" rows="8" id="fd-paste"></textarea>',
        t('fields.option_paste_ok'), function () {
          var lines = document.getElementById('fd-paste').value.split('\n').map(function (s) { return s.trim(); }).filter(Boolean);
          f.values = f.values || [];
          lines.forEach(function (l) { f.values.push({label: l, value: l, selected: false}); });
          changed('', true);
        });
      setTimeout(function () { document.getElementById('fd-paste').focus(); }, 30);
      return;
    }
    if (b.matches('[data-copy]')) {
      if (navigator.clipboard) navigator.clipboard.writeText(b.dataset.copy);
      RZ.toast(t('fields.copied'));
      return;
    }
    if (b.matches('[data-act-sel]')) { act(b.dataset.actSel, f._id); }
  });

  props.addEventListener('toggle', function (e) {
    if (e.target.matches('.fd-adv')) {
      var f = byId(sel);
      if (f) f._keyOpen = e.target.open;
    }
  }, true);

  // --- akcje na polach ---------------------------------------------------

  function act(what, id) {
    var i = indexOfId(id);
    if (i < 0) return;
    var f = fields[i];
    switch (what) {
      case 'up':
        if (i > 0) { fields.splice(i, 1); fields.splice(i - 1, 0, f); changed(''); }
        break;
      case 'down':
        if (i < fields.length - 1) { fields.splice(i, 1); fields.splice(i + 1, 0, f); changed(''); }
        break;
      case 'dup':
        var c = JSON.parse(JSON.stringify(f));
        c._id = ++uid;
        c._orig = '';
        c._new = true;
        c._nameEdited = false;
        c._nr = false;
        if (isInput(c.type)) {
          if (roleOf(c)) {
            c.name = uniqueName(slug(c.label) || 'pole', c._id);
            c.required = false;
          } else {
            c.name = uniqueName(f.name.replace(/-\d+$/, ''), c._id);
          }
        }
        fields.splice(i + 1, 0, c);
        sel = c._id;
        changed('', true);
        break;
      case 'del':
        var n = answersOf(f);
        var doDelete = function () {
          fields.splice(i, 1);
          if (moneyId === f._id) moneyId = 0;
          if (sel === f._id) sel = 0;
          changed('', true);
          RZ.toast(t('fields.deleted', plain(f.label) || t('fields.type.' + f.type)), {label: t('fields.undo'), action: undo});
        };
        if (n) {
          dialog(t('fields.delete_title'), '<p>' + esc(t('fields.delete_answers', plain(f.label), n)) + '</p>', t('fields.delete_field'), doDelete, true);
        } else {
          doDelete();
        }
        break;
    }
    if (what === 'up' || what === 'down') {
      var item = canvas.querySelector('[data-id="' + id + '"]');
      if (item) item.focus({preventScroll: false});
    }
  }

  canvas.addEventListener('click', function (e) {
    var add = e.target.closest('[data-add]');
    if (add) {
      if (add.dataset.add === 'preset-person') {
        ['role:name', 'role:surname', 'role:email'].forEach(function (k) {
          var f = newField(k);
          if (f) fields.push(f);
        });
        sel = 0;
        changed('', true);
      }
      return;
    }
    var tool = e.target.closest('[data-act]');
    var item = e.target.closest('.fd-item');
    if (!item) {
      if (e.target === canvas || e.target.closest('.gform-form') === e.target) select(0);
      return;
    }
    var id = +item.dataset.id;
    if (tool) { act(tool.dataset.act, id); return; }
    select(id);
  });

  canvas.addEventListener('dblclick', function (e) {
    if (!e.target.closest('.fd-item')) return;
    var ed = props.querySelector('.fd-rich');
    if (ed) ed.focus();
  });

  // --- przeciąganie ------------------------------------------------------

  document.getElementById('fd-pal').addEventListener('click', function (e) {
    var b = e.target.closest('[data-add]');
    if (b) addField(b.dataset.add);
  });

  document.getElementById('fd-pal').addEventListener('dragstart', function (e) {
    var b = e.target.closest('[data-add]');
    if (!b) return;
    dragSrc = {add: b.dataset.add};
    e.dataTransfer.effectAllowed = 'copy';
    e.dataTransfer.setData('text/plain', b.dataset.add);
  });

  canvas.addEventListener('dragstart', function (e) {
    var item = e.target.closest('.fd-item');
    if (!item) return;
    dragSrc = {move: +item.dataset.id};
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', item.dataset.id);
    setTimeout(function () { item.classList.add('dragging'); }, 0);
  });

  function clearDrop() {
    canvas.querySelectorAll('.drop-before, .drop-after').forEach(function (el) {
      el.classList.remove('drop-before', 'drop-after', 'horiz');
    });
    canvas.classList.remove('drop-end');
  }

  // ownRow stawia pole fields[i] w osobnym wierszu: nie dokleja się do
  // poprzedniego i następne pole też się do niego nie dokleja
  function ownRow(i) {
    fields[i]._nr = true;
    var nx = fields[i + 1];
    if (nx && isInput(nx.type)) nx._nr = true;
  }

  // ROWZONE - pas przy górnej/dolnej krawędzi wiersza z polami obok siebie,
  // gdzie upuszczenie wstawia pole jako nowy wiersz nad/pod nim
  var ROWZONE = 14;

  function dropTarget(e) {
    var items = Array.prototype.slice.call(canvas.querySelectorAll('.fd-item'));
    if (!items.length) return {index: 0, el: null};
    var best = null, bestD = Infinity;
    items.forEach(function (it) {
      var r = it.getBoundingClientRect();
      var dx = e.clientX < r.left ? r.left - e.clientX : e.clientX > r.right ? e.clientX - r.right : 0;
      var dy = e.clientY < r.top ? r.top - e.clientY : e.clientY > r.bottom ? e.clientY - r.bottom : 0;
      var d = dx * dx + dy * dy;
      if (d < bestD) { bestD = d; best = {el: it, r: r}; }
    });
    var horiz = best.el.parentNode.children.length > 1 || !best.el.classList.contains('w12');
    var before = horiz ? e.clientX < best.r.left + best.r.width / 2 : e.clientY < best.r.top + best.r.height / 2;
    var idx = indexOfId(+best.el.dataset.id);
    if (horiz) {
      var row = best.el.parentNode, rr = row.getBoundingClientRect();
      var up = e.clientY < rr.top + ROWZONE, down = e.clientY > rr.bottom - ROWZONE;
      if (up || down) {
        var edge = row.children[up ? 0 : row.children.length - 1];
        var ei = indexOfId(+edge.dataset.id);
        return {index: up ? ei : ei + 1, el: row, before: up, row: true};
      }
    }
    return {index: before ? idx : idx + 1, el: best.el, before: before, horiz: horiz};
  }

  canvas.addEventListener('dragover', function (e) {
    if (!dragSrc) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = dragSrc.add ? 'copy' : 'move';
    clearDrop();
    var tg = dropTarget(e);
    if (tg.el) {
      tg.el.classList.add(tg.before ? 'drop-before' : 'drop-after');
      if (tg.horiz) tg.el.classList.add('horiz');
    }
  });

  canvas.addEventListener('dragleave', function (e) {
    if (!canvas.contains(e.relatedTarget)) clearDrop();
  });

  canvas.addEventListener('drop', function (e) {
    if (!dragSrc) return;
    e.preventDefault();
    var tg = dropTarget(e);
    clearDrop();
    if (dragSrc.add) {
      addField(dragSrc.add, tg.index, tg.row);
    } else {
      var from = indexOfId(dragSrc.move);
      if (from >= 0) {
        var f = fields[from];
        var to = tg.index > from ? tg.index - 1 : tg.index;
        if (to !== from || tg.horiz || tg.row) {
          // upuszczone obok innego pola - staje w jego wierszu; przed polem
          // zaczynającym wiersz przejmuje jego „nowy wiersz”
          var tf = tg.el && byId(+tg.el.dataset.id);
          if (tg.horiz) {
            f._nr = !!(tg.before && tf && tf !== f && tf._nr);
            if (f._nr) tf._nr = false;
          }
          fields.splice(from, 1);
          fields.splice(to, 0, f);
          if (tg.row) ownRow(to);
          sel = f._id;
          changed('', true);
        }
      }
    }
    dragSrc = null;
  });

  document.addEventListener('dragend', function () {
    dragSrc = null;
    clearDrop();
    canvas.querySelectorAll('.dragging').forEach(function (el) { el.classList.remove('dragging'); });
  });

  // --- okno dialogowe ----------------------------------------------------

  var dlg = document.getElementById('fd-dlg');
  var dlgOK = null;
  function dialog(title, body, okLabel, onOK, danger) {
    dlg.querySelector('#fd-dlg-title').textContent = title;
    dlg.querySelector('.fd-dlg-body').innerHTML = body;
    var ok = document.getElementById('fd-dlg-ok');
    ok.textContent = okLabel;
    ok.className = danger ? 'btn danger' : 'btn';
    dlgOK = onOK;
    dlg.hidden = false;
    if (!body.match(/<textarea|<input/)) ok.focus();
  }
  function closeDialog() { dlg.hidden = true; dlgOK = null; }
  document.getElementById('fd-dlg-ok').addEventListener('click', function () {
    var fn = dlgOK;
    closeDialog();
    if (fn) fn();
  });
  document.getElementById('fd-dlg-cancel').addEventListener('click', closeDialog);
  dlg.addEventListener('click', function (e) { if (e.target === dlg) closeDialog(); });

  // --- zapis -------------------------------------------------------------

  function save() {
    if (btnSave.disabled) return;
    var bad = fields.filter(function (f) { return isInput(f.type) && !f.name; })[0];
    if (bad) { select(bad._id); RZ.toast(t('fields.key_bad')); return; }

    btnSave.disabled = true;
    stateEl.textContent = t('common.loading');
    var selIdx = indexOfId(sel);
    RZ.postJSON('/api/forms/' + formID + '/fields', {content: serialize(), moneyfield: moneyName(), renames: renames()})
      .then(function (resp) {
        counts = resp.counts || {};
        load(resp.content, resp.moneyfield);
        sel = selIdx >= 0 && fields[selIdx] ? fields[selIdx]._id : 0;
        savedSig = signature();
        undoStack = [];
        redoStack = [];
        current = snapshot();
        render();
        renderProps();
        afterChange();
        stateEl.textContent = t('common.saved');
        document.getElementById('fd-draft').hidden = true;
        RZ.toast(t('common.saved'));
      })
      .catch(function (err) {
        stateEl.textContent = '';
        RZ.toast(t('common.save_error') + ': ' + err.message);
      })
      .finally(function () { btnSave.disabled = false; });
  }

  btnSave.addEventListener('click', save);
  btnUndo.addEventListener('click', undo);
  btnRedo.addEventListener('click', redo);

  root.querySelectorAll('[data-view]').forEach(function (b) {
    b.addEventListener('click', function () {
      root.querySelectorAll('[data-view]').forEach(function (x) { x.classList.toggle('on', x === b); });
      canvas.classList.toggle('phone', b.dataset.view === 'phone');
      try { localStorage.setItem('rezerwo.fields.view', b.dataset.view); } catch (e) { /* nic */ }
    });
  });

  function editing(el) {
    return el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName));
  }

  document.addEventListener('keydown', function (e) {
    var mod = e.ctrlKey || e.metaKey;
    if (!dlg.hidden) {
      if (e.key === 'Escape') closeDialog();
      return;
    }
    if (mod && e.key.toLowerCase() === 's') { e.preventDefault(); save(); return; }
    if (mod && !editing(document.activeElement)) {
      if (e.key.toLowerCase() === 'z' && !e.shiftKey) { e.preventDefault(); undo(); return; }
      if ((e.key.toLowerCase() === 'z' && e.shiftKey) || e.key.toLowerCase() === 'y') { e.preventDefault(); redo(); return; }
    }
    if (editing(document.activeElement)) return;
    if (!sel) return;
    if (e.altKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
      e.preventDefault();
      act(e.key === 'ArrowUp' ? 'up' : 'down', sel);
    } else if (e.key === 'Delete') {
      e.preventDefault();
      act('del', sel);
    } else if (e.key === 'Escape') {
      select(0);
    } else if (e.key === 'Enter' && document.activeElement && document.activeElement.classList.contains('fd-item')) {
      select(+document.activeElement.dataset.id);
    }
  });

  canvas.addEventListener('focusin', function (e) {
    var item = e.target.closest('.fd-item');
    if (item && +item.dataset.id !== sel && e.target === item) select(+item.dataset.id);
  });

  window.addEventListener('beforeunload', function (e) {
    if (isDirty()) {
      e.preventDefault();
      e.returnValue = '';
    }
  });

  // --- start -------------------------------------------------------------

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    load(FD_DATA.content, FD_DATA.moneyfield);
    savedSig = signature();

    // niezapisane zmiany sprzed wyjścia ze strony
    var draft = null;
    try { draft = JSON.parse(localStorage.getItem(draftKey) || 'null'); } catch (e) { draft = null; }
    if (draft && draft.base === savedSig && draft.snap) {
      restore(draft.snap);
      uid = fields.reduce(function (m, f) { return Math.max(m, f._id); }, uid);
      var bar = document.getElementById('fd-draft');
      bar.hidden = false;
      document.getElementById('fd-draft-discard').addEventListener('click', function () {
        try { localStorage.removeItem(draftKey); } catch (e) { /* nic */ }
        load(FD_DATA.content, FD_DATA.moneyfield);
        sel = 0;
        current = snapshot();
        undoStack = [];
        redoStack = [];
        bar.hidden = true;
        render();
        renderProps();
        afterChange();
      });
    } else if (draft) {
      try { localStorage.removeItem(draftKey); } catch (e) { /* nic */ }
    }

    try {
      if (localStorage.getItem('rezerwo.fields.view') === 'phone') root.querySelector('[data-view="phone"]').click();
    } catch (e) { /* nic */ }

    current = snapshot();
    render();
    renderProps();
    afterChange();
  });
})();
