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

// SetSums sets sums of selected rows
// Need as much elements with id='footX'(where X=number)
// as there are columns
function SetSums(dt) {
    var rows = dt.rows({selected: true}).data();
    if (rows.length > 0) {
      var sums = Array(rows[0].length).fill(0);
      for (i=0; i < rows.length;i++){ // iterate over rows
        for (n=0; n < rows[0].length;n++){ // iterate over row cols
          sums[n] += Number(rows[i][n]);
        };
      };
      for (i=0; i < sums.length;i++){
        if (!isNaN(sums[i])) {
          $('#tfoot'+i).html('<b>'+sums[i]+'</b>');
        }
      };
    } else {
      colnum = $('#form-raport thead th').length;
      for (i=0;i < colnum;i++) {
        $('#tfoot'+i).empty();
      }
    };
    $('#total-rows').html(rows.length);
}

$(function() {

  // Column filering
  // Setup - add a text input to each footer cell
  $('#form-raport thead tr').clone(true).appendTo( '#form-raport thead' );
  $('#form-raport thead tr:eq(1) th').each( function (i) {
    var title = $(this).text();
    var width = '';
    $(this).html( '<input type="text" '+width+' placeholder="Search '+title+'" />' );
    $( 'input', this ).on( 'keyup change', function () {
      // a bit overhead, but need to have actual value when typing
      var colnr = table.colReorder.transpose(i);
      if ( table.column(colnr).search() !== this.value ) {
         table.column(colnr).search( this.value ).draw();
      }
    });
  });

  var table = $('#form-raport').DataTable({
    orderCellsTop: true,
    fixedHeader: true,
    fixedColumns: {
      leftColumns: 2
    },
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
        text: 'Eksportuj',
        buttons: ['copy', 'excel', 'csv' ,'pdf', 'print']
      },
      'colvis',
      {
        extend: 'selected',
        text: 'Toggle "ordered/payed"',
        action: function ( e, dt, button, config ) {
          indexes = dt.rows({selected: true}).indexes();
          for (i=0; i < indexes.length;i++){
            row = dt.row(indexes[i]).data()
            if (row[stscol] === "ordered") {
              row[stscol] = "payed";
              dt.row(indexes[i]).data(row);
              $.ajax({
                method: "POST",
                url: "/api/formstatus",
                data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol] , status: "payed"})
              });
            } else if (row[stscol] === "payed") {
              row[stscol] = "ordered";
              dt.row(indexes[i]).data(row);
              $.ajax({
                method: "POST",
                url: "/api/formstatus",
                data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol] , status: "ordered"})
              });
            } 
          }
          dt.rows({selected: true}).deselect();
          //$('#total-price').html(0);
          //$('#total-rows').html(0);
        }
      },
      {
        extend: "selected",
        text: "Kasuj",
        action: function ( e, dt, button, config ) {
          var furnNumberCol = table.colReorder.transpose(ChairNr);
          var roomNameCol = table.colReorder.transpose(Room);
          var indexes = dt.rows({selected: true}).indexes();
          bootbox.confirm({
            message: "Naprawde usunąć <b>" + indexes.length + "</b> zaznaczonych wpisów? Będą usunięte bezpowrotnie!",
              buttons: {
                cancel: {
                    label: '<i class="fa fa-times"></i> Anuluj'
                },
                confirm: {
                    label: '<i class="fa fa-check"></i> Kasuj'
                }
              },
            callback: function(result) {
              if (result) {
                for (i=0; i < indexes.length;i++){
                  var row = dt.row(indexes[i]).data();
                  $.ajax({
                    method: "DELETE",
                    url: "/api/formansdelete",
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
        text: "Usuń filtry",
        action: function ( e, dt, button, config ) {
          $('#form-raport thead tr:eq(1) th input').each( function (i) {
            table.column(i).search("");
            $(this).val("");
          });
          table.draw();
        }
      },
      {
        // select only visible, not all
        text: "Zaznacz wszystko",
        action: function ( e, dt, button, config ) {
          dt.rows( { page: 'current' } ).select();
        }
      },
      'selectNone',
    ]
  });

  // position info panel
  $("div#form-raport_length").css("display", "inline");
  $("div#form-raport_length").css("float", "left");
  $("div#form-raport_length").css("margin-right", "10px");

  // set content of total-price-lbl, it is created in "dom" table param
  $("div.total-price-lbl").css("display", "inline");
  $("div.total-price-lbl").css("margin-left", "5px");
  $("div.total-price-lbl").html('Wybranych: <span id="total-rows"></span></div>');

  // show total sits and price on select/deselect
  table.on( 'select', function ( e, dt, items ) {
    SetSums(dt);
  });

  table.on( 'deselect', function ( e, dt, items ) {
    SetSums(dt);
  });

});
