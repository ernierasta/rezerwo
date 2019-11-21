$(function() {
   // Setup - add a text input to each footer cell
  $('#reservations thead tr').clone(true).appendTo( '#reservations thead' );
  $('#reservations thead tr:eq(1) th').each( function (i) {
      var title = $(this).text();
      $(this).html( '<input type="text" placeholder="Search '+title+'" />' );

      $( 'input', this ).on( 'keyup change', function () {
          if ( table.column(i).search() !== this.value ) {
              table
                  .column(i)
                  .search( this.value )
                  .draw();
          }
      } );
  } );

  var table = $('#reservations').DataTable({
    orderCellsTop: true,
    fixedHeader: true,
    colReorder: true,
    columnDefs: [{
      orderable: true,
      className: 'select-checkbox',
      targets:   0
    }],
    'lengthMenu': [ [10, 50, 100, -1], [10, 50, 100, "All"] ],
    'pageLength': -1,
    select: {
      style: 'multi', //'os'
    },
    // TODO: figure how to make it usefull
    //rowGroup: {
    //    dataSrc: 'group'
    //},
    dom: 'Blfrtip',
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
          var stscol = table.colReorder.transpose(8);
          var furnNumberCol = table.colReorder.transpose(0);
          var roomNameCol = table.colReorder.transpose(1);
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
          //dt.rows({selected: true}).deselect();
        }
      },
      {
        extend: "selected",
        text: "Delete",
        action: function ( e, dt, button, config ) {
          var furnNumberCol = table.colReorder.transpose(0);
          var roomNameCol = table.colReorder.transpose(1);
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
        // select only visible, not all
        text: "Select All",
        action: function ( e, dt, button, config ) {
          dt.rows( { page: 'current' } ).select();
        }
      },
      'selectNone',
    ]
  });


  table.on( 'select', function ( e, dt, items ) {
    var rows = dt.rows({selected: true}).data();
    var price = 0;
    var pricerow = table.colReorder.transpose(6);
    for (i=0; i < rows.length;i++){
      price += Number(rows[i][pricerow]);
    };
    $('#total-price').html(price);
  });

});
