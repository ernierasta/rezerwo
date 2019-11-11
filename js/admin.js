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

$(function() {
  var QuillOrderNoteEditor = new Quill('#order-note-editor', {
    theme: 'snow'
  });
  var QuillHowtoEditor = new Quill('#howto-editor', {
    theme: 'snow'
  });
  // copy editor content to hidden text area x2
  $("#event-form").on("submit",function(){
    $("#html-order-note").val($("#order-note-editor").html());
    $("#html-howto").val($("#howto-editor").html());
})

});


