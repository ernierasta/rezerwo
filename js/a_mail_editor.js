
function GoBack() {
  window.history.back();
}

var QuillMailTextEditor = new Quill('#text-editor', {
    theme: 'snow'
  }
);

function Save() {
  var finald = {};
  finald.id = $('#notification-id').val();
  finald.userid = $('#user-id').val();
  finald.name = $('#name').val();
  finald.subject = $('#subject').val();
  finald.text = QuillMailTextEditor.root.innerHTML;
  finald.sharable = $('#sharable').is(":checked");
  finald.relatedto = $('#related-to-select').val(); // events or forms
  finald.attachedfiles = $('#attachedfiles').val();
  finald.embeddedimgs = $('#embeddedimgs').val();
  //finald.message = $('#ba-message').val();
  //finald.fieldname = $('#ba-amount-field-name').val();
  //finald.varsymbol = $('#ba-varsymbol').val();
  //finald.currency = $('#ba-currency').val();

  console.log(finald.sharable);

  $.ajax({
    type: "POST",
    url: "/api/maed",
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
      GoBack();
      //window.location.replace("/admin");
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
