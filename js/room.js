// other selected elements moved together with the dragged one
var DragGroup = $([]);

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
  //console.log({"event-id": CurrentEvent.id, "sits": selected, "prices": prices, "rooms": rooms, "total-price": price, "default-currency": Price.defaultCurrency});
  return {"event-id": CurrentEvent.id, "sits": selected, "prices": prices, "rooms": rooms, "total-price": price, "default-currency": Price.defaultCurrency}
}

function Order() {
  data = GetOrderedAndCountPrice();
  if (data["sits"] != "") {
    Post("/order", data);
  } else {
    $('#NoSitsSelected').modal();
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

// SaveRoom saves all furnitures, returns promise resolved when all are saved
function SaveRoom() {
  var parent = $("#room")
  var requests = [];
  $(".table, .chair, .object, .label").each(function(i, obj) {
    var child = $(this);
    if (child.attr('furniture') == "table") {
      current = {event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 13), y: Math.round(child.offset().top - parent.offset().top - 13)};
    } else if (child.attr('furniture') == "chair") {
      current = {event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), disabled: child.hasClass("disabled"), x: Math.round(child.offset().left - parent.offset().left - 5), y: Math.round(child.offset().top - parent.offset().top - 5)};
    } else if (child.attr('furniture') == "object") {
      current = {event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 3), y: Math.round(child.offset().top - parent.offset().top - 3), width: child.width(), height: child.height(), color: getHexColor(child.css("backgroundColor")), label: child.children().text()};
    } else if (child.attr('furniture') == "label") {
      current = {event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 3), y: Math.round(child.offset().top - parent.offset().top - 3), color: getHexColor(child.css("color")), label: child.children().text()};
    } else {
      console.log("this should not run, type: ", child.attr('furniture'))
      current = {event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), name: Number(child.attr('name')), type: child.attr('furniture'), orientation: child.attr('orientation'),capacity: Number(child.attr('capacity')), x: Math.round(child.offset().left - parent.offset().left - 3), y: Math.round(child.offset().top - parent.offset().top - 3)};
    }


    requests.push($.ajax({
      method: "POST",
      url: "/api/furnit",
      data: JSON.stringify(current)
    }));
  });

  var btn = $("#save");
  btn.prop("disabled", true);
  return $.when.apply($, requests)
    .always(function() { btn.prop("disabled", false); })
    .done(function() { btn.removeClass("btn-danger").addClass("btn-success"); })
    .fail(function() {
      btn.removeClass("btn-success").addClass("btn-danger");
      console.error("SaveRoom: saving furnitures failed");
    });
}

function SetRoomSize() {
  var width = Number($("#room-width").val());
  var height = Number($("#room-height").val());
  if (!(width > 0 && height > 0)) {
    return;
  }
  // same as in template: inline width/height including border
  $("#room").css({width: width+"px", height: height+"px"});

  $.ajax({
    method: "POST",
    url: "/api/room",
    data: JSON.stringify({room_id: Number($('#room-id').val()), width: width, height: height})
  })
}

// ReloadDesigner loads designer page again (it is POST only page)
function ReloadDesigner() {
  Post("/admin/designer", {"room-id": $("#room-id").val(), "event-id": $("#event-id").val()});
}

// NewChair returns chair element, left/top are css position
function NewChair(nr, left, top) {
  return $('<div class="chair ui-widget-content" furniture="chair" id="chair-'+nr+'" name="'+nr+'" style="top: '+top+'px; left: '+left+'px; position: absolute;"><p>'+nr+'</p></div>');
}


// chair geometry, must match $table and $table-gap in scss/variables.scss
var CHAIR_SIZE = 20;
var CHAIR_GAP = 5;
var CHAIR_PITCH = CHAIR_SIZE + CHAIR_GAP; // distance between chairs spawned along table, default grid size
var CHAIR_MARGIN = 2; // .chair margin in scss/room.scss

function IsRoundTable(orientation) {
  return orientation === "round" || orientation === "oval-vertical" || orientation === "oval-horizontal";
}

