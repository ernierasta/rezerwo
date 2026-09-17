// n_event.js - strona imprezy jako harmonijka.
// Każda sekcja ma własne "Zapisz"/"Anuluj" i własne żądanie API.

(function () {
  'use strict';

  var page = document.getElementById('event-page');
  if (!page) return;
  var eventID = page.dataset.eventId;

  var quillToolbar = [
    [{header: [1, 2, 3, false]}],
    ['bold', 'italic', 'underline'],
    ['link'],
    [{list: 'ordered'}, {list: 'bullet'}],
    [{indent: '-1'}, {indent: '+1'}],
    [{color: ['#28a745', '#ffc107', '#fd7e14', '#dc3545', '#403734', '#000000']}],
    [{align: []}],
    ['clean']
  ];

  var editors = {};   // nazwa pola -> Quill
  var dirty = {};     // nazwa sekcji -> czy są niezapisane zmiany

  // Quill tworzymy dopiero, gdy sekcja jest rozwinięta (strona otwiera się szybciej).
  function initEditors(sec) {
    sec.querySelectorAll('.ed[data-editor]').forEach(function (el) {
      var name = el.dataset.editor;
      if (editors[name]) return;
      editors[name] = new Quill(el, {
        modules: {toolbar: quillToolbar, clipboard: {matchVisual: false}},
        theme: 'snow'
      });
      editors[name].on('text-change', function () { markDirty(sec); });
    });
  }

  function markDirty(sec) {
    sec.classList.add('dirty');
    dirty[sec.dataset.sec] = true;
  }

  function clearDirty(sec) {
    sec.classList.remove('dirty');
    delete dirty[sec.dataset.sec];
  }

  function values(sec) {
    var out = {};
    sec.querySelectorAll('[data-field]').forEach(function (el) {
      if (el.type === 'checkbox') out[el.dataset.field] = el.checked;
      else out[el.dataset.field] = el.value;
    });
    sec.querySelectorAll('.ed[data-editor]').forEach(function (el) {
      var q = editors[el.dataset.editor];
      out[el.dataset.editor] = q ? q.root.innerHTML : el.innerHTML;
    });
    return out;
  }

  function save(sec, section, btn) {
    var msg = sec.querySelector('.sec-f .msg');
    btn.disabled = true;
    if (msg) { msg.textContent = RZ.t('common.loading'); msg.classList.remove('bad'); }

    RZ.postJSON('/api/events/' + eventID + '/section', {section: section, values: values(sec)})
      .then(function (resp) {
        clearDirty(sec);
        if (msg) msg.textContent = RZ.t('common.saved');
        RZ.toast(RZ.t('common.saved'));
        if (resp.summary) {
          var span = sec.querySelector('.sec-h > span');
          if (span) {
            span.textContent = resp.summary;
            span.classList.toggle('miss', !!resp.missing);
          }
        }
        if (resp.title) {
          var h = document.querySelector('.dh h3');
          if (h) h.textContent = resp.title;
        }
        if (resp.subtitle) {
          var s = document.querySelector('.dsub');
          if (s) s.textContent = resp.subtitle;
        }
      })
      .catch(function (err) {
        if (msg) { msg.textContent = RZ.t('common.save_error') + ': ' + err.message; msg.classList.add('bad'); }
      })
      .finally(function () { btn.disabled = false; });
  }

  // pola dodatków edytuje na razie stary designer formularzy (formBuilder),
  // który otwiera się POST-em z form-id
  function openFieldDesigner(templateID) {
    var f = document.createElement('form');
    f.method = 'POST';
    f.action = '/admin/formeditor';
    var i = document.createElement('input');
    i.type = 'hidden';
    i.name = 'form-id';
    i.value = templateID;
    f.appendChild(i);
    document.body.appendChild(f);
    f.submit();
  }

  page.addEventListener('sec:open', function (e) { initEditors(e.target); });

  page.addEventListener('input', function (e) {
    var sec = e.target.closest('.sec');
    if (sec) markDirty(sec);
  });
  page.addEventListener('change', function (e) {
    var sec = e.target.closest('.sec');
    if (sec) markDirty(sec);
  });

  page.addEventListener('click', function (e) {
    var saveBtn = e.target.closest('[data-save]');
    if (saveBtn) {
      e.preventDefault();
      save(saveBtn.closest('.sec'), saveBtn.dataset.save, saveBtn);
      return;
    }
    if (e.target.closest('[data-cancel]')) {
      e.preventDefault();
      location.reload();
      return;
    }
    // dodatki: utworzenie definicji i przejście do designera pól
    if (e.target.id === 'addons-create') {
      e.preventDefault();
      var createBtn = e.target;
      createBtn.disabled = true;
      RZ.postJSON('/api/events/' + eventID + '/addons', {})
        .then(function (resp) { openFieldDesigner(resp.id); })
        .catch(function (err) {
          createBtn.disabled = false;
          RZ.toast(RZ.t('common.save_error') + ': ' + err.message);
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
        markDirty(rooms.closest('.sec'));
      }
      return;
    }
    if (e.target.id === 'room-clear') {
      e.preventDefault();
      var r = document.getElementById('f-rooms');
      r.value = '';
      markDirty(r.closest('.sec'));
    }
  });

  window.addEventListener('beforeunload', function (e) {
    if (Object.keys(dirty).length === 0) return;
    e.preventDefault();
    e.returnValue = '';
  });

  RZ.loadLocale(document.documentElement.lang || 'pl').then(function () {
    RZ.accordion(page, 'rezerwo.event.' + eventID + '.sections');
  });
})();
