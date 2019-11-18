// this creates the selected variable
// we are going to store the selected objects in here
var selected = $([]), offset = {top:0, left:0}; 

draggies = {
      start: function(ev, ui) {
          if ($(this).hasClass("ui-selected")){
              selected = $(".ui-selected").each(function() {
                 var el = $(this);
                 el.data("offset", el.offset());
              });
          }
          else {
              selected = $([]);
              $("#room > div").removeClass("ui-selected");
          }
          offset = $(this).offset();
      },
      drag: function(ev, ui) {
          var dt = ui.position.top - offset.top, dl = ui.position.left - offset.left;
          // take all the elements that are selected expect $("this"), which is the element being dragged and loop through each.
          selected.not(this).each(function() {
               // create the variable for we don't need to keep calling $("this")
               // el = current element we are on
               // off = what position was this element at when it was selected, before drag
               var el = $(this), off = el.data("offset");
              el.css({top: off.top + dt, left: off.left + dl});
          });
      }
}

// https://stackoverflow.com/questions/1740700/how-to-get-hex-color-value-rather-than-rgb-value
function getHexColor( color ){
    //if color is already in hex, just return it...
    if( color.indexOf('#') != -1 ) return color;

    //leave only "R,G,B" :
    color = color
                .replace("rgba", "") //must go BEFORE rgb replace
                .replace("rgb", "")
                .replace("(", "")
                .replace(")", "");
    color = color.split(","); // get Array["R","G","B"]

    // 0) add leading #
    // 1) add leading zero, so we get 0XY or 0X
    // 2) append leading zero with parsed out int value of R/G/B
    //    converted to HEX string representation
    // 3) slice out 2 last chars (get last 2 chars) => 
    //    => we get XY from 0XY and 0X stays the same
    return  "#"
            + ( '0' + parseInt(color[0], 10).toString(16) ).slice(-2)
            + ( '0' + parseInt(color[1], 10).toString(16) ).slice(-2)
            + ( '0' + parseInt(color[2], 10).toString(16) ).slice(-2);
}

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

function GetOrderedAndCountPrice() {
  var selected = [];
  var price = 0;
  var prices = [];
  var rooms = [];

  $(".ui-selected").each(function(i, obj) {
    if ($(this).hasClass("chair")) {
      selected.push($(this).attr('name'));
      rooms.push($(this).attr('room'));
      if ($(this).attr('price') != 0 && $(this).attr('price') != "") {
        price += Number($(this).attr('price'));
        prices.push($(this).attr('price'));
      } else {
        price += Number(Price.defaultPrice);
        prices.push(Price.defaultPrice);
      }
    }
  });
  console.log({"sits": selected, "prices": prices, "rooms": rooms, "total-price": price, "default-currency": Price.defaultCurrency});
  return {"sits": selected, "prices": prices, "rooms": rooms, "total-price": price, "default-currency": Price.defaultCurrency}
}

function Order() {
  data = GetOrderedAndCountPrice();
  if (data["sits"] != "") {
    Post("/order", data);
  } else {
    $('#NoSitsSelected').modal()
  }
}

function ToggleDisable() {
  $(".ui-selected.chair, .ui-selecting.chair").each(function(i, obj) {
    var chair = $(this)
    if (chair.hasClass("disabled")) {
      chair.removeClass("disabled");
    } else {
      chair.addClass("disabled");
    }
  });
}

