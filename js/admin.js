function EventEdit() {

  console.log($("#events-select :selected").val());
  $("#events-select-form").submit()
}

function RoomEdit() {

  console.log($("#rooms-select :selected").val());
  $("#rooms-select-form").submit()
}


function GoBack() {
  window.history.back();
}


//function ValidateOrder() {
//  if ($("email")== "") {
//
//  }
//}


function EventDetailSubmit() {
  $("#html-order-note").val($("#order-note-editor").html());
  $("#html-howto").val($("#howto-editor").html());
  return false;
}


(function() {
  'use strict';
  window.addEventListener('load', function() {
    // Fetch all the forms we want to apply custom Bootstrap validation styles to
    var forms = document.getElementsByClassName('needs-validation');
    // Loop over them and prevent submission
    var validation = Array.prototype.filter.call(forms, function(form) {
      form.addEventListener('submit', function(event) {
        if (form.checkValidity() === false) {
          event.preventDefault();
          event.stopPropagation();
        }
        form.classList.add('was-validated');
      }, false);
    });
  }, false);
})();

$(function() {
  var QuillOrderNoteEditor = new Quill('#order-note-editor', {
    theme: 'snow'
  });
  var QuillHowtoEditor = new Quill('#howto-editor', {
    theme: 'snow'
  });
  // copy editor content to hidden text area x2
  //$("#event-form").on("submit",function(){
  //  $("#html-order-note").val($("#order-note-editor").html());
  //  $("#html-howto").val($("#howto-editor").html());
// })

});


