
var options = {
  i18n: {
    locale: 'pl-PL',
    location: '/js/',
     //location: 'http://languagefile.url/directory/'
    //extension: '.ext'
    //override: {
    //    'en-US': {...}
    //}
  },
  templates: templates,
  fields: customFields,
  typeUserAttrs: userAttrs
};


var fbEditor = document.getElementById('fb-editor');
var formBuilder = $(fbEditor).formBuilder(options);

document.getElementById('save').addEventListener('click', function() {
    SendFormDef(formBuilder.actions.getData('json', true));
});


const toolbarOptions = [
  [{ 'header': [1, 2, 3, false] }],
  ['bold', 'italic', 'underline'],        // toggled buttons
  ['link', 'image', 'video'],

  [{ 'list': 'ordered'}, { 'list': 'bullet' }],
  [{ 'indent': '-1'}, { 'indent': '+1' }],          // outdent/indent

  [{ 'color': ['#28a745','#ffc107', '#fd7e14', '#dc3545', '#403734'] }],          // dropdown with defaults from theme
  [{ 'align': [] }],

  ['clean']                                         // remove formatting button
];

var QuillHowToEditor = new Quill('#howto-editor', {
  modules: {
    toolbar: toolbarOptions
  },
  theme: 'snow'
});

//var QuillHowToEditor = new Quill('#howto-editor', {
//    theme: 'snow'
//  });

var QuillThankYouEditor = new Quill('#thankyou-editor', {
    theme: 'snow'
  }
);

var QuillInfoPanelEditor = new Quill('#infopanel-editor', {
    theme: 'snow'
  }
);


function NewBankAccount(){
  
}


function SendFormDef(data) {

  var finald = {};
  dataj = JSON.parse(data);
  finald.name = $('#form-name').val();
  finald.url = $('#form-url').val();
  finald.howto = QuillHowToEditor.root.innerHTML;
  finald.banner = $('#form-banner').val();
  finald.thankyou = QuillThankYouEditor.root.innerHTML;
  finald.infopanel = QuillInfoPanelEditor.root.innerHTML;
  finald.moneyfield = $('#money-field').val();
  finald.bankaccount = $('#qrpay-select').val();
  finald.thankyoumail = $('#mail-thankyou-select').val();

  // remove all trailing <br> from end of label json field
  // it some kind of bug in formBuilder
  for (var i = 0; i < dataj.length; i++) {
    dataj[i].label = dataj[i].label.replace(/<br>+$/, "");
  }
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