function SaveRoom() {
  var parent = $("#room")
  console.log("parent: " + parent.offset().top)
  $(".table, .chair, .object, .label").each(function(i, obj) {
    var child = $(this);
    if (child.attr('furniture') == "table") {
      current = {room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 13), y: Math.round(child.offset().top - parent.offset().top - 13)};
    } else if (child.attr('furniture') == "chair") {
      current = {room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), disabled: child.hasClass("disabled"), x: Math.round(child.offset().left - parent.offset().left - 5), y: Math.round(child.offset().top - parent.offset().top - 5)};
    } else if (child.attr('furniture') == "object") {
      current = {room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 3), y: Math.round(child.offset().top - parent.offset().top - 3), width: child.width(), height: child.height(), color: getHexColor(child.css("backgroundColor")), label: child.children().text()};
    } else if (child.attr('furniture') == "label") {
      current = {room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 3), y: Math.round(child.offset().top - parent.offset().top - 3), color: getHexColor(child.css("color")), label: child.children().text()};
    } else {
      console.log("this should not run, type: ", child.attr('furniture'))
      current = {room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 3), y: Math.round(child.offset().top - parent.offset().top - 3)};
    }


    $.ajax({
      method: "POST",
      url: "/api/furnit",
      data: JSON.stringify(current)
    })
    console.log(obj);
  });
  console.log("end");
}

function SetRoomSize() {
  $("#room").width($("#room-width").val());
  $("#room").height($("#room-height").val());

  $.ajax({
    method: "POST",
    url: "/api/room",
    data: JSON.stringify({room_id: Number($('#room-id').val()), width: $("#room").width(), height: $("#room").height()})
  })
}


function SpawnChairs() {
  $(".table.ui-selected, .table.ui-selecting").each(function(i, obj) {
    var table = $(this);
    var capacity = Number(table.attr("capacity"));
    var orientation = table.attr("orientation");
    for (i=1; i <= capacity; i++) {
      var parent = $( "#room" ).offset();
      if (orientation === "vertical") {
        if (i <= capacity/2) {
          var top = Math.round($(this).offset().top - parent.top - 5 + i*25 - 25);
          var left = Math.round($(this).offset().left - parent.left - 5 - 23);
        } else {
          var top = Math.round($(this).offset().top - parent.top - 5 + (i-capacity/2)*25 - 25);
          var left = Math.round($(this).offset().left - parent.left - 5 + 23);
        }
      } else {
        if (i <= capacity/2) {
          var left = Math.round($(this).offset().left - parent.left - 5 + i*25 - 25);
          var top = Math.round($(this).offset().top - parent.top - 5 - 23);
        } else {
          var left = Math.round($(this).offset().left - parent.left - 5 + (i-capacity/2)*25 - 25);
          var top = Math.round($(this).offset().top - parent.top - 5 + 23);
        }
      }
      table.after('<div class="chair ui-widget-content" furniture="chair" id="chair-'+Designer.chairNr+'" name="'+Designer.chairNr+'" style="top: '+top+'px; left: '+left+'px; position: absolute;">'+Designer.chairNr+'</div>')
      Designer.chairNr++;
    }
  });
}

function AddObject() {
  var width = $("#object-width").val();
  var height = $("#object-height").val();
  var color = $("#object-color").val();
  var label = $("#object-label").val();
  obj = '<div class="object ui-widget-content" furniture="object" id="object-'+Designer.objectNr+'" name="'+Designer.objectNr+'" style="width: '+width+'px; height: '+height+'px;position: absolute; background: '+color+';"><p>'+label+'</p></div>';
  $("#room").append(obj);
  Designer.objectNr++;
}

function AddLabel() {
  var color = $("#label-color").val();
  var label = $("#label-title").val();
  console.log(label);
  labelObj = '<div class="label ui-widget-content" furniture="label" id="label-'+Designer.labelNr+'" name="'+Designer.labelNr+'" style="position: absolute; color: '+color+';"><p>'+label+'</p></div>';
  $("#room").append(labelObj);
  Designer.labelNr++;
}

function DeleteFurnitures() {
  $(".ui-selected, .ui-selecting").each(function(i, obj) {
    $(this).remove();
    $.ajax({
      method: "POST",
      url: "/api/furdel",
      data: JSON.stringify({room_id: Number($("#room-id").val()),name: Number($(this).attr('name')), type: $(this).attr('furniture')})
    })
    console.log(JSON.stringify({room_id: Number($("#room-id").val()), id: $(this).attr('name'), type: $(this).attr('furniture')}))

  });

}

