// n_linkpage.js - strona z linkami w panelu (tmpl/n_linkpage.html,
// linkpages.go): harmonijka sekcji, edytor przycisków, kopiowanie adresu
// i odświeżanie kodu QR po zmianie adresu.
//
// Przyciski są w ukrytym polu sekcji (data-field="buttons") jako JSON
// [{kind: "event"|"form"|"url", id, url, label, color, size, nodate, img,
// imgonly}]; rzędy edytora rysujemy z niego i po każdej zmianie zapisujemy
// z powrotem. Wygląd przycisku (kolor, wielkość, data, obrazek) jest pod
// rozwijanym "Wygląd przycisku"; obrazek wybiera ten sam wybór co banner
// (n_ui.js, .bpick).

document.addEventListener('DOMContentLoaded', function () {
  'use strict';

  var page = document.getElementById('lpage');
  var id = page.dataset.pageId;
  var t = RZ.t;

  var hidden = document.getElementById('lp-buttons');
  var rowsBox = document.getElementById('lp-rows');
  var addBtn = document.getElementById('lp-add');
  var max = parseInt(rowsBox.dataset.max, 10) || 8;
  var media = rowsBox.dataset.media || '/';
  // linkpages.go: linkButtonColors, linkButtonSizes
  var COLORS = ['', 'outline', 'ink', 'red', 'orange', 'green', 'blue', 'purple'];
  var SIZES = ['s', '', 'l'];
  var opened = []; // rozwinięty "Wygląd przycisku" w rzędzie i
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
    document.getElementById('lp-qr').src = '/api/pages/' + id + '/qr.svg?v=' + Date.now();
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
          var o = opened[i]; opened[i] = opened[j]; opened[j] = o;
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
        opened.splice(i, 1);
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

      if (b.kind) row.appendChild(lookBox(b, i));

      sel.addEventListener('change', function () {
        var l = read();
        var v = sel.value;
        var old = l[i];
        if (v === 'url') l[i] = {kind: 'url', url: old.url || '', label: old.label || ''};
        else if (v === '') l[i] = {kind: '', label: old.label || ''};
        else {
          var p = v.split(':');
          l[i] = {kind: p[0], id: parseInt(p[1], 10), label: old.label || ''};
        }
        // wygląd zostaje przy zmianie celu
        ['color', 'size', 'img', 'imgonly'].forEach(function (k) { if (old[k]) l[i][k] = old[k]; });
        if (old.nodate && l[i].kind === 'event') l[i].nodate = true;
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
    RZ.bannerPicks(rowsBox);
  }

  // --- wygląd przycisku ------------------------------------------------------

  function update(i, fn, again) {
    var l = read();
    if (!l[i]) return;
    fn(l[i]);
    write(l);
    if (again) render();
  }

  function seg(cls, label) {
    var s = el('div', 'seg ' + cls);
    s.setAttribute('role', 'radiogroup');
    s.setAttribute('aria-label', label);
    return s;
  }

  function lookBox(b, i) {
    var box = el('div', 'lp-look');
    var custom = !!(b.color || b.size || b.nodate || b.img);
    var open = opened[i] === undefined ? false : opened[i];
    var tog = el('button', 'link lp-more', t('lpage.look') + (custom && !open ? ' · ' + t('lpage.look_custom') : ''));
    tog.type = 'button';
    tog.setAttribute('aria-expanded', open ? 'true' : 'false');
    tog.addEventListener('click', function () {
      opened[i] = !open;
      render();
    });
    box.appendChild(tog);
    if (!open) return box;

    var body = el('div', 'lp-look-b');

    // kolor i wielkość nie mają znaczenia, gdy przycisk to sam obrazek
    var plain = el('div', 'lp-o-row' + (b.imgonly && b.img ? ' off' : ''));
    var cbox = el('div', 'lp-o');
    cbox.appendChild(el('span', 'lp-o-l', t('lpage.color')));
    var colors = seg('lp-colors', t('lpage.color'));
    COLORS.forEach(function (c) {
      var cb = el('button', 'lp-sw c-' + (c || 'def'));
      cb.type = 'button';
      cb.appendChild(el('i'));
      var name = t('lpage.color_' + (c || 'def'));
      cb.title = name;
      cb.setAttribute('aria-label', name);
      cb.setAttribute('role', 'radio');
      var on = (b.color || '') === c;
      cb.classList.toggle('on', on);
      cb.setAttribute('aria-checked', on ? 'true' : 'false');
      cb.addEventListener('click', function () { update(i, function (x) { if (c) x.color = c; else delete x.color; }, true); });
      colors.appendChild(cb);
    });
    cbox.appendChild(colors);
    plain.appendChild(cbox);

    var sbox = el('div', 'lp-o');
    sbox.appendChild(el('span', 'lp-o-l', t('lpage.size')));
    var sizes = seg('lp-sizes', t('lpage.size'));
    SIZES.forEach(function (z) {
      var zb = el('button', '', t('lpage.size_' + (z || 'm')));
      zb.type = 'button';
      zb.setAttribute('role', 'radio');
      var on = (b.size || '') === z;
      zb.classList.toggle('on', on);
      zb.setAttribute('aria-checked', on ? 'true' : 'false');
      zb.addEventListener('click', function () { update(i, function (x) { if (z) x.size = z; else delete x.size; }, true); });
      sizes.appendChild(zb);
    });
    sbox.appendChild(sizes);
    plain.appendChild(sbox);
    body.appendChild(plain);

    if (b.kind === 'event') {
      body.appendChild(check(t('lpage.show_date'), !b.nodate, function (on) {
        update(i, function (x) { if (on) delete x.nodate; else x.nodate = true; });
      }));
    }

    // obrazek: ten sam wybór co banner, bez szerokości
    var ibox = el('div', 'lp-o lp-o-img');
    ibox.appendChild(el('span', 'lp-o-l', t('lpage.image')));
    var pick = el('div', 'bpick lp-bpick');
    pick.dataset.base = media;
    var hid = el('input');
    hid.type = 'hidden';
    hid.dataset.field = 'lp_img'; // n_ui.js szuka input[data-field]; serwer to pole pomija
    hid.value = b.img || '';
    pick.appendChild(hid);
    var bb = el('div', 'bp-box');
    var th = el('div', 'bp-thumb');
    var im = el('img');
    im.alt = '';
    im.hidden = true;
    th.appendChild(im);
    th.appendChild(el('span', 'bp-none', t('banner.none')));
    bb.appendChild(th);
    var ctl = el('div', 'bp-ctl');
    var btns = el('div', 'bp-btns');
    [['btn quiet sm bp-up', 'banner.upload'], ['btn quiet sm bp-choose', 'banner.choose'], ['link bp-del', 'banner.remove']].forEach(function (x) {
      var k = el('button', x[0], t(x[1]));
      k.type = 'button';
      btns.appendChild(k);
    });
    ctl.appendChild(btns);
    ctl.appendChild(el('small', 'bp-name'));
    var only = check(t('lpage.image_only'), !!b.imgonly, function (on) {
      update(i, function (x) { if (on) x.imgonly = true; else delete x.imgonly; }, true);
    });
    only.hidden = !b.img;
    ctl.appendChild(only);
    ctl.appendChild(el('small', 'bp-hint', t('lpage.image_hint')));
    bb.appendChild(ctl);
    pick.appendChild(bb);
    var file = el('input', 'bp-file');
    file.type = 'file';
    file.accept = 'image/png,image/jpeg,image/gif,image/webp';
    file.hidden = true;
    pick.appendChild(file);
    hid.addEventListener('input', function () {
      var f = hid.value.split(';')[0].trim();
      update(i, function (x) {
        if (f) x.img = f;
        else { delete x.img; delete x.imgonly; }
      }, !f && b.imgonly);
      only.hidden = !f;
    });
    ibox.appendChild(pick);
    body.appendChild(ibox);

    box.appendChild(body);
    return box;
  }

  function check(text, on, fn) {
    var l = el('label', 'lp-chk');
    var c = el('input');
    c.type = 'checkbox';
    c.checked = on;
    c.addEventListener('change', function () { fn(c.checked); });
    l.appendChild(c);
    l.appendChild(el('span', '', text));
    return l;
  }

  addBtn.addEventListener('click', function () {
    var l = read();
    if (l.length >= max) return;
    l.push({kind: '', label: ''});
    opened[l.length - 1] = false;
    write(l);
    render();
    var sels = rowsBox.querySelectorAll('select');
    if (sels.length) sels[sels.length - 1].focus();
  });
});
