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

  // editorDOM porządkuje kopię treści edytora Quill 2 przed zapisem: listy
  // Quill trzyma jako <ol><li data-list="bullet">, a maile i strony gości
  // potrzebują zwykłych <ul>/<ol> (takich, jakie dawał Quill 1). Wcięcia
  // zostają jako klasy ql-indent-N. Zmienia węzeł el w miejscu.
  function editorDOM(el) {
    el.querySelectorAll('.ql-ui').forEach(function (n) { n.remove(); });
    el.querySelectorAll('ol').forEach(function (ol) {
      var items = Array.prototype.slice.call(ol.children);
      if (!items.some(function (li) { return li.hasAttribute('data-list'); })) return;
      var frag = document.createDocumentFragment();
      var cur = null, curTag = '';
      items.forEach(function (li) {
        var tag = li.getAttribute('data-list') === 'ordered' ? 'ol' : 'ul';
        if (tag !== curTag) {
          cur = document.createElement(tag);
          frag.appendChild(cur);
          curTag = tag;
        }
        li.removeAttribute('data-list');
        cur.appendChild(li);
      });
      ol.replaceWith(frag);
    });
    return el;
  }

  // Quill 2 przy wczytywaniu HTML zamienia &nbsp; na zwykłe spacje, przez co
  // wcięcia i "sklejone" daty w zapisanych treściach by się rozjechały.
  // Ten matcher (po wbudowanym) przywraca twarde spacje tam, gdzie były.
  function nbspMatcher(node, delta) {
    var data = node.data || '';
    if (data.indexOf('\u00a0') < 0 || delta.ops.length !== 1 || typeof delta.ops[0].insert !== 'string') return delta;
    var text = delta.ops[0].insert, out = '', j = 0;
    for (var i = 0; i < text.length; i++) {
      var ch = text[i];
      if (ch !== ' ') {
        var k = data.indexOf(ch, j);
        if (k >= 0) j = k + 1;
        out += ch;
        continue;
      }
      while (j < data.length && !/\s/.test(data[j])) j++;
      if (data[j] === '\u00a0') {
        out += '\u00a0';
        j++;
      } else {
        out += ' ';
        while (j < data.length && /[^\S\u00a0]/.test(data[j])) j++;
      }
    }
    delta.ops[0].insert = out;
    return delta;
  }

  // quillModules - wspólna konfiguracja modułów edytora.
  function quillModules(toolbar) {
    return {toolbar: toolbar, clipboard: {matchers: [[Node.TEXT_NODE, nbspMatcher]]}};
  }

  // editorHTML - HTML do zapisu z edytora Quill (patrz editorDOM).
  function editorHTML(q) {
    return editorDOM(q.root.cloneNode(true)).innerHTML;
  }

  // applyValues wstawia zapamiętane wartości z powrotem w pola sekcji.
  // Edytory Quill dostają treść przez clipboard, żeby Quill wiedział o zmianie.
  // editorIn (opcjonalne) zamienia zapisany HTML na treść edytora (pastylki).
  function applyValues(sec, values, editors, editorIn) {
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
      if (editorIn) v = editorIn(name, v);
      if (q) q.clipboard.dangerouslyPasteHTML(v);
      else el.innerHTML = v;
    });
    syncThemePick(sec);
    syncBannerPicks(sec);
  }

  // Wybór tematu strony gościa (szablon "themepick" w n_base.html): próbki
  // zapisują klucz w ukrytym polu sekcji, input oznacza sekcję jako zmienioną.
  function syncThemePick(root) {
    root.querySelectorAll('.theme-pick').forEach(function (box) {
      var inp = box.parentNode.querySelector('input[data-field="theme"]');
      box.querySelectorAll('.tp').forEach(function (b) {
        var on = b.dataset.v === inp.value;
        b.classList.toggle('on', on);
        b.setAttribute('aria-checked', on ? 'true' : 'false');
      });
    });
  }

  document.addEventListener('click', function (e) {
    var b = e.target.closest('.theme-pick .tp');
    if (!b) return;
    var inp = b.closest('.f').querySelector('input[data-field="theme"]');
    if (inp.value === b.dataset.v) return;
    inp.value = b.dataset.v;
    syncThemePick(b.closest('.f'));
    inp.dispatchEvent(new Event('input', {bubbles: true}));
  });

  // Wybór bannera (szablon "bannerpick" w n_base.html, banner.go). Ukryte
  // pole trzyma wartość do bazy: "plik.png" (auto = cała szerokość) albo
  // "plik.png;50%". Zmiana wartości wysyła input, który oznacza sekcję jako
  // zmienioną.
  var BANNER_WIDTHS = [100, 75, 50, 33];

  function parseBanner(v) {
    var parts = (v || '').split(';');
    var w = parts.length === 2 ? parseInt(parts[1], 10) : 100;
    return {file: parts[0].trim(), w: BANNER_WIDTHS.indexOf(w) >= 0 ? w : 100};
  }

  function bannerURL(box, file) {
    return box.dataset.base + file.split('/').map(encodeURIComponent).join('/');
  }

  function syncBannerPick(box) {
    var b = parseBanner(box.querySelector('input[data-field]').value);
    var img = box.querySelector('.bp-thumb img');
    var none = box.querySelector('.bp-none');
    var name = box.querySelector('.bp-name');
    box.classList.toggle('empty', !b.file);
    box.classList.remove('missing');
    name.textContent = b.file;
    if (b.file) {
      img.onerror = function () {
        box.classList.add('missing');
        img.hidden = true;
        none.hidden = false;
        none.textContent = t('banner.missing');
      };
      img.src = bannerURL(box, b.file);
      img.hidden = false;
      none.hidden = true;
    } else {
      img.removeAttribute('src');
      img.hidden = true;
      none.hidden = false;
      none.textContent = t('banner.none');
    }
    img.style.width = b.w + '%';
    box.querySelectorAll('.bp-w button').forEach(function (btn) {
      var on = +btn.dataset.w === b.w;
      btn.classList.toggle('on', on);
      btn.setAttribute('aria-checked', on ? 'true' : 'false');
      btn.disabled = !b.file;
    });
    box.querySelector('.bp-del').hidden = !b.file;
  }

  function syncBannerPicks(root) {
    root.querySelectorAll('.bpick').forEach(syncBannerPick);
  }

  function setBanner(box, file, w) {
    var inp = box.querySelector('input[data-field]');
    inp.value = !file ? '' : (w === 100 ? file : file + ';' + w + '%');
    syncBannerPick(box);
    inp.dispatchEvent(new Event('input', {bubbles: true}));
  }

  function uploadBanner(box, file) {
    var name = box.querySelector('.bp-name');
    var prev = name.textContent;
    name.textContent = t('banner.uploading');
    var fd = new FormData();
    fd.append('file', file);
    fetch('/api/media/banners', {method: 'POST', body: fd})
      .then(function (r) {
        return r.json().catch(function () { return {}; }).then(function (body) {
          if (!r.ok) throw new Error(body.msg || ('HTTP ' + r.status));
          return body;
        });
      })
      .then(function (resp) {
        setBanner(box, resp.name, parseBanner(box.querySelector('input[data-field]').value).w);
      })
      .catch(function (err) {
        name.textContent = prev;
        toast(err.message);
      });
  }

  // okienko z miniaturkami wgranych obrazków
  var bpMenu = null;
  function closeBannerMenu() {
    if (bpMenu) bpMenu.remove();
    bpMenu = null;
    document.removeEventListener('mousedown', bpOutside, true);
    document.removeEventListener('keydown', bpEsc, true);
  }
  function bpOutside(e) { if (bpMenu && !bpMenu.contains(e.target)) closeBannerMenu(); }
  function bpEsc(e) { if (e.key === 'Escape') closeBannerMenu(); }

  function chooseBanner(box, anchor) {
    closeBannerMenu();
    fetch('/api/media/banners')
      .then(function (r) { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
      .then(function (files) {
        closeBannerMenu();
        bpMenu = document.createElement('div');
        bpMenu.className = 'bp-menu';
        bpMenu.setAttribute('role', 'listbox');
        var current = parseBanner(box.querySelector('input[data-field]').value);
        if (!files.length) {
          var p = document.createElement('p');
          p.className = 'muted';
          p.textContent = t('banner.no_files');
          bpMenu.appendChild(p);
        }
        files.forEach(function (f) {
          var b = document.createElement('button');
          b.type = 'button';
          b.setAttribute('role', 'option');
          if (f.name === current.file) b.classList.add('on');
          var img = document.createElement('img');
          img.src = f.url;
          img.alt = '';
          img.loading = 'lazy';
          var s = document.createElement('span');
          s.textContent = f.name;
          b.appendChild(img);
          b.appendChild(s);
          b.addEventListener('click', function () {
            closeBannerMenu();
            setBanner(box, f.name, current.w);
          });
          bpMenu.appendChild(b);
        });
        document.body.appendChild(bpMenu);
        var r = anchor.getBoundingClientRect();
        var left = Math.min(r.left + window.scrollX, window.scrollX + document.documentElement.clientWidth - bpMenu.offsetWidth - 8);
        bpMenu.style.left = Math.max(8, left) + 'px';
        bpMenu.style.top = (r.bottom + window.scrollY + 4) + 'px';
        setTimeout(function () {
          document.addEventListener('mousedown', bpOutside, true);
          document.addEventListener('keydown', bpEsc, true);
        }, 0);
        var first = bpMenu.querySelector('button');
        if (first) first.focus();
      })
      .catch(function (err) { toast(err.message); });
  }

  document.addEventListener('click', function (e) {
    var box = e.target.closest('.bpick');
    if (!box) return;
    if (e.target.closest('.bp-up')) {
      box.querySelector('.bp-file').click();
    } else if (e.target.closest('.bp-choose')) {
      chooseBanner(box, e.target.closest('.bp-choose'));
    } else if (e.target.closest('.bp-del')) {
      setBanner(box, '', 100);
    } else if (e.target.closest('.bp-w button')) {
      var cur = parseBanner(box.querySelector('input[data-field]').value);
      if (cur.file) setBanner(box, cur.file, +e.target.closest('button').dataset.w);
    }
  });

  document.addEventListener('change', function (e) {
    if (!e.target.classList.contains('bp-file')) return;
    e.stopPropagation(); // sam wybór pliku nie zmienia jeszcze wartości
    var f = e.target.files && e.target.files[0];
    if (f) uploadBanner(e.target.closest('.bpick'), f);
    e.target.value = '';
  }, true);

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
      // "Zwiń" w pasku Zapisz/Anuluj - pasek jest przyklejony do dołu ekranu,
      // więc w długiej sekcji nie trzeba przewijać do nagłówka
      var foot = sec.querySelector('.sec-b > .sec-f');
      if (foot && !foot.querySelector('.sec-collapse')) {
        var fold = document.createElement('button');
        fold.type = 'button';
        fold.className = 'link sec-collapse';
        fold.textContent = t('common.collapse');
        fold.addEventListener('click', function (e) {
          e.preventDefault();
          setOpen(sec, false);
          head.scrollIntoView({block: 'nearest'});
        });
        foot.appendChild(fold);
      }
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
  ready.then(function () { syncBannerPicks(document); });

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
        // opts.editorToolbar(name, toolbar) może dopisać własne przyciski,
        // opts.editorIn / opts.editorOut zamieniają treść przy wczytaniu i zapisie
        var initial = el.innerHTML;
        var tb = opts.editorToolbar ? opts.editorToolbar(name, toolbar) : toolbar;
        editors[name] = new Quill(el, {
          modules: quillModules(tb),
          theme: 'snow'
        });
        if (opts.editorIn) {
          var html = opts.editorIn(name, initial);
          if (html !== initial) {
            editors[name].setContents([]);
            editors[name].clipboard.dangerouslyPasteHTML(html);
            editors[name].history.clear();
          }
        }
        if (opts.onEditor) opts.onEditor(name, editors[name]);
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
        if (q && opts.editorOut) out[el.dataset.editor] = opts.editorOut(el.dataset.editor, q);
        else out[el.dataset.editor] = q ? editorHTML(q) : el.innerHTML;
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
          applyValues(sec, drafts.get(name), editors, opts.editorIn);
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
    draftNotice: draftNotice,
    editorDOM: editorDOM,
    quillModules: quillModules,
    editorHTML: editorHTML
  };
})(window);
