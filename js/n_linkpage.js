// n_linkpage.js - strona z linkami w panelu (tmpl/n_linkpage.html,
// linkpages.go): harmonijka sekcji, edytor przycisków, kopiowanie adresu
// i odświeżanie kodu QR po zmianie adresu.
//
// Przyciski są w ukrytym polu sekcji (data-field="buttons") jako JSON
// [{kind: "event"|"form"|"url", id, url, label}]; rzędy edytora rysujemy
// z niego i po każdej zmianie zapisujemy z powrotem.

document.addEventListener('DOMContentLoaded', function () {
  'use strict';

  var page = document.getElementById('lpage');
  var id = page.dataset.pageId;
  var t = RZ.t;

  var hidden = document.getElementById('lp-buttons');
  var rowsBox = document.getElementById('lp-rows');
  var addBtn = document.getElementById('lp-add');
  var max = parseInt(rowsBox.dataset.max, 10) || 8;
  var targets = {events: [], forms: []};
  try { targets = JSON.parse(rowsBox.dataset.targets || '{}'); } catch (e) { /* brak list */ }
  targets.events = targets.events || [];
  targets.forms = targets.forms || [];

  var secs = RZ.sections(page, '/api/pages/' + id + '/section', {
    draftKey: 'rezerwo.lpage.' + id + '.draft',
    onSaved: saved
  });

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    var acc = RZ.accordion(page, 'rezerwo.lpage.' + id + '.sections');
    secs.restore(acc); // niezapisane zmiany sprzed wyjścia ze strony
    render();
  });

  // --- po zapisie: adres strony mógł się zmienić -----------------------------
  function saved(section, resp) {
    if (!resp || !resp.url) return;
    document.getElementById('lp-link').textContent = resp.full_url;
    document.getElementById('lp-qr-url').textContent = resp.full_url;
    document.getElementById('lp-open').href = resp.url;
    document.getElementById('lp-qr').src = '/api/pages/' + id + '/qr.png?v=' + Date.now();
  }

  document.getElementById('lp-copy').addEventListener('click', function () {
    var url = document.getElementById('lp-link').textContent.trim();
    if (navigator.clipboard) navigator.clipboard.writeText(url);
    RZ.toast(t('fields.copied'));
  });

  // --- edytor przycisków -------------------------------------------------------

  function read() {
    try {
      var v = JSON.parse(hidden.value || '[]');
      return Array.isArray(v) ? v : [];
    } catch (e) { return []; }
  }

  function write(list) {
    hidden.value = JSON.stringify(list);
    secs.markDirty(hidden.closest('.sec'));
  }

  function find(kind, tid) {
    var list = kind === 'event' ? targets.events : kind === 'form' ? targets.forms : [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].id === tid) return list[i];
    }
    return null;
  }

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text) e.textContent = text;
    return e;
  }

  function option(value, text, selected) {
    var o = el('option', '', text);
    o.value = value;
    if (selected) o.selected = true;
    return o;
  }

  function targetSelect(b) {
    var sel = el('select', 'inp');
    var cur = b.kind === 'url' ? 'url' : (b.kind && b.id ? b.kind + ':' + b.id : '');
    sel.appendChild(option('', t('lpage.pick'), cur === ''));
    // usunięta impreza/formularz: zostaje w wyborze, żeby admin widział, co zmienić
    if ((b.kind === 'event' || b.kind === 'form') && b.id && !find(b.kind, b.id)) {
      sel.appendChild(option(cur, t('lpage.missing'), true));
    }
    [['event', 'lpage.events', targets.events], ['form', 'lpage.forms', targets.forms]].forEach(function (g) {
      if (!g[2].length) return;
      var og = el('optgroup');
      og.label = t(g[1]);
      g[2].forEach(function (x) {
        og.appendChild(option(g[0] + ':' + x.id, x.name + (x.sub ? ' (' + x.sub + ')' : ''), cur === g[0] + ':' + x.id));
      });
      sel.appendChild(og);
    });
    sel.appendChild(option('url', t('lpage.other_url'), cur === 'url'));
    return sel;
  }

  function render() {
    var list = read();
    rowsBox.textContent = '';
    if (!list.length) rowsBox.appendChild(el('p', 'muted', t('lpage.no_buttons')));
    list.forEach(function (b, i) {
      var row = el('div', 'lp-row');
      row.appendChild(el('span', 'lp-n', String(i + 1)));

      var target = el('div', 'lp-target');
      var sel = targetSelect(b);
      sel.setAttribute('aria-label', t('lpage.target'));
      target.appendChild(sel);
      if ((b.kind === 'event' || b.kind === 'form') && b.id && !find(b.kind, b.id)) {
        row.classList.add('bad');
        target.appendChild(el('small', 'miss', t('lpage.missing_hint')));
      }
      row.appendChild(target);

      var lab = el('div', 'lp-label');
      var inp = el('input', 'inp');
      inp.value = b.label || '';
      inp.maxLength = 120;
      var tg = find(b.kind, b.id);
      inp.placeholder = tg ? tg.name : t('lpage.label_ph');
      inp.setAttribute('aria-label', t('lpage.label'));
      lab.appendChild(inp);
      lab.appendChild(el('small', '', b.kind === 'url' ? t('lpage.label_req') : t('lpage.label_hint')));
      row.appendChild(lab);

      var ctl = el('div', 'lp-ctl');
      [['↑', 'lpage.up', -1], ['↓', 'lpage.down', 1]].forEach(function (m) {
        var mb = el('button', 'btn quiet sm', m[0]);
        mb.type = 'button';
        mb.title = t(m[1]);
        mb.setAttribute('aria-label', t(m[1]));
        mb.disabled = (m[2] < 0 && i === 0) || (m[2] > 0 && i === list.length - 1);
        mb.addEventListener('click', function () {
          var l = read();
          var j = i + m[2];
          var x = l[i]; l[i] = l[j]; l[j] = x;
          write(l);
          render();
        });
        ctl.appendChild(mb);
      });
      var del = el('button', 'btn quiet sm', '✕');
      del.type = 'button';
      del.title = t('lpage.remove');
      del.setAttribute('aria-label', t('lpage.remove'));
      del.addEventListener('click', function () {
        var l = read();
        l.splice(i, 1);
        write(l);
        render();
      });
      ctl.appendChild(del);
      row.appendChild(ctl);

      if (b.kind === 'url') {
        var u = el('div', 'lp-url');
        var ui = el('input', 'inp');
        ui.value = b.url || '';
        ui.placeholder = 'https://…';
        ui.spellcheck = false;
        ui.setAttribute('aria-label', t('lpage.url_label'));
        u.appendChild(ui);
        u.appendChild(el('small', '', t('lpage.url_label')));
        ui.addEventListener('input', function () {
          var l = read();
          l[i].url = ui.value.trim();
          write(l);
        });
        row.appendChild(u);
      }

      sel.addEventListener('change', function () {
        var l = read();
        var v = sel.value;
        if (v === 'url') l[i] = {kind: 'url', url: l[i].url || '', label: l[i].label || ''};
        else if (v === '') l[i] = {kind: '', label: l[i].label || ''};
        else {
          var p = v.split(':');
          l[i] = {kind: p[0], id: parseInt(p[1], 10), label: l[i].label || ''};
        }
        write(l);
        render();
      });
      inp.addEventListener('input', function () {
        var l = read();
        l[i].label = inp.value;
        write(l);
      });

      rowsBox.appendChild(row);
    });
    addBtn.disabled = list.length >= max;
  }

  addBtn.addEventListener('click', function () {
    var l = read();
    if (l.length >= max) return;
    l.push({kind: '', label: ''});
    write(l);
    render();
    var sels = rowsBox.querySelectorAll('select');
    if (sels.length) sels[sels.length - 1].focus();
  });
});
