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

function SendToAPI(jsonData) {
  $.ajax({
    type: "POST",
    url: "/api/roomcopy",
    data: JSON.stringify(jsonData),
    success: function(resp) {
      console.log(resp.msg);
      // we can check msg to determine insert/update

      // make button green
      $("#select-room-copy-button").addClass("btn-success");
      // set timer to make button blue again
      window.setTimeout(function(){
        $("#select-room-copy-button").removeClass("btn-success");
      },2000);
      //window.location.replace("/admin");
      //window.location.href = "/admin";
    },
    statusCode: {
      418: function(xhr) {
      $("#select-room-copy-button").addClass("btn-danger");
      // set timer to make button blue again
      window.setTimeout(function(){
        $("#select-room-copy-button").removeClass("btn-danger");
      },2000);

        alert(xhr.responseJSON.msg);
        console.log(xhr.responseJSON.msg);
      },
    },
    dataType: "json",
  });

}

function SendDelRoom(jsonData) {
  $.ajax({
    type: "POST",
    url: "/api/roomdel",
    data: JSON.stringify(jsonData),
    success: function(resp) {
      console.log(resp.msg);
      // we can check msg to determine insert/update

      // make button green
      $("#del-room-button").addClass("btn-success");
      // set timer to make button blue again
      window.setTimeout(function(){
        $("#del-room-button").removeClass("btn-success");
      },2000);
      //window.location.replace("/admin");
      //window.location.href = "/admin";
    },
    statusCode: {
      418: function(xhr) {
      $("#del-room-button").addClass("btn-danger");
      // set timer to make button blue again
      window.setTimeout(function(){
        $("#del-room-button").removeClass("btn-danger");
      },2000);

        alert(xhr.responseJSON.msg);
        console.log(xhr.responseJSON.msg);
      },
    },
    dataType: "json",
  });

}



function EventEdit() {
  var eventID = $('#events-select :selected').val();
  Post("/admin/event", {"event-id": eventID});
}

function NewEvent() {
  name = $('#new-event-name').val();
  Post("/admin/event", {"name": name });
}


function RoomEdit() {
  $('#room-event').modal();
  //$("#rooms-select-form").submit();
}

function RoomDel() {
  var roomID = $('#rooms-select :selected').val(); // get room id
  SendDelRoom({"room_id": Number(roomID)})
}

function FinalRoomEdit() {
  //console.log($("#events-select :selected").val());
  var eventID = $('#room-event-select :selected').val(); // get event selected for room
  var roomID = $('#rooms-select :selected').val(); // get room id
  Post("/admin/designer", {"room-id": roomID, "event-id": eventID})
}

function RoomCopy() {
	$('#room-copy').modal();
}

function FinalRoomCopy() {
	roomID = $('#room-copy-select :selected').val(); // get room id
	console.log({"room_id": roomID});
	SendToAPI({"room_id": Number(roomID)});
	$('#room-copy').modal('hide');
}

function ShowRaports() {
  var eventID = $('#events-raports-select :selected').val();
  Post("/admin/reservations", {"event-id": eventID});
}

function NewForm() {
  name = $('#new-form-name').val();
  url = $('#new-form-url').val();
  Post("/admin/formeditor", {"name": name, "url": url });
}

function EditForm() {
  var formID = $('#forms-select :selected').val();
  Post("/admin/formeditor", {"form-id": formID});
}

function ShowFormRaports() {
  var formtmplID = $('#forms-raports-select :selected').val();
  Post("/admin/formraport", {"formtmpl-id": formtmplID});
}

function NewBankAccount() {
  name = $('#new-ba-name').val();
  Post("/admin/bankacceditor", {"name": name });
}

function EditBankAccount() {
  var baID = $('#ba-select :selected').val();
  Post("/admin/bankacceditor", {"ba-id": baID});
}
function DeleteBankAccount() {
  var baID = $('#ba-select :selected').val();
  Post("/admin/bankacceditor", {"ba-id": baID, "action": "delete"});
}

function NewNotification() {
  name = $('#new-notification-name').val();
  Post("/admin/maileditor", {"name": name });
}

function EditNotification() {
  var baID = $('#notification-select :selected').val();
  Post("/admin/maileditor", {"mail-id": baID});
}
function DeleteNotification() {
  var baID = $('#notification-select :selected').val();
  Post("/admin/maileditor", {"mail-id": baID, "action": "delete"});
}
