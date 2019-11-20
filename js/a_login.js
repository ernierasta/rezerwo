function LoginForm(form) {
  var formdata = $(form).serializeArray()
  formdata.push({name: 'role', value: 'admin'});
  var finaldata = {};
  for (element of formdata) {
    var name = element.name;
    finaldata[name] = element.value;
  }
  //console.log(finaldata);

  $.ajax({
    type: "POST",
    url: "/api/login",
    data: JSON.stringify(finaldata),
    success: function(data) {
      console.log("ok!");
      //window.location.replace("/admin");
      window.location.href = "/admin";
    },
    statusCode: {
      401: function() {
        alert("failed!");
      },
    },
    dataType: "json",
  });
  return false;
}

