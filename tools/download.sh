#!/bin/sh

# sass compiler
wget https://github.com/sass/dart-sass/releases/download/1.22.12/dart-sass-1.22.12-linux-x64.tar.gz

# air hot reloader
curl -fLo air \
    https://raw.githubusercontent.com/cosmtrek/air/master/bin/linux/air
chmod +x air

# jquery
wget https://code.jquery.com/jquery-3.4.1.min.js -O ../js/jquery.min.js
wget https://code.jquery.com/jquery-3.4.1.min.map -O ../js/jquery.min.map

# jquery-ui (tylko designer sali: draggable, resizable, selectable)
wget https://jqueryui.com/resources/download/jquery-ui-1.12.1.zip

#quill - text editor
wget https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js -O ../js/quill2.min.js
wget https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js.map -O ../js/quill2.min.js.map
wget https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.snow.css -O ../css/quill2.snow.css
echo "In quill2.min.js change sourceMappingURL to quill2.min.js.map"
