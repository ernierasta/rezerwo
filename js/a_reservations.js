var ChairNr = 0;
var Room = 1;
var Name = 2;
var Surname = 3;
var OrderStatus = 4;
var Email = 5;
var Notes = 6;
var Phone = 7;
var Price = 8;
var Currency = 9;
var Ordered = 10;
var Payed = 11;

$(function() {

  // Column filering
  // Setup - add a text input to each footer cell
  $('#reservations thead tr').clone(true).appendTo( '#reservations thead' );
  $('#reservations thead tr:eq(1) th').each( function (i) {
    var title = $(this).text();
    var width = '';
    if (i == OrderStatus || i == ChairNr || i == Name || i == Surname) {
      width = 'style="width: 110px;"';
    }
    $(this).html( '<input type="text" '+width+' placeholder="Search '+title+'" />' );
    $( 'input', this ).on( 'keyup change', function () {
      // a bit overhead, but need to have actual value when typing
      var colnr = table.colReorder.transpose(i);
      if ( table.column(colnr).search() !== this.value ) {
         table.column(colnr).search( this.value ).draw();
      }
    });
  });

  var table = $('#reservations').DataTable({
    orderCellsTop: true,
    fixedHeader: true,
    colReorder: true,
    //responsive: true,
    columnDefs: [{
      orderable: true,
      //className: 'select-checkbox',
      targets:   0
    },
      {
        targets: 4,
        render: function ( data, type, row ) {
          var color = 'black';
          if (data == 'ordered') {
            color = '#fd7e14';
          }
          if (data == 'payed') {
            color = '#28a745';
          }
          return '<span style="color:' + color + '">' + data + '</span>';
        }
     }
    ],
    'lengthMenu': [ [10, 50, 100, -1], [10, 50, 100, "All"] ],
    'pageLength': -1,
    select: {
      style: 'multi', //'os'
    },
    // TODO: figure how to make it usefull
    //rowGroup: {
    //    dataSrc: 'group'
    //},
    dom: 'ilfB<"total-price-lbl">rtp',
    buttons: [ 
      {
        extend: 'collection',
        text: 'Export',
        buttons: ['copy', 'excel', 'csv' ,'pdf', 'print']
      },
      'colvis',
      {
        extend: 'selected',
        text: 'Toggle "ordered/payed"',
        action: function ( e, dt, button, config ) {
          var stscol = table.colReorder.transpose(OrderStatus);
          var furnNumberCol = table.colReorder.transpose(ChairNr);
          var roomNameCol = table.colReorder.transpose(Room);
          indexes = dt.rows({selected: true}).indexes();
          for (i=0; i < indexes.length;i++){
            row = dt.row(indexes[i]).data()
            if (row[stscol] === "ordered") {
              row[stscol] = "payed";
              dt.row(indexes[i]).data(row);
              $.ajax({
                method: "POST",
                url: "/api/resstatus",
                data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol] , status: "payed"})
              });
            } else if (row[stscol] === "payed") {
              row[stscol] = "ordered";
              dt.row(indexes[i]).data(row);
              $.ajax({
                method: "POST",
                url: "/api/resstatus",
                data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol] , status: "ordered"})
              });
            } 
          }
          dt.rows({selected: true}).deselect();
          //$('#total-price').html(0);
          //$('#total-sits').html(0);
        }
      },
      {
        extend: "selected",
        text: "Delete",
        action: function ( e, dt, button, config ) {
          var furnNumberCol = table.colReorder.transpose(ChairNr);
          var roomNameCol = table.colReorder.transpose(Room);
          var indexes = dt.rows({selected: true}).indexes();
          bootbox.confirm({
            message: "Really delete selected orders? It is UNREVERSABLE!",
              buttons: {
                cancel: {
                    label: '<i class="fa fa-times"></i> Cancel'
                },
                confirm: {
                    label: '<i class="fa fa-check"></i> Delete'
                }
              },
            callback: function(result) {
              if (result) {
                for (i=0; i < indexes.length;i++){
                  var row = dt.row(indexes[i]).data();
                  $.ajax({
                    method: "DELETE",
                    url: "/api/resdelete",
                    data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol]})
                  });
                }
                dt.rows({selected: true}).remove().draw();
              }
            }
          });
        },
      },
      {
        text: "Cancel filters",
        action: function ( e, dt, button, config ) {
          $('#reservations thead tr:eq(1) th input').each( function (i) {
            table.column(i).search("");
            $(this).val("");
          });
          table.draw();
        }
      },
      {
        // select only visible, not all
        text: "Select All",
        action: function ( e, dt, button, config ) {
          dt.rows( { page: 'current' } ).select();
        }
      },
      'selectNone',
    ]
  });

  // position info panel
  $("div#reservations_length").css("display", "inline");
  $("div#reservations_length").css("float", "left");
  $("div#reservations_length").css("margin-right", "10px");

  // set content of total-price-lbl, it is created in "dom" table param
  $("div.total-price-lbl").css("display", "inline");
  $("div.total-price-lbl").css("margin-left", "5px");
  $("div.total-price-lbl").html('Total: <span id="total-price"></span><span>, Sits: <span id="total-sits"></span></div>');

  // show total sits and price on select/deselect
  table.on( 'select', function ( e, dt, items ) {
    var rows = dt.rows({selected: true}).data();
    var price = 0;
    var pricecol = table.colReorder.transpose(Price);
    for (i=0; i < rows.length;i++){
      price += Number(rows[i][pricecol]);
    };
    $('#total-price').html(price);
    $('#total-sits').html(rows.length);
  });

  table.on( 'deselect', function ( e, dt, items ) {
    var rows = dt.rows({selected: true}).data();
    var price = 0;
    var pricecol = table.colReorder.transpose(Price);
    for (i=0; i < rows.length;i++){
      price += Number(rows[i][pricecol]);
    };
    $('#total-price').html(price);
    $('#total-sits').html(rows.length);
  });

});
