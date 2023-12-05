
var options = {
  i18n: {
    locale: 'pl-PL',
    location: '/js/'
    //location: 'http://languagefile.url/directory/'
    //extension: '.ext'
    //override: {
    //    'en-US': {...}
    //}
  }
};

var fbEditor = document.getElementById('fb-editor');
var formBuilder = $(fbEditor).formBuilder(options);

document.getElementById('save').addEventListener('click', function() {
    SendFormDef(formBuilder.actions.getData('json', true));
});

var QuillHowToEditor = new Quill('#howto-editor', {
    theme: 'snow'
  });

var QuillThankYouEditor = new Quill('#thankyou-editor', {
    theme: 'snow'
  });


function SendFormDef(data) {
  var finald = {};
  dataj = JSON.parse(data);
  finald.name = $('#form-name').val();
  finald.url = $('#form-url').val();
  finald.howto = QuillHowToEditor.root.innerHTML;
  finald.banner = $("#form-banner").val();
  finald.thankyou = QuillThankYouEditor.root.innerHTML;
  finald.content = dataj;
  console.log(finald);
  

  $.ajax({
    type: "POST",
    url: "/api/formed",
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
