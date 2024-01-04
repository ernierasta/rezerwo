function Save() {
  var finald = {};
  finald.name = $('#ba-name').val();
  finald.account = $('#ba-account').val();
  finald.recipient = $('#ba-recipientname').val();
  finald.message = $('#ba-message').val();
  finald.fieldname = $('#ba-amount-field-name').val();
  finald.varsymbol = $('#ba-varsymbol').val();
  finald.currency = $('#ba-currency').val();


  $.ajax({
    type: "POST",
    url: "/api/baed",
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
