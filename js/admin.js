function EventEdit() {

  console.log($("#events-select :selected").val());
  $("#events-select-form").submit()
}

function GoBack() {
  window.history.back();
}

$(function() {
  var quill = new Quill('#editor', {
    theme: 'snow'
  });
});


