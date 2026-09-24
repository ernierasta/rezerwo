// n_newmail.js - okno "Nowy mail" w katalogach.
// Nowy mail to zawsze kopia: pusty, z gotowego szablonu albo z własnego maila.
// Źródła: GET /api/mails/sources, podgląd: POST /api/mails/preview,
// utworzenie: POST /api/mails {name, template | copy_of | related_to}.

(function (global) {
  'use strict';

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  function nextName(name) {
    // "Bal 2025" -> "Bal 2026"
    return String(name).replace(/(\d{4})/, function (y) { return String(Number(y) + 1); });
  }

  function open() {
    fetch('/api/mails/sources')
      .then(function (r) { return r.json().then(function (d) { if (!r.ok) throw new Error(d.msg || r.status); return d; }); })
      .then(show)
      .catch(function (e) { RZ.toast(RZ.t('common.save_error') + ': ' + e.message); });
  }

  function show(src) {
    var kind = 'events';
    var picked = null;   // {type: 'blank'|'template'|'mine', item}
    var previews = {};

    var back = el('div', 'dlg-back');
    var dlg = el('div', 'dlg dlg-wide nm');
    dlg.setAttribute('role', 'dialog');
    dlg.setAttribute('aria-modal', 'true');
    back.appendChild(dlg);

    dlg.appendChild(el('h3', '', RZ.t('newmail.title')));

    var seg = el('div', 'seg');
    [['events', RZ.t('catalog.for_events')], ['forms', RZ.t('catalog.for_forms')]].forEach(function (k) {
      var b = el('button', k[0] === kind ? 'on' : '', k[1]);
      b.type = 'button';
      b.dataset.kind = k[0];
      seg.appendChild(b);
    });
    dlg.appendChild(seg);

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

    function item(type, it, title, sub) {
      var b = el('button', 'nm-item');
      b.type = 'button';
      b.appendChild(el('b', '', title));
      if (sub) b.appendChild(el('small', '', sub));
      b.addEventListener('click', function () { pick(b, {type: type, item: it}); });
      return b;
    }

    function renderList() {
      list.textContent = '';
      var first = item('blank', null, RZ.t('newmail.blank'), RZ.t('newmail.blank_hint'));
      list.appendChild(first);

      var tpls = src.templates.filter(function (t) { return t.related_to === kind; });
      if (tpls.length) {
        list.appendChild(el('h5', '', RZ.t('newmail.templates')));
        tpls.forEach(function (t) { list.appendChild(item('template', t, t.name, t.description)); });
      }

      list.appendChild(el('h5', '', RZ.t('newmail.mine')));
      var mine = src.mine.filter(function (m) {
        return kind === 'forms' ? m.related_to === 'forms' : m.related_to !== 'forms';
      });
      if (!mine.length) list.appendChild(el('p', 'muted', RZ.t('newmail.none_mine')));
      mine.forEach(function (m) { list.appendChild(item('mine', m, m.name, m.subject)); });

      pick(first, {type: 'blank'});
    }

    function pick(btn, p) {
      picked = p;
      list.querySelectorAll('.nm-item').forEach(function (b) { b.classList.toggle('on', b === btn); });
      if (p.type === 'blank') nameI.value = '';
      else if (p.type === 'template') nameI.value = p.item.name;
      else nameI.value = nextName(p.item.name);
      showPreview(p);
    }

    function showPreview(p) {
      prev.textContent = '';
      if (p.type === 'blank') {
        prev.appendChild(el('p', 'muted', RZ.t('newmail.blank_preview')));
        return;
      }
      var key = p.type + ':' + (p.item.key || p.item.id);
      var head = el('div', 'nm-subj');
      var frame = document.createElement('iframe');
      frame.setAttribute('sandbox', '');
      frame.title = RZ.t('mail.preview_title');
      prev.appendChild(head);
      prev.appendChild(frame);

      var cached = previews[key];
      var req = cached ? Promise.resolve(cached) : RZ.postJSON('/api/mails/preview', {
        subject: p.item.subject || '',
        text: p.item.text || '',
        related_to: p.item.related_to
      }).then(function (r) { previews[key] = r; return r; });

      req.then(function (r) {
        if (picked !== p) return;
        head.textContent = r.subject;
        var files = r.files && r.files.length ? '<p style="color:#6e7885;font-size:13px">📎 ' + r.files.join(', ').replace(/</g, '&lt;') + '</p>' : '';
        frame.srcdoc = '<html><head><base href="' + location.origin + '/"><style>body{font:14px/1.5 sans-serif;color:#222;margin:12px}p{margin:0}img{max-width:100%}</style></head><body>' + r.html + files + '</body></html>';
      }).catch(function (e) {
        if (picked !== p) return;
        prev.textContent = '';
        prev.appendChild(el('p', 'miss', e.message));
      });
    }

    seg.addEventListener('click', function (e) {
      var b = e.target.closest('button');
      if (!b || b.dataset.kind === kind) return;
      kind = b.dataset.kind;
      seg.querySelectorAll('button').forEach(function (x) { x.classList.toggle('on', x === b); });
      renderList();
    });

    function done() {
      back.remove();
      document.removeEventListener('keydown', esc);
    }
    function esc(e) { if (e.key === 'Escape') done(); }
    cancel.addEventListener('click', done);
    back.addEventListener('click', function (e) { if (e.target === back) done(); });
    document.addEventListener('keydown', esc);

    create.addEventListener('click', function () {
      var name = nameI.value.trim();
      if (!name) {
        msg.textContent = RZ.t('newmail.name_needed');
        msg.classList.add('bad');
        nameI.focus();
        return;
      }
      var data = {name: name, related_to: kind};
      if (picked.type === 'template') data.template = picked.item.key;
      if (picked.type === 'mine') data.copy_of = picked.item.id;

      create.disabled = true;
      msg.classList.remove('bad');
      msg.textContent = RZ.t('common.loading');
      RZ.postJSON('/api/mails', data)
        .then(function (r) { location.href = '/admin/mails/' + r.id; })
        .catch(function (e) {
          msg.textContent = RZ.t('common.save_error') + ': ' + e.message;
          msg.classList.add('bad');
          create.disabled = false;
        });
    });

    renderList();
    document.body.appendChild(back);
    list.querySelector('.nm-item').focus();
  }

  global.RZNewMail = {open: open};
})(window);
