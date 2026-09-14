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
      RefreshAdminLists({"rooms-select": resp.room_id});
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
	RoomCopyDefaultName();
	$('#room-copy').modal();
}

// RoomCopyDefaultName prefills room copy name based on selected room
function RoomCopyDefaultName() {
	var $opt = $('#room-copy-select :selected');
	$('#room-copy-name').val($opt.val() !== "0" ? $opt.text().trim() + " - kopia" : "");
}

function FinalRoomCopy() {
	roomID = $('#room-copy-select :selected').val(); // get room id
	var data = {"room_id": Number(roomID), "name": $('#room-copy-name').val()};
	console.log(data);
	SendToAPI(data);
	$('#room-copy').modal('hide');
}

// RefreshAdminLists reloads all selects on admin page from server, without
// page reload. Current selection is kept, select => value in "select" overrides it.
var ADMIN_SELECTS = ["room-event-select", "room-copy-select", "rooms-select",
  "events-select", "events-raports-select", "forms-select", "forms-raports-select",
  "ba-select", "notification-select"];

function RefreshAdminLists(select) {
  select = select || {};
  $.get("/admin", function(html) {
    var doc = new DOMParser().parseFromString(html, "text/html");
    ADMIN_SELECTS.forEach(function(id) {
      var fresh = doc.getElementById(id);
      var $cur = $("#" + id);
      if (!fresh || !$cur.length) {
        return;
      }
      var val = select[id] !== undefined ? String(select[id]) : $cur.val();
      $cur.html(fresh.innerHTML);
      if (val && $cur.find('option[value="' + val + '"]').length) {
        $cur.val(val);
      }
    });
  });
}

// event copy dialog
var eventCopyInfo = null;

function EventCopy() {
  var eventID = $('#events-select :selected').val();
  if (!eventID || eventID === "0") {
    alert("Wybierz imprezę do skopiowania.");
    return;
  }
  $.ajax({
    type: "GET",
    url: "/api/eventcopyinfo",
    data: {"event_id": eventID},
    dataType: "json",
    success: function(info) {
      eventCopyInfo = info;
      $('#event-copy-name').val(info.name + " - kopia");
      // same dates as original, moved to current year (both by the same number of years,
      // so reservation start stays before event date)
      var years = new Date().getFullYear() - Number(info.date.slice(0, 4));
      $('#event-copy-date').val(EventCopyShiftYear(info.date, years));
      $('#event-copy-from-date').val(EventCopyShiftYear(info.from_date, years));
      $('#event-copy-only-mine').prop("checked", true);
      $('#event-copy-rooms').empty();
      info.room_ids.forEach(function(id) {
        EventCopyAddRoomRow(id);
      });

      var hasNotifs = !!(info.thankyou || info.admin);
      $('#event-copy-notifs').prop("hidden", !hasNotifs);
      $('#event-copy-no-notifs').prop("hidden", hasNotifs);
      $('#event-copy-notifs-check').prop("checked", false);
      $('#event-copy-notifs-names').prop("hidden", true);
      $('#event-copy-thankyou-group').prop("hidden", !info.thankyou);
      $('#event-copy-thankyou-name').val(info.thankyou ? info.thankyou.name + " - kopia" : "");
      // the same notification may be used for both, then it is copied only once
      var sameNotif = info.thankyou && info.admin && info.thankyou.id === info.admin.id;
      $('#event-copy-admin-group').prop("hidden", !info.admin || sameNotif);
      $('#event-copy-admin-name').val(info.admin ? info.admin.name + " - kopia" : "");

      $('#event-copy').modal();
    },
    error: function(xhr) {
      var msg = xhr.responseJSON ? xhr.responseJSON.msg : xhr.statusText;
      alert(msg);
      console.log(msg);
    },
  });
}

function EventCopyAddRoomRow(roomID) {
  var $row = $('#event-copy-room-tmpl').clone().removeAttr("id").prop("hidden", false);
  var $sel = $row.find(".event-copy-room-select");
  eventCopyInfo.all_rooms.forEach(function(room) {
    $("<option>").val(room.id).text(room.name + " (" + room.id + ")")
      .attr("data-mine", room.mine ? "1" : "0").appendTo($sel);
  });
  if (roomID) {
    $sel.val(String(roomID));
  } else {
    // first visible room
    var first = eventCopyInfo.all_rooms.find(function(room) {
      return room.mine || !$('#event-copy-only-mine').is(":checked");
    });
    if (first) {
      $sel.val(String(first.id));
    }
  }
  $('#event-copy-rooms').append($row);
  EventCopyFilterRooms();
}

