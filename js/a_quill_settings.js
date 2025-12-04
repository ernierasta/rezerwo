
  // 1. Tworzymy własny attributor klas (Quill 1.3.7 nie ma już wbudowanego)
  const Parchment = Quill.import('parchment');

  class CustomClass extends Parchment.Attributor.Class {
    add(node, value) {
      // Usuwamy wszystkie nasze klasy, żeby nie nakładać kilku naraz
      ['free-text','marked-text','ordered-text','payed-text','disabled-text']
        .forEach(c => node.classList.remove(c));
      if (value) node.classList.add(value);
    }
    value(node) {
      for (let cls of ['free-text','marked-text','ordered-text','payed-text','disabled-text']) {
        if (node.classList.contains(cls)) return cls;
      }
      return '';
    }
  }

  const customClass = new CustomClass('customClass', '', {
    scope: Parchment.Scope.INLINE,
    whitelist: ['free-text','marked-text','ordered-text','payed-text','disabled-text']
  });

  Quill.register(customClass, true);