// ChairPositionsRect returns chair box positions (relative to room inner area)
// for rectangular table, half of chairs on each long side
function ChairPositionsRect(x, y, orientation, capacity) {
  var pos = [];
  var perSide = Math.ceil(capacity/2);
  for (var i = 0; i < capacity; i++) {
    var along = (i % perSide) * CHAIR_PITCH;
    var firstSide = i < perSide;
    if (orientation === "vertical") {
      pos.push({left: firstSide ? x - CHAIR_PITCH : x + CHAIR_PITCH, top: y + along});
    } else {
      pos.push({left: x + along, top: firstSide ? y - CHAIR_PITCH : y + CHAIR_PITCH});
    }
  }
  return pos;
}

// ChairPositionsEllipse distributes chairs evenly (by arc length) around
// round/oval table, starting on top and going clockwise. Chair center lies on
// outward normal of table edge, far enough to keep CHAIR_GAP between square
// chair and the table also on diagonals.
function ChairPositionsEllipse(x, y, w, h, orientation, capacity) {
  var cx = x + w/2, cy = y + h/2;
  var a = w/2, b = h/2;
  var steps = 720;
  var pts = [], lens = [0];
  for (var s = 0; s <= steps; s++) {
    var t = -Math.PI/2 + 2*Math.PI*s/steps;
    var ex = a*Math.cos(t), ey = b*Math.sin(t);
    var nx = ex/(a*a), ny = ey/(b*b);
    var nl = Math.hypot(nx, ny);
    nx /= nl;
    ny /= nl;
    var dist = CHAIR_SIZE/2 * (Math.abs(nx) + Math.abs(ny)) + CHAIR_GAP;
    pts.push({x: cx + ex + nx*dist, y: cy + ey + ny*dist});
    if (s > 0) {
      lens.push(lens[s-1] + Math.hypot(pts[s].x - pts[s-1].x, pts[s].y - pts[s-1].y));
    }
  }
  var perimeter = lens[steps];
  // for ovals shift by half step, so chairs are symmetric along long sides
  var shift = orientation === "round" ? 0 : 0.5;
  var pos = [];
  var j = 0;
  for (var i = 0; i < capacity; i++) {
    var target = (i + shift) * perimeter / capacity;
    while (j < steps && lens[j+1] < target) { j++; }
    pos.push({left: Math.round(pts[j].x - CHAIR_SIZE/2), top: Math.round(pts[j].y - CHAIR_SIZE/2)});
  }
  return pos;
}

function SpawnChairs() {
  var room = $("#room");
  var inner = {left: room.offset().left + room[0].clientLeft, top: room.offset().top + room[0].clientTop};
  $(".table.ui-selected, .table.ui-selecting").each(function(i, obj) {
    var table = $(this);
    var capacity = Number(table.attr("capacity"));
    var orientation = table.attr("orientation");
    // table box position relative to room inner area
    var x = Math.round(table.offset().left - inner.left);
    var y = Math.round(table.offset().top - inner.top);
    var positions;
    if (IsRoundTable(orientation)) {
      positions = ChairPositionsEllipse(x, y, table.outerWidth(), table.outerHeight(), orientation, capacity);
    } else {
      positions = ChairPositionsRect(x, y, orientation, capacity);
    }
    positions.forEach(function(p) {
      var chair = NewChair(Designer.chairNr, p.left - CHAIR_MARGIN, p.top - CHAIR_MARGIN);
      table.after(chair);
      InitFurniture(chair);
      Designer.chairNr++;
    });
  });
}

// Grid

var GRID_STORAGE_KEY = "rezerwo-designer-grid";

function GridSettings() {
  var size = Number($("#grid-size").val());
  return {
    size: size > 0 ? size : CHAIR_PITCH,
    show: $("#grid-show").is(":checked"),
    snap: $("#grid-snap").is(":checked")
  };
}