// hides not user's rooms when "only mine" is checked, selected room stays visible
function EventCopyFilterRooms() {
  var onlyMine = $('#event-copy-only-mine').is(":checked");
  $('#event-copy-rooms .event-copy-room-select').each(function() {
    var $sel = $(this);
    var val = $sel.val();
    $sel.find("option").each(function() {
      var hide = onlyMine && $(this).attr("data-mine") !== "1" && $(this).val() !== val;
      $(this).prop("hidden", hide).prop("disabled", hide);
    });
  });
}

// EventCopyShiftYear moves "YYYY-MM-DD" date by given number of years, 29.02 becomes 28.02 if needed
function EventCopyShiftYear(d, years) {
  var y = Number(d.slice(0, 4)) + years;
  var md = d.slice(4);
  if (md === "-02-29" && new Date(Date.UTC(y, 1, 29)).getUTCMonth() !== 1) {
    md = "-02-28";
  }
  return String(y).padStart(4, "0") + md;
}

// EventCopyRoomName shows room copy name input when "make copy" is checked
// and prefills it from selected room
function EventCopyRoomName($row) {
  var copy = $row.find(".event-copy-room-copy").is(":checked");
  var $name = $row.find(".event-copy-room-name");
  $name.prop("hidden", !copy);
  if (copy) {
    var roomID = Number($row.find(".event-copy-room-select").val());
    var room = eventCopyInfo.all_rooms.find(function(r) { return r.id === roomID; });
    $name.val(room ? room.name + " - kopia" : "");
  }
}

$(function() {
  $('#room-copy-select').on("change", RoomCopyDefaultName);
  $('#event-copy-only-mine').on("change", EventCopyFilterRooms);  $('#event-copy-rooms').on("change", ".event-copy-room-select", function() {
    EventCopyFilterRooms();
    EventCopyRoomName($(this).closest(".event-copy-room"));
  });
  $('#event-copy-rooms').on("change", ".event-copy-room-copy", function() {
    EventCopyRoomName($(this).closest(".event-copy-room"));
  });
  $('#event-copy-rooms').on("click", ".event-copy-room-del", function() {
    $(this).closest(".event-copy-room").remove();
  });
  $('#event-copy-add-room').on("click", function() {
    EventCopyAddRoomRow(null);
  });
  $('#event-copy-notifs-check').on("change", function() {
    $('#event-copy-notifs-names').prop("hidden", !$(this).is(":checked"));
  });
});

function FinalEventCopy() {
  if (!eventCopyInfo) {
    return;
  }
  var copyNotifs = $('#event-copy-notifs-check').is(":checked");
  var rooms = [];
  $('#event-copy-rooms .event-copy-room').each(function() {
    rooms.push({
      "room_id": Number($(this).find(".event-copy-room-select").val()),
      "copy": $(this).find(".event-copy-room-copy").is(":checked"),
      "name": $(this).find(".event-copy-room-name").val(),
    });
  });
  var data = {
    "event_id": eventCopyInfo.event_id,
    "name": $('#event-copy-name').val(),
    "date": $('#event-copy-date').val(),
    "from_date": $('#event-copy-from-date').val(),
    "rooms": rooms,
    "copy_notifications": copyNotifs,
    "thankyou_name": copyNotifs && eventCopyInfo.thankyou ? $('#event-copy-thankyou-name').val() : "",
    "admin_name": copyNotifs && eventCopyInfo.admin ? $('#event-copy-admin-name').val() : "",
  };
  // same notification for both - backend copies it once, name is not needed
  if (copyNotifs && eventCopyInfo.thankyou && eventCopyInfo.admin && eventCopyInfo.thankyou.id === eventCopyInfo.admin.id) {
    data.admin_name = data.thankyou_name;
  }
  if (copyNotifs && ((eventCopyInfo.thankyou && !data.thankyou_name.trim()) || (eventCopyInfo.admin && !data.admin_name.trim()))) {
    alert("Podaj nazwy kopiowanych notyfikacji.");
    return;
  }

  $.ajax({
    type: "POST",
    url: "/api/eventcopy",
    data: JSON.stringify(data),
    contentType: "application/json",
    dataType: "json",
    success: function(resp) {
      console.log(resp.msg);
      $('#event-copy').modal('hide');
      $("#copy-event").addClass("btn-success");
      window.setTimeout(function(){
        $("#copy-event").removeClass("btn-success");
      },2000);
      RefreshAdminLists({"events-select": resp.event_id});
    },
    error: function(xhr) {
      var msg = xhr.responseJSON ? xhr.responseJSON.msg : xhr.statusText;
      alert(msg);
      console.log(msg);
      // event may be created even with errors
      RefreshAdminLists();
    },
  });
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
