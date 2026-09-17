// n_ui.js - drobne narzędzia nowego panelu: teksty, powiadomienia, harmonijka.
// Bez jQuery i bez Bootstrapa.

(function (global) {
  'use strict';

  var dict = {};

  // t("klucz") - tekst interfejsu z katalogu wczytanego z /locales/{lang}.json.
  function t(key, args) {
    var s = dict[key];
    if (s === undefined) return key;
    if (args === undefined) return s;
    if (!Array.isArray(args)) args = [args];
    var i = 0;
    return s.replace(/%[ds]/g, function () { return args[i++]; });
  }

  function loadLocale(lang) {
    return fetch('/locales/' + lang + '.json')
      .then(function (r) { return r.ok ? r.json() : {}; })
      .then(function (d) { dict = d; return d; })
      .catch(function () { return {}; });
  }

  // toast("Zapisano") albo toast("Oznaczono 3", {label: "Cofnij", action: fn})
  var toastEl = null, toastTimer = null;
  function toast(text, undo) {
    if (toastEl) toastEl.remove();
    toastEl = document.createElement('div');
    toastEl.className = 'toast';
    var span = document.createElement('span');
    span.textContent = text;
    toastEl.appendChild(span);
    if (undo && undo.action) {
      var b = document.createElement('button');
      b.textContent = undo.label || 'Cofnij';
      b.addEventListener('click', function () { undo.action(); hideToast(); });
      toastEl.appendChild(b);
    }
    document.body.appendChild(toastEl);
    clearTimeout(toastTimer);
    toastTimer = setTimeout(hideToast, undo ? 8000 : 3000);
  }

  function hideToast() {
    if (toastEl) { toastEl.remove(); toastEl = null; }
  }

  // postJSON zwraca obiekt odpowiedzi; rzuca błąd z komunikatem serwera.
  function postJSON(url, data, method) {
    return fetch(url, {
      method: method || 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(data)
    }).then(function (r) {
      return r.text().then(function (txt) {
        var body = {};
        try { body = txt ? JSON.parse(txt) : {}; } catch (e) { body = {msg: txt}; }
        if (!r.ok) throw new Error(body.msg || ('HTTP ' + r.status));
        return body;
      });
    });
  }

  // Szkic formularza: to, co admin wpisał, ale jeszcze nie zapisał.
  // Trzymamy go w przeglądarce, żeby dało się wyjść do innego ekranu
  // (np. do designera sali) i wrócić do tych samych wartości. Do bazy
  // trafia dopiero po kliknięciu "Zapisz".
  function draft(key) {
    function all() {
      try { return JSON.parse(localStorage.getItem(key) || '{}') || {}; } catch (e) { return {}; }
    }
    function write(obj) {
      try {
        if (Object.keys(obj).length === 0) localStorage.removeItem(key);
        else localStorage.setItem(key, JSON.stringify(obj));
      } catch (e) { /* prywatne okno - trudno, szkicu nie będzie */ }
    }
    return {
      all: all,
      get: function (section) { return all()[section]; },
      save: function (section, values) { var o = all(); o[section] = values; write(o); },
      clear: function (section) { var o = all(); delete o[section]; write(o); },
      clearAll: function () { write({}); },
      sections: function () { return Object.keys(all()); }
    };
  }

  // applyValues wstawia zapamiętane wartości z powrotem w pola sekcji.
  // Edytory Quill dostają treść przez clipboard, żeby Quill wiedział o zmianie.
  function applyValues(sec, values, editors) {
    if (!values) return;
    sec.querySelectorAll('[data-field]').forEach(function (el) {
      var v = values[el.dataset.field];
      if (v === undefined) return;
      if (el.type === 'checkbox') el.checked = !!v;
      else el.value = v;
    });
    sec.querySelectorAll('.ed[data-editor]').forEach(function (el) {
      var name = el.dataset.editor;
      var v = values[name];
      if (v === undefined) return;
      var q = editors && editors[name];
      if (q) q.clipboard.dangerouslyPasteHTML(v);
      else el.innerHTML = v;
    });
  }

  // draftNotice pokazuje na górze strony, że wartości pochodzą ze szkicu.
  function draftNotice(onDiscard) {
    var doc = document.querySelector('.doc');
    if (!doc || doc.querySelector('.draft-notice')) return;

    var bar = document.createElement('div');
    bar.className = 'draft-notice';

    var text = document.createElement('span');
    text.textContent = t('draft.restored');
    bar.appendChild(text);

    var btn = document.createElement('button');
    btn.className = 'link';
    btn.textContent = t('draft.discard');
    btn.addEventListener('click', function () { onDiscard(); });
    bar.appendChild(btn);

    doc.insertBefore(bar, doc.firstChild);
  }

  // Harmonijka: rozwijanie sekcji, zapamiętany stan, adres z #sekcją.
  function accordion(root, storageKey) {
    var open = {};
    try { open = JSON.parse(localStorage.getItem(storageKey) || '{}') || {}; } catch (e) { open = {}; }

    function store() {
      try { localStorage.setItem(storageKey, JSON.stringify(open)); } catch (e) { /* prywatne okno */ }
    }

    function setOpen(sec, isOpen) {
      sec.classList.toggle('open', isOpen);
      open[sec.dataset.sec] = isOpen;
      var act = sec.querySelector('.sec-h .act');
      if (act) act.textContent = isOpen ? t('common.collapse') : (act.dataset.label || act.textContent);
      var body = sec.querySelector('.sec-b');
      if (body) body.hidden = !isOpen;
      if (isOpen) sec.dispatchEvent(new CustomEvent('sec:open', {bubbles: true}));
      store();
    }

    root.querySelectorAll('.sec').forEach(function (sec) {
      var head = sec.querySelector('.sec-h');
      var act = sec.querySelector('.sec-h .act');
      if (act && !act.dataset.label) act.dataset.label = act.textContent;
      if (!sec.querySelector('.sec-b')) return; // sekcja tylko z linkiem
      head.addEventListener('click', function (e) {
        if (e.target.tagName === 'A' && e.target.getAttribute('href') && !e.target.classList.contains('act')) return;
        e.preventDefault();
        setOpen(sec, !sec.classList.contains('open'));
      });
      setOpen(sec, !!open[sec.dataset.sec]);
    });

    var hash = location.hash.replace('#', '');
    if (hash) {
      var target = root.querySelector('.sec[data-sec="' + hash + '"]');
      if (target) {
        setOpen(target, true);
        target.scrollIntoView({block: 'start'});
      }
    }

    return {
      open: function (name) {
        var sec = root.querySelector('.sec[data-sec="' + name + '"]');
        if (sec && sec.querySelector('.sec-b')) setOpen(sec, true);
        return sec;
      }
    };
  }

  // katalog wczytujemy od razu, żeby teksty w JS działały na każdej stronie
  // (strony, które same wołają loadLocale, dostaną go z pamięci przeglądarki)
  var ready = loadLocale(document.documentElement.lang || 'pl');

  // Sekcje harmonijki z zapisem: edytory Quill tworzone przy rozwinięciu sekcji,
  // znacznik niezapisanych zmian, zapis jednej sekcji jednym żądaniem.
  // saveURL dostaje {section: "...", values: {...}} i oddaje {summary, missing}.
  function sections(root, saveURL, opts) {
    opts = opts || {};
    var editors = {};
    var dirty = {};
    // szkic niezapisanych zmian tej strony (patrz draft() wyżej)
    var drafts = draft(opts.draftKey || ('rezerwo.draft:' + saveURL));

    var toolbar = [
      [{header: [1, 2, 3, false]}],
      ['bold', 'italic', 'underline'],
      ['link'],
      [{list: 'ordered'}, {list: 'bullet'}],
      [{indent: '-1'}, {indent: '+1'}],
      [{color: ['#28a745', '#ffc107', '#fd7e14', '#dc3545', '#403734', '#000000']}],
      [{align: []}],
      ['clean']
    ];

    function initEditors(sec) {
      if (typeof Quill === 'undefined') return;
      sec.querySelectorAll('.ed[data-editor]').forEach(function (el) {
        var name = el.dataset.editor;
        if (editors[name]) return;
        editors[name] = new Quill(el, {
          modules: {toolbar: toolbar, clipboard: {matchVisual: false}},
          theme: 'snow'
        });
        editors[name].on('text-change', function () { markDirty(sec); });
      });
    }

    function markDirty(sec) {
      sec.classList.add('dirty');
      dirty[sec.dataset.sec] = true;
      drafts.save(sec.dataset.sec, values(sec));
    }

    function values(sec) {
      var out = {};
      sec.querySelectorAll('[data-field]').forEach(function (el) {
        out[el.dataset.field] = el.type === 'checkbox' ? el.checked : el.value;
      });
      sec.querySelectorAll('.ed[data-editor]').forEach(function (el) {
        var q = editors[el.dataset.editor];
        out[el.dataset.editor] = q ? q.root.innerHTML : el.innerHTML;
      });
      return out;
    }

    root.addEventListener('sec:open', function (e) { initEditors(e.target); });
    root.addEventListener('input', function (e) {
      var sec = e.target.closest('.sec');
      if (sec) markDirty(sec);
    });
    root.addEventListener('change', function (e) {
      var sec = e.target.closest('.sec');
      if (sec) markDirty(sec);
    });

    root.addEventListener('click', function (e) {
      var btn = e.target.closest('[data-save]');
      if (btn) {
        e.preventDefault();
        var sec = btn.closest('.sec');
        var msg = sec.querySelector('.sec-f .msg');
        btn.disabled = true;
        if (msg) { msg.textContent = t('common.loading'); msg.classList.remove('bad'); }

        postJSON(saveURL, {section: btn.dataset.save, values: values(sec)})
          .then(function (resp) {
            sec.classList.remove('dirty');
            delete dirty[sec.dataset.sec];
            drafts.clear(sec.dataset.sec);
            if (msg) msg.textContent = t('common.saved');
            toast(t('common.saved'));
            var span = sec.querySelector('.sec-h > span');
            if (span && resp.summary) {
              span.textContent = resp.summary;
              span.classList.toggle('miss', !!resp.missing);
            }
            if (resp.title) {
              var h = document.querySelector('.dh h3');
              if (h) h.textContent = resp.title;
            }
            if (resp.subtitle) {
              var s = document.querySelector('.dsub');
              if (s) s.textContent = resp.subtitle;
            }
            if (opts.onSaved) opts.onSaved(btn.dataset.save, resp);
          })
          .catch(function (err) {
            if (msg) { msg.textContent = t('common.save_error') + ': ' + err.message; msg.classList.add('bad'); }
          })
          .finally(function () { btn.disabled = false; });
        return;
      }
      if (e.target.closest('[data-cancel]')) {
        e.preventDefault();
        // "Anuluj" wyrzuca szkic tej sekcji i wraca do wartości z bazy
        var cancelled = e.target.closest('.sec');
        if (cancelled) drafts.clear(cancelled.dataset.sec);
        location.reload();
      }
    });

    return {
      markDirty: markDirty,
      editors: editors,

      // restore przywraca niezapisane zmiany po powrocie na stronę.
      // Wołać po zbudowaniu harmonijki, z jej uchwytem.
      restore: function (acc) {
        var pending = drafts.sections();
        if (!pending.length) return;

        var restored = false;
        pending.forEach(function (name) {
          var sec = acc && acc.open ? acc.open(name) : root.querySelector('.sec[data-sec="' + name + '"]');
          if (!sec) {
            drafts.clear(name); // sekcji już nie ma
            return;
          }
          initEditors(sec);
          applyValues(sec, drafts.get(name), editors);
          sec.classList.add('dirty');
          dirty[name] = true;
          restored = true;
        });

        if (restored) {
          draftNotice(function () {
            drafts.clearAll();
            location.reload();
          });
        }
      }
    };
  }

  global.RZ = {
    t: t,
    ready: ready,
    sections: sections,
    loadLocale: loadLocale,
    toast: toast,
    postJSON: postJSON,
    accordion: accordion,
    draft: draft,
    applyValues: applyValues,
    draftNotice: draftNotice
  };
})(window);
