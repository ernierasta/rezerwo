$(function() {

  var table = $('#reservations').DataTable({
    columnDefs: [{
      orderable: false,
      className: 'select-checkbox',
      targets:   0
    }],
    select: {
      style: 'multi', //'os'
    },
    colReorder: true,
    dom: 'Blfrtip',
    buttons: [ 
      {
        extend: 'collection',
        text: 'Export',
        buttons: ['copy', 'excel', 'csv' ,'pdf']
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
