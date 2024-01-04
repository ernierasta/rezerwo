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


function EventEdit() {
  var eventID = $('#events-select :selected').val();
  console.log(eventID);
  Post("/admin/event", {"event-id": eventID});
}

function RoomEdit() {
  $('#room-event').modal();
  //$("#rooms-select-form").submit();
}

function FinalRoomEdit() {
  //console.log($("#events-select :selected").val());
  var eventID = $('#room-event-select :selected').val(); // get event selected for room
  var roomID = $('#rooms-select :selected').val(); // get room id
  Post("/admin/designer", {"room-id": roomID, "event-id": eventID})
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
