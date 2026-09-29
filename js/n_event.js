// n_event.js - strona imprezy jako harmonijka.
// Każda sekcja ma własne "Zapisz"/"Anuluj" i własne żądanie API; zapis,
// szkic niezapisanych zmian i edytory Quill daje RZ.sections (n_ui.js).
//
// Własne rzeczy imprezy: pastylki danych zamówienia w tekście "Po zamówieniu"
// (n_vars.js), ostrzeżenie o opisie kolorów w tekście pod planem sali,
// dopisywanie sal i dodatki.

(function () {
  'use strict';

  var page = document.getElementById('event-page');
  if (!page) return;
  var eventID = page.dataset.eventId;
  var t = RZ.t;
  var V = RZ.vars;

  // "Po zamówieniu – tekst" jest wykonywany jako szablon z danymi zamówienia
  // (ParseOrderTmpl), więc ma te same pastylki co mail - kod QR jako zwykły
  // obrazek na stronie (.QRPay), bez PDF.
  var ORDERED = 'ordered_note_text';
  var orderedEl = page.querySelector('.ed[data-editor="' + ORDERED + '"]');
  if (orderedEl && V && typeof Quill !== 'undefined') {
    var meta = {};
    try { meta = JSON.parse(orderedEl.dataset.vars || '{}'); } catch (e) { meta = {}; }
    V.setup(meta, '.QRPay');
  }

  var legend = legendCheck();

  var secs = RZ.sections(page, '/api/events/' + eventID + '/section', {
    draftKey: 'rezerwo.event.' + eventID + '.draft',
    editorToolbar: function (name, tb) {
      if (name !== ORDERED) return tb;
      return {
        container: tb.concat([['mailvar']]),
        handlers: {mailvar: V.toolbarHandler(function () { return secs.editors[ORDERED]; })}
      };
    },
    editorIn: function (name, html) { return name === ORDERED ? V.toEditor(html).html : html; },
    editorOut: function (name, q) { return name === ORDERED ? V.fromEditor(q) : RZ.editorHTML(q); },
    onEditor: function (name, q) {
      if (name === ORDERED) V.attach(q);
      if (name === 'how_to') legend.watch(q);
    }
  });

  // --- opis kolorów w tekście pod planem sali ----------------------------------
  //
  // Pod planem sali strona gościa ma automatyczną legendę kolorów zgodną
  // z tematem. Starsze imprezy mają kolory opisane też w tekście ("Zielony =
  // wolne"), który po zmianie tematu kłamie. Niczego nie zmieniamy sami:
  // pokazujemy ostrzeżenie z przyciskiem, który usuwa w edytorze tylko te
  // linijki (można cofnąć Ctrl+Z albo "Anuluj").
  function legendCheck() {
    var box = document.getElementById('legend-warn');
    var themeInp = page.querySelector('[data-sec="strona"] input[data-field="theme"]');
    var startTheme = themeInp ? themeInp.value : '';
    var quill = null;

    // słowo koloru przed "=" / "–" / "-" / ":"
    var COLOR = /(zielon|żółt|zolt|pomarańcz|pomarancz|czerwon|czarn|biał|bial|granatow|szar|niebiesk|fiolet|brąz|braz|zelen|žlut|oranžov|červen|čern|bíl|šed|modr|hněd|green|yellow|orange|red\b|black|white|gr[ae]y|blue|brown|purple)/i;
    var SEP = /\s*(=|–|—|\s-\s|:)\s*/;
    var TITLE = /^\s*(legend[ae]?|legenda planu)\s*:?\s*$/i;

    function colorBlocks(root) {
      var out = [];
      root.querySelectorAll('li, p').forEach(function (el) {
        var txt = el.textContent;
        var m = SEP.exec(txt);
        if (!m || m.index > 60) return;
        if (COLOR.test(txt.slice(0, m.index))) out.push(el);
      });
      return out;
    }

    function update() {
      if (!box || !quill) return;
      var found = colorBlocks(quill.root).length > 0;
      box.hidden = !found;
      if (found) {
        var changed = themeInp && themeInp.value !== startTheme;
        box.querySelector('p').textContent = t(changed ? 'legend.warn_theme' : 'legend.warn');
      }
    }

    function remove() {
      var tpl = document.createElement('template');
      tpl.innerHTML = RZ.editorHTML(quill);
      var root = tpl.content;
      colorBlocks(root).forEach(function (el) {
        var list = el.parentNode;
        el.remove();
        if (list && /^(UL|OL)$/.test(list.tagName) && !list.querySelector('li')) {
          // nagłówek "Legenda:" nad usuniętą listą też znika
          var prev = list.previousElementSibling;
          if (prev && TITLE.test(prev.textContent)) prev.remove();
          list.remove();
        }
      });
      // "Legenda:" bez listy pod spodem (np. opis kolorów był w akapitach)
      root.querySelectorAll('h1, h2, h3, h4, p').forEach(function (el) {
        var next = el.nextElementSibling;
        if (TITLE.test(el.textContent) && (!next || /^H\d$/.test(next.tagName))) el.remove();
      });
      quill.setContents(quill.clipboard.convert({html: tpl.innerHTML}), 'user');
      RZ.toast(t('legend.removed'));
      update();
    }

    if (box) box.querySelector('button').addEventListener('click', function () { if (quill) remove(); });
    page.addEventListener('input', function (e) { if (e.target === themeInp) update(); });

    return {
      watch: function (q) {
        quill = q;
        q.on('text-change', update);
        update();
      }
    };
  }

  // --- sale i dodatki ----------------------------------------------------------

  // pola dodatków edytuje ten sam designer co formularze (forms_fields.go)
  function openFieldDesigner(templateID) {
    location.href = '/admin/forms/' + templateID + '/fields';
  }

  page.addEventListener('click', function (e) {
    // dodatki: utworzenie definicji i przejście do designera pól
    if (e.target.id === 'addons-create') {
      e.preventDefault();
      var createBtn = e.target;
      createBtn.disabled = true;
      RZ.postJSON('/api/events/' + eventID + '/addons', {})
        .then(function (resp) { openFieldDesigner(resp.id); })
        .catch(function (err) {
          createBtn.disabled = false;
          RZ.toast(t('common.save_error') + ': ' + err.message);
        });
      return;
    }
    if (e.target.id === 'addons-edit') {
      e.preventDefault();
      openFieldDesigner(e.target.dataset.template);
      return;
    }
    // sala: dopisywanie i czyszczenie listy sal
    if (e.target.id === 'room-add') {
      e.preventDefault();
      var sel = document.getElementById('room-add-select');
      var rooms = document.getElementById('f-rooms');
      var id = sel.value;
      if (id && id !== '0') {
        var list = rooms.value.split(',').filter(Boolean);
        if (list.indexOf(id) === -1) list.push(id);
        rooms.value = list.join(',');
        secs.markDirty(rooms.closest('.sec'));
      }
      return;
    }
    if (e.target.id === 'room-clear') {
      e.preventDefault();
      var r = document.getElementById('f-rooms');
      r.value = '';
      secs.markDirty(r.closest('.sec'));
    }
  });

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    var acc = RZ.accordion(page, 'rezerwo.event.' + eventID + '.sections');
    secs.restore(acc); // niezapisane zmiany sprzed wyjścia ze strony
  });
})();
