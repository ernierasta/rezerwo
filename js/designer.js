// current is currently dropped item (table, chair, etc.)
var current=null;
var selected = $([]);
var offset = {top:0, left:0};

function GenerateTable(nr, chairs) {

}

function GenerateChairs(capacity, table_nr, chair_nr) {
  //generate chairs
  var chairs = '';
  for (i=1; i <= capacity; i++) {
    chairs += '<div class="ui-widget-content chair draggable" id='+table_nr+'-chair-'+chair_nr+'>'+chair_nr+'</div>';
    chair_nr++;
  }
  $('#table-'+table_nr).append(chairs);
  MakeDraggable(); //make chairs draggable
}

function MakeSelectableDraggable(parent) {
  $( "#table-storage > div" ).draggable({
    start: function(event, ui) {
      if ($(this).hasClass("ui-selected")){
        selected = $(".ui-selected").each(function() {
          var el = $(this);
          el.data("offset", el.offset());
        });
        //current = {id: $(this).attr('id'), x: pos.left - parent.left, y: pos.top - parent.top };
        //$("#pos").html( 'id:' + current.id + ', x:' + current.x + ', y:' + current.y );
      } else {
        selected=$([]);
        // why?
        $("#table-storage > div").removeClass("ui-selected");
      }
      offset = $(this).offset();
    },
    drag: function(event, ui) {
      var dt = ui.position.top - offset.top, dl = ui.position.left - offset.left;
      selected.not(this).each(function() {
        var el = $(this), off = el.data("offset");
        el.css({top: off.top + dt, left: off.left + dl});
      });
    }
  });

  $("#table-storage").selectable();

  // manually trigger the "select" of clicked elements
  $( "#table-storage > div" ).click( function(e){
    if (e.ctrlKey == false) {
        // if command key is pressed don't deselect existing elements
        $( "#table-storage > div" ).removeClass("ui-selected");
        $(this).addClass("ui-selecting");
    }
    else {
        if ($(this).hasClass("ui-selected")) {
            // remove selected class from element if already selected
            $(this).removeClass("ui-selected");
        }
        else {
            // add selecting class if not
            $(this).addClass("ui-selecting");
        }
    }
    
    //$( "#table-storage" ).data("table-storage")._mouseStop(null);
  });

}

function MakeDraggable(parent) {
  $( ".draggable" ).draggable({
   drag: function(){
     pos = $(this).offset();
     current = {id: $(this).attr('id'), type: $(this).attr('furniture'), orientation: $(this).attr('orientation'), x: pos.left - parent.left, y: pos.top - parent.top };
     $("#pos").html( 'id:' + current.id + ', x:' + current.x + ', y:' + current.y );
     return current;
   }
 });
}

function MakeSelectable() {
  $(function() {
    $( ".selectable" ).bind("mousedown", function(event, ui) {
      var result = $( "#select-result" ).empty();
      event.ctrlKey = true;
    });
    $( ".selectable" ).selectable();
  });
}

$(function() {
  var pos=null;
  var parent=null;
  var table_nr=1;
  var chair_nr=1;
  var orientation="vertical";

  // set room size
  $("#set-room-size").click(function () {
    $("#droppable").width($("#room-width").val());
    $("#droppable").height($("#room-height").val());
  })

  // dynamically add draggable table
  $("#add-table").click(function () {
    //var table = document.createElement('div');
    var parent = $( "#droppable" ).offset();
    var capacity = $("#chairs-ammount").val();
    //table.innerHTML = '<p>'+nr+'</p>';
    //chairs = GenerateChairs(capacity, table_nr, chair_nr); 
    table = '<div id="table-'+table_nr+'" furniture="table" orientation="'+orientation+'" class="ui-widget-content table draggable '+orientation+'-'+capacity+'"><p>Table nr '+table_nr+'</p><div class="table-controls"><button onclick="GenerateChairs('+capacity+', '+table_nr+', '+chair_nr+')" type="button" class="hidden">+</button></div></div>';
    $("#table-storage").append(table);
    //$("table"+table_nr).css({top: parent.top, left: parent.left});
    // set draggable function for every new table
    //MakeSelectableDraggable(parent);
    MakeDraggable(parent);
    //MakeSelectable();
    table_nr++;
  });

  // change text on dropped, send data when mouse is released inside room
  $( "#droppable" ).droppable({
    drop: function( event, ui ) {
      $(this).addClass( "ui-state-highlight" )
        .find( "p" ).html( "Dropped!" );
      $.ajax({
        method: "POST",
        url: "/mv",
        data: JSON.stringify(current)
      })
      console.log(current);
    }
  });

  // show buttons on mouseenter
  $(document).on('mouseenter', '.table', function () {
    $(this).find(":button").show();
    //console.log($(this).width());
    //$(this).css('width', $(this).width()+100+"px");
    $(this).addClass("mouseover");
    //console.log($(this).width());
  }).on('mouseleave', '.table', function () {
    $(this).find(":button").hide();
    //console.log($(this).width());
    //$(this).css('width', $(this).width()-100+"px");
    $(this).removeClass("mouseover");
    //console.log($(this).width());
  });
});
