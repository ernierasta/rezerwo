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
          var stsrow = table.colReorder.transpose(8);
          indexes = dt.rows({selected: true}).indexes();
          for (i=0; i < indexes.length;i++){
            row = dt.row(indexes[i]).data()
            if (row[stsrow] === "ordered") {
              row[stsrow] = "payed";
              dt.row(indexes[i]).data(row);
            } else if (row[stsrow] === "payed") {
              row[stsrow] = "ordered";
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
    var pricerow = table.colReorder.transpose(6);
    for (i=0; i < rows.length;i++){
      price += Number(rows[i][pricerow]);
    };
    $('#total-price').html(price);
  });

});
