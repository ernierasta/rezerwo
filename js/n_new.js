// n_new.js - okna "Nowy mail", "Nowe konto", "Nowa sala".
// Wszystkie wyglądają tak samo: po lewej lista (pusty, gotowe szablony,
// własne do skopiowania), po prawej podgląd wybranego, pod spodem nazwa.
// Otwierane z katalogów i przyciskami "Nowy…" przy wyborach na stronie
// imprezy i formularza (RZNew.wire) - tam nowy element od razu się wybiera,
// sekcja zapisuje i otwiera się jego edytor z powrotem na tę stronę.
//
// Źródła: GET /api/{mails,accounts,rooms}/sources,
// utworzenie: POST /api/{mails,accounts,rooms} {name, copy_of | template}.

(function (global) {
  'use strict';

  var SVGNS = 'http://www.w3.org/2000/svg';

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  function nextName(name) {
    // "Bal 2025" -> "Bal 2026", bez roku "Konto (kopia)"
    var n = String(name).replace(/(\d{4})/, function (y) { return String(Number(y) + 1); });
    return n === name ? RZ.t('formpage.copy_name', name) : n;
  }

  function getJSON(url) {
    return fetch(url).then(function (r) {
      return r.json().then(function (d) { if (!r.ok) throw new Error(d.msg || r.status); return d; });
    });
  }

  // dialog buduje okno. cfg:
  //   title, kinds ([[klucz, etykieta]] - przełącznik nad listą), kind,
  //   items(kind) -> [{head: "nagłówek"} | {empty: "tekst"} | {type, item, title, sub}],
  //   preview(p, box, current) - rysuje podgląd wybranego p,
  //   name(p) - podpowiedź nazwy, nameNeeded - komunikat bez nazwy,
  //   create(name, p, kind) -> obietnica {id},
  //   done(id, name) - co dalej (może zwrócić obietnicę),
  //   extra(rerender) -> element nad listą (np. wybór formularza), widoczny
  //   tylko dla rodzaju extraFor; check(kind) -> komunikat, gdy czegoś brak.
  function dialog(cfg) {
    var kind = cfg.kind;
    var picked = null;

    var back = el('div', 'dlg-back');
    var dlg = el('div', 'dlg dlg-wide nm');
    dlg.setAttribute('role', 'dialog');
    dlg.setAttribute('aria-modal', 'true');
    back.appendChild(dlg);

    dlg.appendChild(el('h3', '', cfg.title));

    var seg = null;
    if (cfg.kinds) {
      seg = el('div', 'seg');
      cfg.kinds.forEach(function (k) {
        var b = el('button', k[0] === kind ? 'on' : '', k[1]);
        b.type = 'button';
        b.dataset.kind = k[0];
        seg.appendChild(b);
      });
      dlg.appendChild(seg);
      seg.addEventListener('click', function (e) {
        var b = e.target.closest('button');
        if (!b || b.dataset.kind === kind) return;
        kind = b.dataset.kind;
        seg.querySelectorAll('button').forEach(function (x) { x.classList.toggle('on', x === b); });
        syncExtra();
        renderList();
      });
    }

    var extra = cfg.extra ? cfg.extra(function () { renderList(); }) : null;
    if (extra) dlg.appendChild(extra);
    function syncExtra() { if (extra) extra.hidden = !!cfg.extraFor && cfg.extraFor !== kind; }
    syncExtra();

    var body = el('div', 'nm-body');
    var list = el('div', 'nm-list');
    var prev = el('div', 'nm-prev');
    body.appendChild(list);
    body.appendChild(prev);
    dlg.appendChild(body);

    var nameF = el('div', 'f');
    var nameL = el('label', '', RZ.t('field.name'));
    var nameI = el('input', 'inp');
    nameI.id = 'nm-name';
    nameI.autocomplete = 'off';
    nameL.htmlFor = 'nm-name';
    nameF.appendChild(nameL);
    nameF.appendChild(nameI);
    dlg.appendChild(nameF);

    var foot = el('div', 'sec-f');
    var create = el('button', 'btn', RZ.t('newmail.create'));
    var cancel = el('button', 'btn quiet', RZ.t('common.cancel'));
    var msg = el('small', 'msg');
    foot.appendChild(create);
    foot.appendChild(cancel);
    foot.appendChild(msg);
    dlg.appendChild(foot);

    function renderList() {
      list.textContent = '';
      var first = null;
      cfg.items(kind).forEach(function (it) {
        if (it.head) { list.appendChild(el('h5', '', it.head)); return; }
        if (it.empty) { list.appendChild(el('p', 'muted', it.empty)); return; }
        var b = el('button', 'nm-item');
        b.type = 'button';
        b.appendChild(el('b', '', it.title));
        if (it.sub) b.appendChild(el('small', '', it.sub));
        var p = {type: it.type, item: it.item};
        b.addEventListener('click', function () { pick(b, p); });
        list.appendChild(b);
        if (!first) first = {b: b, p: p};
      });
      if (first) pick(first.b, first.p);
    }

    function pick(btn, p) {
      picked = p;
      list.querySelectorAll('.nm-item').forEach(function (b) { b.classList.toggle('on', b === btn); });
      nameI.value = cfg.name(p);
      prev.textContent = '';
      cfg.preview(p, prev, function () { return picked === p; });
    }

    function done() {
      back.remove();
      document.removeEventListener('keydown', esc);
    }
    function esc(e) { if (e.key === 'Escape') done(); }
    cancel.addEventListener('click', done);
    back.addEventListener('click', function (e) { if (e.target === back) done(); });
    document.addEventListener('keydown', esc);
    nameI.addEventListener('keydown', function (e) { if (e.key === 'Enter') create.click(); });

    create.addEventListener('click', function () {
      var name = nameI.value.trim();
      var missing = cfg.check ? cfg.check(kind) : '';
      if (missing) {
        msg.textContent = missing;
        msg.classList.add('bad');
        return;
      }
      if (!name) {
        msg.textContent = cfg.nameNeeded;
        msg.classList.add('bad');
        nameI.focus();
        return;
      }
      create.disabled = true;
      msg.classList.remove('bad');
      msg.textContent = RZ.t('common.loading');
      cfg.create(name, picked, kind)
        .catch(function (e) {
          msg.textContent = RZ.t('common.save_error') + ': ' + e.message;
          msg.classList.add('bad');
          create.disabled = false;
          throw e;
        })
        .then(function (r) { return cfg.done(r.id, name); })
        // błąd po utworzeniu (np. zapis sekcji) pokazuje już strona
        .catch(function () { if (create.disabled) done(); });
    });

    renderList();
    document.body.appendChild(back);
    var on = list.querySelector('.nm-item');
    if (on) on.focus();
  }

  function failed(e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); }

  // edytor nowego elementu, z ?back= jeśli przyszliśmy ze strony imprezy itp.
  function goTo(url, opts) {
    return function (id) {
      location.href = url + id + (opts.back ? '?back=' + encodeURIComponent(opts.back) : '');
    };
  }

  // --- mail -------------------------------------------------------------------

  // mail(opts): opts.kind ('events' | 'forms'), opts.lockKind - bez
  // przełącznika, opts.done(id, name), opts.back, opts.formID - formularz,
  // do którego należy nowy mail (strona formularza; z katalogów admin musi
  // go wybrać), opts.audience ('guest' | 'admin') - który szkic z polami
  // formularza pokazać pierwszy.
  function mail(opts) {
    opts = opts || {};
    getJSON('/api/mails/sources').then(function (src) {
      var previews = {};
      var formID = Number(opts.formID) || 0;
      var forms = src.forms || [];

      function formName(id) {
        for (var i = 0; i < forms.length; i++) if (forms[i].id === id) return forms[i].name;
        return '';
      }

      // szkice z wszystkimi polami formularza (FormMailDraftAPI)
      function drafts() {
        if (!formID) return [];
        var order = opts.audience === 'admin' ? ['admin', 'guest'] : ['guest', 'admin'];
        return order.map(function (a) {
          return {type: 'draft', item: {audience: a}, title: RZ.t('newmail.draft_' + a), sub: RZ.t('newmail.draft_hint')};
        });
      }

      dialog({
        title: RZ.t('newmail.title'),
        kind: opts.kind || 'events',
        kinds: opts.lockKind ? null : [['events', RZ.t('catalog.for_events')], ['forms', RZ.t('catalog.for_forms')]],
        nameNeeded: RZ.t('newmail.name_needed'),
        extraFor: 'forms',
        extra: formID ? null : function (rerender) {
          var f = el('div', 'f');
          f.style.marginTop = '14px';
          var l = el('label', '', RZ.t('mail.form'));
          var sel = el('select', 'inp');
          sel.id = 'nm-form';
          l.htmlFor = 'nm-form';
          var o0 = el('option', '', RZ.t('field.select_hint'));
          o0.value = '0';
          sel.appendChild(o0);
          forms.forEach(function (fm) {
            var o = el('option', '', fm.name);
            o.value = String(fm.id);
            sel.appendChild(o);
          });
          sel.addEventListener('change', function () { formID = Number(sel.value) || 0; rerender(); });
          f.appendChild(l);
          f.appendChild(sel);
          f.appendChild(el('small', '', RZ.t('newmail.form_hint')));
          return f;
        },
        check: function (kind) {
          return kind === 'forms' && !formID ? RZ.t('mail.form_needed') : '';
        },
        items: function (kind) {
          var out = [];
          if (kind === 'forms') {
            var d = drafts();
            if (d.length) {
              out.push({head: RZ.t('newmail.from_form', formName(formID) || '')});
              out = out.concat(d);
              out.push({head: RZ.t('newmail.other')});
            }
          }
          out.push({type: 'blank', title: RZ.t('newmail.blank'), sub: RZ.t('newmail.blank_hint')});
          var tpls = src.templates.filter(function (t) { return t.related_to === kind; });
          if (tpls.length) {
            out.push({head: RZ.t('newmail.templates')});
            tpls.forEach(function (t) { out.push({type: 'template', item: t, title: t.name, sub: t.description}); });
          }
          out.push({head: RZ.t('newmail.mine')});
          var mine = src.mine.filter(function (m) {
            return kind === 'forms' ? m.related_to === 'forms' : m.related_to !== 'forms';
          });
          if (!mine.length) out.push({empty: RZ.t('newmail.none_mine')});
          mine.forEach(function (m) { out.push({type: 'mine', item: m, title: m.name, sub: m.subject}); });
          return out;
        },
        name: function (p) {
          if (p.type === 'draft') return RZ.t('newmail.draft_name_' + p.item.audience, formName(formID));
          if (p.type === 'template') return p.item.name;
          if (p.type === 'mine') return nextName(p.item.name);
          return '';
        },
        preview: function (p, box, current) {
          if (p.type === 'blank') {
            box.appendChild(el('p', 'muted', RZ.t('newmail.blank_preview')));
            return;
          }
          // mail formularza pokazujemy z etykietami pól swojego formularza
          var fid = p.item.form_id || formID;
          var key = p.type + ':' + (p.item.key || p.item.id || p.item.audience) + ':' + fid;
          var head = el('div', 'nm-subj');
          var frame = document.createElement('iframe');
          frame.setAttribute('sandbox', '');
          frame.title = RZ.t('mail.preview_title');
          box.appendChild(head);
          box.appendChild(frame);

          var req = previews[key] ? Promise.resolve(previews[key]) : (p.type === 'draft'
            ? getJSON('/api/forms/' + formID + '/maildraft?for=' + p.item.audience).then(function (r) { return r.preview; })
            : RZ.postJSON('/api/mails/preview', {
              subject: p.item.subject || '',
              text: p.item.text || '',
              related_to: p.item.related_to,
              form_id: fid
            })).then(function (r) { previews[key] = r; return r; });

          req.then(function (r) {
            if (!current()) return;
            head.textContent = r.subject;
            var files = r.files && r.files.length ? '<p style="color:#6e7885;font-size:13px">📎 ' + r.files.join(', ').replace(/</g, '&lt;') + '</p>' : '';
            frame.srcdoc = '<html><head><base href="' + location.origin + '/"><style>body{font:14px/1.5 sans-serif;color:#222;margin:12px}p{margin:0}img{max-width:100%}</style></head><body>' + r.html + files + '</body></html>';
          }).catch(function (e) {
            if (!current()) return;
            box.textContent = '';
            box.appendChild(el('p', 'miss', e.message));
          });
        },
        create: function (name, p, kind) {
          var data = {name: name, related_to: kind, form_id: kind === 'forms' ? formID : 0};
          if (p.type === 'draft') data.template = 'form:' + p.item.audience;
          if (p.type === 'template') data.template = p.item.key;
          if (p.type === 'mine') data.copy_of = p.item.id;
          return RZ.postJSON('/api/mails', data);
        },
        done: opts.done || goTo('/admin/mails/', opts)
      });
    }).catch(failed);
  }

  // --- konto bankowe -------------------------------------------------------------

  function account(opts) {
    opts = opts || {};
    getJSON('/api/accounts/sources').then(function (src) {
      dialog({
        title: RZ.t('catalog.new_account'),
        nameNeeded: RZ.t('newacc.name_needed'),
        items: function () {
          var out = [{type: 'blank', title: RZ.t('newacc.blank'), sub: RZ.t('newacc.blank_hint')}];
          out.push({head: RZ.t('newacc.mine')});
          if (!src.mine.length) out.push({empty: RZ.t('newacc.none_mine')});
          src.mine.forEach(function (a) { out.push({type: 'mine', item: a, title: a.name, sub: a.iban}); });
          return out;
        },
        name: function (p) { return p.type === 'mine' ? nextName(p.item.name) : ''; },
        preview: function (p, box) {
          if (p.type === 'blank') {
            box.appendChild(el('p', 'muted', RZ.t('newacc.blank_preview')));
            return;
          }
          box.appendChild(el('p', 'muted', RZ.t('newacc.copy_preview')));
          var dl = el('dl', 'nm-dl');
          [['account.iban', p.item.iban], ['account.recipient', p.item.recipient],
            ['field.currency', p.item.currency], ['account.varsymbol', p.item.varsymbol || '']].forEach(function (r) {
            dl.appendChild(el('dt', '', RZ.t(r[0])));
            dl.appendChild(el('dd', '', r[1] || '—'));
          });
          box.appendChild(dl);
        },
        create: function (name, p) {
          return RZ.postJSON('/api/accounts', {name: name, copy_of: p.type === 'mine' ? p.item.id : 0});
        },
        done: opts.done || goTo('/admin/accounts/', opts)
      });
    }).catch(failed);
  }

  // --- sala ----------------------------------------------------------------------

  // roomSVG - miniatura planu sali z prostokątów z /api/rooms/sources
  function roomSVG(room) {
    var svg = document.createElementNS(SVGNS, 'svg');
    svg.setAttribute('viewBox', '0 0 ' + room.width + ' ' + room.height);
    svg.setAttribute('class', 'nm-room');
    var bg = document.createElementNS(SVGNS, 'rect');
    bg.setAttribute('width', room.width);
    bg.setAttribute('height', room.height);
    bg.setAttribute('class', 'bg');
    svg.appendChild(bg);
    room.shapes.forEach(function (s) {
      var n;
      if (s.e) {
        n = document.createElementNS(SVGNS, 'ellipse');
        n.setAttribute('cx', s.x + s.w / 2);
        n.setAttribute('cy', s.y + s.h / 2);
        n.setAttribute('rx', s.w / 2);
        n.setAttribute('ry', s.h / 2);
      } else {
        n = document.createElementNS(SVGNS, 'rect');
        n.setAttribute('x', s.x);
        n.setAttribute('y', s.y);
        n.setAttribute('width', s.w);
        n.setAttribute('height', s.h);
        n.setAttribute('rx', s.t === 'chair' ? 4 : 2);
      }
      n.setAttribute('class', s.t);
      if (s.c && s.t !== 'label') n.style.fill = s.c;
      svg.appendChild(n);
    });
    return svg;
  }

  function room(opts) {
    opts = opts || {};
    getJSON('/api/rooms/sources').then(function (src) {
      dialog({
        title: RZ.t('catalog.new_room'),
        nameNeeded: RZ.t('newroom.name_needed'),
        items: function () {
          var out = [];
          if (src.mine.length) {
            out.push({head: RZ.t('newroom.mine')});
            src.mine.forEach(function (r) { out.push({type: 'mine', item: r, title: r.name, sub: r.seats}); });
          }
          out.push({head: RZ.t('newroom.other')});
          out.push({type: 'blank', title: RZ.t('newroom.blank'), sub: RZ.t('newroom.blank_hint')});
          return out;
        },
        name: function (p) { return p.type === 'mine' ? nextName(p.item.name) : ''; },
        preview: function (p, box) {
          if (p.type === 'blank') {
            box.appendChild(el('p', 'muted', RZ.t('newroom.blank_preview')));
            return;
          }
          box.appendChild(el('div', 'nm-subj', p.item.size + ' · ' + p.item.seats));
          box.appendChild(roomSVG(p.item));
          box.appendChild(el('small', 'muted', RZ.t('catalog.new_room_hint')));
        },
        create: function (name, p) {
          return RZ.postJSON('/api/rooms', {name: name, copy_of: p.type === 'mine' ? p.item.id : 0});
        },
        done: opts.done || goTo('/admin/rooms/', opts)
      });
    }).catch(failed);
  }

  // --- przyciski "Nowy…" przy wyborach w sekcjach ------------------------------
  //
  // <button data-new="mail|account|room" data-kind="events"
  //         data-select="f-mail-guest" (albo data-list="f-rooms" - lista "1,2,3")
  //         data-edit="/admin/mails/" data-back="/admin/events/26#mail-gosc">
  // Nowy element trafia do wyboru, sekcja się zapisuje i otwiera się edytor.
  function wire(root, secs) {
    var open = {mail: mail, account: account, room: room};
    root.addEventListener('click', function (e) {
      var b = e.target.closest('[data-new]');
      if (!b || !open[b.dataset.new]) return;
      e.preventDefault();
      var sec = b.closest('.sec');
      open[b.dataset.new]({
        kind: b.dataset.kind,
        formID: b.dataset.form,
        audience: b.dataset.audience,
        lockKind: true,
        done: function (id, name) {
          if (b.dataset.list) {
            var inp = document.getElementById(b.dataset.list);
            var ids = inp.value.split(',').filter(Boolean);
            ids.push(String(id));
            inp.value = ids.join(',');
          } else {
            var sel = document.getElementById(b.dataset.select);
            var o = el('option', '', name);
            o.value = String(id);
            sel.appendChild(o);
            sel.value = String(id);
          }
          secs.markDirty(sec);
          return secs.saveSection(sec).then(function () {
            goTo(b.dataset.edit, {back: b.dataset.back})(id);
          });
        }
      });
    });
  }

  global.RZNew = {mail: mail, account: account, room: room, wire: wire};
})(window);
