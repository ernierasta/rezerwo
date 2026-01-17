var ID = 0;
var ChairNr = 1;
var Room = 2;
var LastNotif = 3;
var NotifAmmount = 4; 
var Name = 5;
var Surname = 6;
var OrderStatus = 7;
var Email = 8;
var Notes = 9;
var Phone = 10;
var Price = 11;
var Currency = 12;
var Ordered = 13;
var Payed = 14;
var RoomID = 15;

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
    $(this).html( '<input type="text" '+width+' placeholder="Szukaj '+title+'" />' );
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
        text: 'Exportuj',
        buttons: ['copy', 'excel', 'csv' ,'pdf', 'print']
      },
      'colvis',
      {
        extend: 'selected',
        text: 'Przełącz "zamówiono/zapłacono"',
        action: function ( e, dt, button, config ) {
          var stscol = table.colReorder.transpose(OrderStatus);
          var furnNumberCol = table.colReorder.transpose(ChairNr);
          var roomNameCol = table.colReorder.transpose(Room);
	  var roomIDCol = table.colReorder.transpose(RoomID);
          var payedCol = table.colReorder.transpose(Payed);
          var now = new Date();
          indexes = dt.rows({selected: true}).indexes();
          for (i=0; i < indexes.length;i++){
            row = dt.row(indexes[i]).data()
            if (row[stscol] === "ordered") {
              row[stscol] = "payed";
              row[payedCol] = now.toISOString().slice(0, 16).replace('T', ' ');
              dt.row(indexes[i]).data(row);
              $.ajax({
                method: "POST",
                url: "/api/resstatus",
		      data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol], room_id: Number(row[roomIDCol]) , status: "payed"})
              });
            } else if (row[stscol] === "payed") {
              row[stscol] = "ordered";
              row[payedCol] = "1970-01-01 01:00";
              dt.row(indexes[i]).data(row);
              $.ajax({
                method: "POST",
                url: "/api/resstatus",
		      data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_name: row[roomNameCol], room_id: Number(row[roomIDCol]) , status: "ordered"})
              });
            } 
          }
          dt.rows({selected: true}).deselect();
          //$('#total-price').html(0);
          //$('#total-sits').html(0);
        }
      },
      {
        extend: 'selected',
        text: 'Wyślij e-mail',
        action: function ( e, dt, button, config ) {
          var reservationID = table.colReorder.transpose(ID);
	  var cust_email = table.colReorder.transpose(Email);
	  var notif_ammount = table.colReorder.transpose(NotifAmmount);
	  var last_notif = table.colReorder.transpose(LastNotif);
          var indexes = dt.rows({selected: true}).indexes();
          var notificationID = Number($('#notif-select').val());
          
          
          // check if mail template is selected - if 0 abort
          if (notificationID == 0 ) {
            bootbox.alert("Nie wybrano szablonu maila, wybierz powyżej i spróbuj ponownie.");
            return
          }

          // confirm sending - send mail if confirmed
          bootbox.confirm({
                        message: "Naprawde wysłać e-mail do wybranych <b>" + indexes.length + "</b> pozycji? Dla każdego zamówienia zostanie wysłany tylko jeden e-mail (szablon: " + $('#notif-select').find('option:selected').text() + ").",
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
		// first we need to agregate all orders by mail (the same email = one order)
		const agregated = {};
                for (i=0; i < indexes.length;i++){
                	var row = dt.row(indexes[i]).data();
			if (!agregated[row[cust_email]]) { // create object: key: mail, values: [reservationID, reservationID2, ...]
				agregated[row[cust_email]] = []; // initialize empty table for this key (email)
			}
			agregated[row[cust_email]].push(row[reservationID]);

			// while we are iterating rows, set values to new values
			row[notif_ammount] = Number(row[notif_ammount]) + 1;
                  	row[last_notif] = "Teraz";
                  	dt.row(indexes[i]).data(row);
		}

		for (const key in agregated) { // remake object, so it is: key: mail, value: "reservationID, reservationID2, ..."
			agregated[key] = agregated[key].join(', ');
		}


		for (const key in agregated){ // iterate mails
                  $.ajax({
                    method: "POST",
                    url: "/api/eventanssendmail",
                    data: JSON.stringify({event_id: Number($('#event-id').val()), reservation_ids: agregated[key], cust_email: key, notification_id: notificationID})
                  });
                }
		// at the end deselect selected
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
          var furnNumberCol = table.colReorder.transpose(ChairNr);
          var roomNameCol = table.colReorder.transpose(Room);
	  var roomIDCol = table.colReorder.transpose(RoomID);
          var indexes = dt.rows({selected: true}).indexes();
          bootbox.confirm({
            message: "Na pewno wykasować zaznaczone rezerwacje? Kasowanie jest NIEODWRACALNE!",
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
                    url: "/api/resdelete",
                    data: JSON.stringify({event_id: Number($('#event-id').val()),furn_number: Number(row[furnNumberCol]), room_id: Number(row[roomIDCol]), room_name: row[roomNameCol]})
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
          $('#reservations thead tr:eq(1) th input').each( function (i) {
            table.column(i).search("");
            $(this).val("");
          });
          table.draw();
        }
      },
      {
        // select only visible, not all
        text: "Wybierz wszystkie",
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
  $("div.total-price-lbl").html('Kwota: <span id="total-price"></span><span>, Krzesła: <span id="total-sits"></span></div>');

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
