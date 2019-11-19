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
  $("#events-select-form").submit();
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