function LoadGridSettings() {
  var s = {size: CHAIR_PITCH, show: true, snap: true};
  try {
    var stored = JSON.parse(window.localStorage.getItem(GRID_STORAGE_KEY));
    if (stored) { s = $.extend(s, stored); }
  } catch (e) {}
  $("#grid-size").val(s.size);
  $("#grid-show").prop("checked", s.show);
  $("#grid-snap").prop("checked", s.snap);
}

function ApplyGrid() {
  var g = GridSettings();
  try {
    window.localStorage.setItem(GRID_STORAGE_KEY, JSON.stringify(g));
  } catch (e) {}
  var room = $("#room");
  if (g.show) {
    var line = "rgba(0, 0, 0, 0.15) 1px, transparent 1px";
    room.css({
      "background-image": "linear-gradient(to right, "+line+"), linear-gradient(to bottom, "+line+")",
      "background-size": g.size+"px "+g.size+"px",
      "background-position": "0 0"
    });
  } else {
    room.css({"background-image": "", "background-size": "", "background-position": ""});
  }
}

// GridDragStart remembers difference between css position and element box
// relative to room, works for absolute and relative (new, floated) elements
// (rounded, offsets can be fractional with page zoom).
// Dragging selected element moves whole selection, dragging not selected
// element clears selection.
function GridDragStart(ev, ui) {
  var room = $("#room");
  var el = $(this);
  var diff;
  if (el.css("position") === "absolute") {
    // box position is css position + margin
    diff = {left: parseFloat(el.css("margin-left")) || 0, top: parseFloat(el.css("margin-top")) || 0};
  } else {
    diff = {
      left: Math.round(el.offset().left - room.offset().left - room[0].clientLeft - ui.position.left),
      top: Math.round(el.offset().top - room.offset().top - room[0].clientTop - ui.position.top)
    };
  }
  el.data("grid-diff", diff);
  // css position, ui.position can be fractional with page zoom
  el.data("drag-origin", {left: parseFloat(el.css("left")) || 0, top: parseFloat(el.css("top")) || 0});

  if (el.is(".ui-selected, .ui-selecting")) {
    DragGroup = $("#room .ui-selected, #room .ui-selecting").not(this).each(function() {
      var o = $(this);
      if (o.css("position") === "static") {
        o.css("position", "relative");
      }
      o.data("drag-start", {left: parseFloat(o.css("left")) || 0, top: parseFloat(o.css("top")) || 0});
    });
  } else {
    $("#room .ui-selected, #room .ui-selecting").removeClass("ui-selected ui-selecting");
    DragGroup = $([]);
  }
}

// GridDrag snaps element box (without margin) to grid and moves the rest of selection
function GridDrag(ev, ui) {
  var g = GridSettings();
  var el = $(this);
  var diff = el.data("grid-diff");
  if (g.snap && diff) {
    ui.position.left = Math.round((ui.position.left + diff.left) / g.size) * g.size - diff.left;
    ui.position.top = Math.round((ui.position.top + diff.top) / g.size) * g.size - diff.top;
  }
  var origin = el.data("drag-origin");
  if (!origin) {
    return;
  }
  var dl = Math.round(ui.position.left - origin.left), dt = Math.round(ui.position.top - origin.top);
  DragGroup.each(function() {
    var start = $(this).data("drag-start");
    $(this).css({left: start.left + dl, top: start.top + dt});
  });
}

// InitFurniture makes element in designer draggable and selectable by click
function InitFurniture(el) {
  el.draggable({start: GridDragStart, drag: GridDrag});
  el.click(TriggerSelect());
}

function AddObject() {
  var width = $("#object-width").val();
  var height = $("#object-height").val();
  var color = $("#object-color").val();
  var label = $("#object-label").val();
  obj = $('<div class="object ui-widget-content" furniture="object" id="object-'+Designer.objectNr+'" name="'+Designer.objectNr+'" style="width: '+width+'px; height: '+height+'px;position: absolute; background: '+color+';"><p>'+label+'</p></div>');
  $("#room").append(obj);
  InitFurniture(obj);
  Designer.objectNr++;
}

