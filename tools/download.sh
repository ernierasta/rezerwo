#!/bin/sh

# sass compiler
wget https://github.com/sass/dart-sass/releases/download/1.22.12/dart-sass-1.22.12-linux-x64.tar.gz

# air hot reloader
curl -fLo air \
    https://raw.githubusercontent.com/cosmtrek/air/master/bin/linux/air
chmod +x air

# popper
wget https://unpkg.com/popper.js -O ../js/popper.js
echo "Now remove last row from file, or it will warn about map file(or get one from somewhere ;-)"

# jquery
wget https://code.jquery.com/jquery-3.4.1.min.js -O ../js/jquery.min.js
wget https://code.jquery.com/jquery-3.4.1.min.map -O ../js/jquery.min.map

# jquery-ui
wget https://jqueryui.com/resources/download/jquery-ui-1.12.1.zip

# jquery-redirect
wget https://raw.githubusercontent.com/mgalante/jquery.redirect/master/jquery.redirect.js -O ../js/jquery.redirect.js

#quill - text editor
wget https://github.com/quilljs/quill/releases/download/v1.3.6/quill.tar.gz

# datatable.js - download by hand from: https://datatables.net
# look inside downloaded zip which modules we actually have
# actually we are using all extensions

# bootbox - easy modals
wget https://github.com/makeusabrew/bootbox/releases/download/v5.3.4/bootbox.min.js -O ../js/bootbox.min.js
wget https://github.com/makeusabrew/bootbox/releases/download/v5.3.4/bootbox.locales.min.js -O ../js/bootbox.locales.min.js

# qr code generator
wget https://raw.githubusercontent.com/davidshimjs/qrcodejs/master/qrcode.min.js -O ../js/qrcode.min.js

