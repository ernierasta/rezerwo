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
          indexes = dt.rows({selected: true}).indexes();
          for (i=0; i < indexes.length;i++){
            row = dt.row(indexes[i]).data()
            if (row[8] === "ordered") {
              row[8] = "payed";
              dt.row(indexes[i]).data(row);
            } else if (row[8] === "payed") {
              row[8] = "ordered";
              dt.row(indexes[i]).data(row);
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
    for (i=0; i < rows.length;i++){
      price += Number(rows[i][6]);
    };
    $('#total-price').html(price);
  });

});