function AddLabel() {
  var color = $("#label-color").val();
  var label = $("#label-title").val();
  console.log(label);
  labelObj = $('<div class="label ui-widget-content" furniture="label" id="label-'+Designer.labelNr+'" name="'+Designer.labelNr+'" style="position: absolute; color: '+color+';"><p>'+label+'</p></div>');
  $("#room").append(labelObj);
  InitFurniture(labelObj);
  Designer.labelNr++;
}

function DeleteFurnitures() {
  $(".ui-selected, .ui-selecting").each(function(i, obj) {
    $(this).remove();
    $.ajax({
      method: "POST",
      url: "/api/furdel",
      data: JSON.stringify({event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), name: Number($(this).attr('name')), type: $(this).attr('furniture')})
    })
    console.log(JSON.stringify({event_id: Number($("#event-id").val()), room_id: Number($("#room-id").val()), id: $(this).attr('name'), type: $(this).attr('furniture')}))

  });

}

var ROTATIONS = {
  "vertical": "horizontal",
  "horizontal": "vertical",
  "oval-vertical": "oval-horizontal",
  "oval-horizontal": "oval-vertical",
  "round": "round"
};

function Rotate() {
  $(".table.ui-selected, .table.ui-selecting").each(function(i, obj) {
    var table = $(this);
    var curOrientation = table.attr("orientation");
    var curCapacity = table.attr("capacity");
    var newOrientation = ROTATIONS[curOrientation] || "vertical";
    table.removeClass(curOrientation+'-'+curCapacity);
    table.attr("orientation", newOrientation);
    table.addClass(newOrientation+'-'+curCapacity);
  });
}

// Renumber saves room, renumbers furnitures on server and reloads designer,
// so numbers on page (and next free numbers) are not stale
function Renumber(type) {
  SaveRoom().done(function() {
    $.ajax({
      method: "POST",
      url: "/api/renumber",
      data: JSON.stringify({room_id: Number($("#room-id").val()), type: type})
    }).always(ReloadDesigner);
  });
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
      $(".selected-chairs").each(function(i, obj) {
        $(this).html(selected.join(", "));
      });
      $(".total-price").each(function(i, obj) {
        $(this).html(price);
      });
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
      $(".selected-chairs").each(function(i, obj) {
        $(this).html(String(selected));
      });
      $(".total-price").each(function(i, obj) {
        $(this).html(price);
      });
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

  // enable deleting with "Delete" or "Backspace" key on keyboard
  $('html').keyup(function(e){
    // do not delete furnitures while editing inputs
    if ($(e.target).is("input, textarea, select")) {
      return;
    }
    if(e.keyCode == 46 || e.keyCode == 8) {
        DeleteFurnitures();
    }
  });

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

  // draggable (with grid snapping) and click-selectable furnitures,
  // newly added ones are initialized in functions adding them
  InitFurniture($( "#room > div" ));

  $( "#room" ).selectable();

  // grid
  if ($("#room").length) {
    LoadGridSettings();
    ApplyGrid();
    $("#grid-size, #grid-show, #grid-snap").on("change input", ApplyGrid);
  }

  // add single chair to top left corner of room
  $("#add-chair").click(function () {
    var chair = NewChair(Designer.chairNr, -CHAIR_MARGIN, -CHAIR_MARGIN);
    $("#room").append(chair);
    InitFurniture(chair);
    Designer.chairNr++;
  });

  // dynamically add draggable table
  $("#add-table").click(function () {
    var capacity = $("#chairs-ammount").val();
    var orientation = $("#table-shape").val() || "vertical";
    var table = $('<div id="table-'+table_nr+'" name='+table_nr+' furniture="table" orientation="'+orientation+'" class="ui-widget-content table '+orientation+'-'+capacity+'" capacity='+capacity+'><p>'+table_nr+'</p></div>');
    $("#room").append(table);
    InitFurniture(table);
    table_nr++;
  });

});
