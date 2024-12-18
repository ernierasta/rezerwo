// this is called after formRender initialization, so
// formRender is available here.

function SendFormData() {
 
  // great trick to check form validity, it returns bool,
  // so you can use it to check manually (but then build in validator is not working)
  // TODO: customize it, howto:
  // https://developer.mozilla.org/en-US/docs/Web/API/HTMLObjectElement/setCustomValidity
  valid = document.getElementById('form-rendered').reportValidity();

  if (!valid) {
    return;
  }

  // send data and uri ()
  var finald = {};
  finald.uri = window.location.pathname;
  finald.uniqid = $('#uniqid').text();
  finald.data = formRender.userData;

  console.log("uniqid: " + finald.uniqid);

  $.ajax({
    type: "POST",
    url: "/api/formans",
    data: JSON.stringify(finald),
    success: function(resp) {
      console.log("FormID:" + resp.id);
      
      // make button green
      $("#save").addClass("btn-success");
      // set timer to make button blue again 
      window.setTimeout(function(){
        $("#save").removeClass("btn-success");
      },2000);
      //window.location.replace(finald.uri + "/done");
      //window.location.assign(finald.uri + "/done");
      //window.location.href = "/admin";
     
      // redirect to given url and send additional
      // post data
      
      $.redirect(finald.uri + "/done",
        {
          formID:   resp.formid,
          name:     resp.name,
          surname:  resp.surname,
        }
      );
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
