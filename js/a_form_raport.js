var FormID = 0;
var Status = 1;
var LastNotif = 2;
var NotifsSent = 3;
var Name = 4;
var Surname = 5;
var Email = 6;
var CreatedDate = 7;
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
        text: 'Wyślij e-mail',
        action: function ( e, dt, button, config ) {
          var formID = table.colReorder.transpose(FormID);
          var indexes = dt.rows({selected: true}).indexes();
          var notificationID = Number($('#notif-select').val());
          
          
          // check if mail template is selected - if 0 abort
          if (notificationID == 0 ) {
            bootbox.alert("Nie wybrano szablonu maila, wybierz powyżej i spróbuj ponownie.");
            return
          }

          // confirm sending - send mail if confirmed
          bootbox.confirm({
                        message: "Naprawde wysłać <b>" + indexes.length + "</b> e-maili (szablon: " + $('#notif-select').find('option:selected').text() + ")?",
              buttons: {
                cancel: {
                    label: '<i class="fa fa-times"></i> Anuluj'
                },
                confirm: {
                    label: '<i class="fa fa-check"></i> Wyślij'
                }
              },
            callback: function(result) {
              if (result) {
                for (i=0; i < indexes.length;i++){
                  var row = dt.row(indexes[i]).data();
                  $.ajax({
                    method: "POST",
                    url: "/api/formanssendmail",
                    data: JSON.stringify({formtmpl_id: Number($('#formtmpl-id').val()),forms_id: Number(row[formID]), notification_id: notificationID})
                  });
                  row[NotifsSent] = Number(row[NotifsSent]) + 1;
                  row[LastNotif] = "Teraz";
                  dt.row(indexes[i]).data(row);
                }
                dt.rows({selected: true}).deselect();
              }
            }
          });
        }
      },
      {
        extend: "selected",
        text: "Kasuj",
        action: function ( e, dt, button, config ) {
          var formID = table.colReorder.transpose(0);
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
                    data: JSON.stringify({formtmpl_id: Number($('#formtmpl-id').val()),forms_id: Number(row[formID])})
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
    ],
    language: {
      buttons: {
        colvis: 'Wyświetlane kolumny',
        selectNone: "Odznacz wszystkie",
      }
    }
  });

  // position info panel
  $("div#form-raport_length").css("display", "inline");
  $("div#form-raport_length").css("float", "left");
  $("div#form-raport_length").css("margin-right", "10px");

  // set content of total-price-lbl, it is created in "dom" table param
  $("div.total-price-lbl").css("display", "inline");
  $("div.total-price-lbl").css("margin-left", "5px");
  $("div.total-price-lbl").html('<div>Wybranych: <span id="total-rows"></span></div>');

  // show total sits and price on select/deselect
  table.on( 'select', function ( e, dt, items ) {
    SetSums(dt);
  });

  table.on( 'deselect', function ( e, dt, items ) {
    SetSums(dt);
  });

});