function Rotate() {
  $(".ui-selected, .ui-selecting").each(function(i, obj) {
    var table = $(this);
    curOrientation = table.attr("orientation");
    curCapacity = table.attr("capacity")
    table.removeClass(curOrientation+'-'+curCapacity);
    if (curOrientation === "vertical") {
      table.attr("orientation", "horizontal");
      table.addClass("horizontal-"+curCapacity);
    } else {
      table.attr("orientation", "vertical");
      table.addClass("vertical-"+curCapacity);
    }
  });   
}

function Renumber(type) {
  $.ajax({
    method: "POST",
    url: "/api/renumber",
    data: JSON.stringify({room_id: Number($("#room-id").attr('value')), type: type})
  })
  console.log(JSON.stringify({room_id: Number($("#room-id").attr('value')), type: type}))
}

function TriggerSelect() {
  return function(e) {
    if (e.ctrlKey == false) {
          // if command key is pressed don't deselect existing elements
          $( "#room > div" ).removeClass("ui-selected");
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
  }
}

function MakeSelectable() {
  var selected;
  var price;

  $( ".room-view" ).bind("mousedown", function(event, ui) {
      //var result = $( "#select-result" ).empty();
      event.ctrlKey = true;
    });
  $( ".room-view" ).selectable({
    selected: function (e, ui) {
      selected = [];
      price = 0;
      $(".ui-selected").each(function(i, obj) {
        if ($(this).hasClass("chair") && !$(this).hasClass("disabled") && !$(this).hasClass("marked") && !$(this).hasClass("ordered")) {
          selected.push($(this).attr('name'));
          if ($(this).attr('price') != 0 && $(this).attr('price') != "") {
            price += Number($(this).attr('price'));
          } else {
            price += Number(Price.defaultPrice);
          }
        } else {
          $(this).removeClass("ui-selecting");
          $(this).removeClass("ui-selected");
        }
      });
      $("#selected-chairs").html(selected.join(", "));
      $("#total-price").html(price);
    },
    unselected: function (e, ui) {
      selected = [];
      price = 0;
      $(".ui-selected").each(function(i, obj) {
        if ($(this).hasClass("chair")) {
          selected.push($(this).attr('name'));
          if ($(this).attr('price') != 0 && $(this).attr('price') != "") {
            price += Number($(this).attr('price'));
          } else {
            price += Number(Price.defaultPrice);
          }
        }
      });
      $("#selected-chairs").html(String(selected));
      $("#total-price").html(price);
    }
  });
}


$(function() {
  if (typeof Designer === 'undefined') {
    Designer = {};
  }
    
  var table_nr=Designer.tableNr;
  var chair_nr=Designer.chairNr;
  var orientation="vertical";

  //room-view
  MakeSelectable();
  // set first room as active tab
  $('.nav-tabs a:first').tab('show');

  // trigger on tab change
  $('a[data-toggle="tab"]').on('shown.bs.tab', function (e) {
    // set current-roomID hidden div content
    $("#current-roomID").html($(e.target).attr("name")) // unused
    // make chairs selectable on active tab
    MakeSelectable();
  });

  $( "#room > div" ).draggable({
    draggies
  });

  $("#room").on("DOMNodeInserted", ".ui-widget-content", function() { $(this).draggable({
    draggies
  })});

    $( "#room" ).selectable();
  
  // manually trigger the "select" of clicked elements
  $( "#room > div" ).click(TriggerSelect());
  $("#room").on("DOMNodeInserted", ".ui-widget-content", TriggerSelect());

  // dynamically add draggable table
  $("#add-table").click(function () {
    //var table = document.createElement('div');
    var parent = $( "#room" ).offset();
    var capacity = $("#chairs-ammount").val();
    table = '<div id="table-'+table_nr+'" name='+table_nr+' furniture="table" orientation="'+orientation+'" class="ui-widget-content table '+orientation+'-'+capacity+'" capacity='+capacity+'><p>Table nr '+table_nr+'</p></div>';
    $("#room").append(table);
    table_nr++;
  });

});
