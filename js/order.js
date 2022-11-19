// get countdown running
// Set the date we're counting down to
//var countDownDate = new Date("Jan 5, 2024 15:37:25").getTime();
var now = new Date().getTime();
var countDownDate = now + (5 * 60 * 1000);

// Update the count down every 1 second
var x = setInterval(function() {

  // Get today's date and time
  var now = new Date().getTime();
    
  // Find the distance between now and the count down date
  var distance = countDownDate - now;
    
  // Time calculations for days, hours, minutes and seconds
  var days = Math.floor(distance / (1000 * 60 * 60 * 24));
  var hours = Math.floor((distance % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  var minutes = Math.floor((distance % (1000 * 60 * 60)) / (1000 * 60));
  var seconds = Math.floor((distance % (1000 * 60)) / 1000);
    
  // Output the result in an element with id="demo"
  document.getElementById("countdown").innerHTML = days + "d " + hours + "h "
  + minutes + "m " + seconds + "s ";
    
  // If the count down is over, write some text 
  if (distance < 0) {
    clearInterval(x);
    document.getElementById("demo").innerHTML = "Sesja wygasła. :-(";
  }
}, 1000);

// countdown end

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

function GoBackToRoom(event) {
  $.when(
    CancelOrder()
  ).then(function() {
    window.location.replace("/res/"+$("#user-url").val());
  });
}
