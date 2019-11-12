function CancelOrder() {
  var roomsVal = $("#rooms").val();
  var sitsVal = $("#sits").val();
  var eventID = Number($("#event-id").val());

  $.ajax({
    method: "POST",
    url: "/api/ordercancel",
    data: JSON.stringify({"rooms": roomsVal, "sits": sitsVal, "event-id": eventID})
  });
}

function GoBackToRoom() {
  CancelOrder();
  window.history.back();
}
