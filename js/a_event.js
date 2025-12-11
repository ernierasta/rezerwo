function GoBack() {
  window.history.back();
}

/**
 * sends a request to the specified url from a form. this will change the window location.
 * @param {string} path the path to send the post request to
 * @param {object} params the paramiters to add to the url
 * @param {string} [method=post] the method to use on the form
 */
function Post(path, params, method='post') {

  // The rest of this code assumes you are not using a library.
  // It can be made less wordy if you use one.
  const form = document.createElement('form');
  form.method = method;
  form.action = path;

  for (const key in params) {
    if (params.hasOwnProperty(key)) {
      const hiddenField = document.createElement('input');
      hiddenField.type = 'hidden';
      hiddenField.name = key;
      hiddenField.value = params[key];

      form.appendChild(hiddenField);
    }
  }

  document.body.appendChild(form);
  form.submit();
}


// UNUSED
function EventDetailSubmit() {
  $("#html-order-note").val($("#order-note-editor").html());
  $("#html-howto").val($("#howto-editor").html());
  return false;
}

function EditNotification() {
  var maID = $('#mail-thankyou-select :selected').val();
  Post("/admin/maileditor", {"mail-id": maID});
}

function EditAdminNotification() {
  var maID = $('#mail-admin-select :selected').val();
  Post("/admin/maileditor", {"mail-id": maID});
}

function RoomAssign() {
  var cur = $('#rooms').val();
  const arr = cur.split(',');
  var sel =  $('#room-select :selected').val()
  if (!arr.includes(sel) && sel != 0) {
    if (cur == '') {
      cur = sel;
    } else {
      cur = cur + ',' + sel;
    }
    $('#rooms').val(cur);
  }
}

function ClearRooms() {
  $('#rooms').val('');
}

function EditBankAccount() {
  var baID = $('#ba-select :selected').val();
  Post("/admin/bankacceditor", {"ba-id": baID});
}
function DeleteBankAccount() {
  var baID = $('#ba-select :selected').val();
  Post("/admin/bankacceditor", {"ba-id": baID, "action": "delete"});
}


// not used, simpler solution with redirect to notification form
$("#mail-thankyou-select").change(function () {
    mailsel = $(this).children(':selected');
    console.log(mailsel);
    
    //$('#mail-subject').val(mailsel);    
});


//$(function() {
//  var QuillOrderNoteEditor = new Quill('#order-note-editor', {
//    theme: 'snow'
//  });
//  var QuillHowtoEditor = new Quill('#howto-editor', {
//    theme: 'snow'
//  });
  // copy editor content to hidden text area x2
  //$("#event-form").on("submit",function(){
  //  $("#html-order-note").val($("#order-note-editor").html());
  //  $("#html-howto").val($("#howto-editor").html());
// })

//});

// QUILL Toolbar def

const toolbarOptions = [
  [{ 'header': [1, 2, 3, false] }],
  ['bold', 'italic', 'underline'],
  ['link'],
  [{ 'list': 'ordered'}, { 'list': 'bullet' }],
  [{ 'indent': '-1'}, { 'indent': '+1' }],
  [{ 'color': ['#28a745','#ffc107', '#fd7e14', '#dc3545', '#403734', '#000000'] }],
  [{ 'align': [] }],
  ['clean']
];

// Now create QUILLS

var QuillNoSitsEditor = new Quill('#no-sits-editor', {
  modules: {
    toolbar: toolbarOptions,
    clipboard: {
      matchVisual: false
    }
  },
  theme: 'snow'
});

var QuillOrderNoteEditor = new Quill('#order-note-editor', {
  modules: {
    toolbar: toolbarOptions
  },

  theme: 'snow'
});

// we will use ordinary editor, Quill strips color tags.

//const toolbarOptions = [
//  [{ 'header': [1, 2, 3, false] }],
//  ['bold', 'italic', 'underline'],        // toggled buttons
//  ['link'],
//
//  [{ 'list': 'ordered'}, { 'list': 'bullet' }],
//  [{ 'indent': '-1'}, { 'indent': '+1' }],          // outdent/indent
//
//  [{ 'color': ['#28a745','#ffc107', '#fd7e14', '#dc3545', '#403734'] }],          // dropdown with defaults from theme
//  [{ 'align': [] }],
//
//  ['clean']                                         // remove formatting button
//];

//var QuillHowtoEditor = new Quill('#howto-editor', {
//  modules: {
//    toolbar: toolbarOptions
//  },
//  theme: 'snow'
//});


var QuillHowtoEditor = new Quill('#howto-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});

var QuillOrderHowtoEditor = new Quill('#order-howto-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});

var QuillMainDescEditor = new Quill('#maindesc-editor', {
  modules: {
    toolbar: toolbarOptions,
    clipboard: {
      matchVisual: false
    }
  },
  theme: 'snow'
});


var QuillRoom1DescEditor = new Quill('#room1desc-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});

var QuillRoom2DescEditor = new Quill('#room2desc-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});

var QuillRoom3DescEditor = new Quill('#room3desc-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});

var QuillRoom4DescEditor = new Quill('#room4desc-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});



function Save() {
  var finald = {};
         
  finald.id = $('#id').val();
  finald.name = $('#name').val();
  finald.language = $('#language').val();
  finald.date = $('#date').val();
  finald.from_date = $('#from-date').val();
  finald.to_date = $('#to-date').val();
  finald.price = $('#default-price').val();
  finald.currency = $('#default-currency').val();
  finald.no_sits_selected_title = $('#no-sits-selected-title').val();
  finald.no_sits_selected_text = QuillNoSitsEditor.root.innerHTML;
  finald.how_to = QuillHowtoEditor.root.innerHTML;
  //finald.how_to = $('#howto-text').val();
  finald.order_howto = QuillOrderHowtoEditor.root.innerHTML;
  finald.order_notes_desc = $('#order_notes_desc').val();
  finald.ordered_note_title = $('#ordered-note-title').val();
  finald.ordered_note_text = QuillOrderNoteEditor.root.innerHTML;
  //finald.mail_subject = $('#mail_subject').val();
  //finald.mail_text = $('#mail_text').val();
  //finald.mail_attachments = $('#mail_attachments').val();
  //finald.mail_embeded_imgs = $('#mail_embeded_imgs').val();
  //finald.admin_mail_subject = $('#admin_mail_subject').val();
  //finald.admin_mail_text = $('#admin_mail_text').val();
  finald.bank_account_id = $('#ba-select').val();
  finald.user_id = $('#user-id').val(); 
  finald.thankyou_notifications_id_fk = $('#mail-thankyou-select').val();
  finald.admin_notifications_id_fk = $('#mail-admin-select').val();
  finald.sharable = $('#sharable').is(":checked");
  finald.rooms = $('#rooms').val();
  finald.maindesc = QuillMainDescEditor.root.innerHTML;
  finald.mainbanner = $('#mainbanner').val();
  finald.room1desc = QuillRoom1DescEditor.root.innerHTML;
  finald.room1banner = $('#room1banner').val();
  finald.room2desc = QuillRoom2DescEditor.root.innerHTML;
  finald.room2banner = $('#room2banner').val();
  finald.room3desc = QuillRoom3DescEditor.root.innerHTML;
  finald.room3banner = $('#room3banner').val();
  finald.room4desc = QuillRoom4DescEditor.root.innerHTML;
  finald.room4banner = $('#room4banner').val();

  //finald.relatedto = $('#related-to-select').val(); // events or forms
  //console.log(finald.sharable);


  $.ajax({
    type: "POST",
    url: "/api/eved",
    data: JSON.stringify(finald),
    success: function(resp) {
      console.log(resp.msg);
      // we can check msg to determine insert/update
      
      // make button green
      $("#save").addClass("btn-success");
      // set timer to make button blue again 
      window.setTimeout(function(){
        $("#save").removeClass("btn-success");
      },2000);
      window.location.replace("/admin");
      //window.location.href = "/admin";
    },
    statusCode: {
      418: function(xhr) {
      $("#save").addClass("btn-danger");
      // set timer to make button blue again
      window.setTimeout(function(){
        $("#save").removeClass("btn-danger");
      },2000);

        alert(xhr.responseJSON.msg);
        console.log(xhr.responseJSON.msg);
      },
    },
    dataType: "json",
  });

}

