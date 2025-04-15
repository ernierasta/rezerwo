function GoBack() {
    window.history.back();
}

var QuillMailTextEditor = new Quill('#text-editor', {
    theme: 'snow'
});

// We are adding button ✨ to Quill toolbar. 
// After user presses it, it will show combobox with
// all forms availabile. After selecting one, definitions
// for all fields (which make sense) will be generated.
// Backend returns "Delta" definitions for Quill, so it will
// work well.

// Function to fetch options from backend
function fetchOptionsFromBackend() {
    return new Promise((resolve, reject) => {
        $.ajax({
            type: "GET",
            url: "/api/formstmpls",
            success: function(resp) {
                console.log("resp:", resp);
                resolve(resp); // Resolve with the response data
            },
            error: function(xhr, status, error) {
                reject(error); // Reject on error
            },
            statusCode: {
                418: function() {
                    alert("Wczytywanie nazw formularzy się nie udało!");
                    reject(new Error("Nie udało się wczytać nazw formularzy."));
                }
            },
            dataType: "json"
        });
    });
}

// Function that returns the Delta to insert, takes formtemplate ID as parameter
async function getCustomText(id) {
    try {
        var sendjson = {
            type: "notification",
            id: parseInt(id)
        };

        console.log("Sending JSON:", JSON.stringify(sendjson)); // Debug

        const response = await $.ajax({
            type: "POST",
            url: "/api/formdefs",
            data: JSON.stringify(sendjson),
            contentType: "application/json",
            dataType: "json"
        });

        console.log("Response:", response); // Debug
        if (response && response.msg && Array.isArray(response.msg.ops)) {
            return response.msg; // Return the Delta object
        } else {
            console.warn("Invalid response.msg:", response);
            return { ops: [{ insert: "Error: Invalid response from server" }] };
        }
    } catch (error) {
        console.error("AJAX error:", error);
        if (error.status === 418) {
            alert("Nie udało się wygenerować definicji pól.");
            console.log("418 Response:", error.responseJSON?.msg);
            return { ops: [{ insert: "error" }] };
        }
        return { ops: [{ insert: "Error: Failed to fetch text" }] };
    }
}

// Add custom button to toolbar
var toolbar = QuillMailTextEditor.getModule('toolbar');

// Create button element
var customButton = document.createElement('button');
customButton.innerHTML = '✨'; // Using Sparkles icon
customButton.className = 'ql-custom';
customButton.title = 'Generuj odpowiedzi';

// Create span element with class 'ql-formats'
var buttonWrapper = document.createElement('span');
buttonWrapper.className = 'ql-formats';
buttonWrapper.appendChild(customButton);

// Function to create and show popup
function showPopup() {
    // Create popup container
    var popup = document.createElement('div');
    popup.style.position = 'absolute';
    popup.style.background = '#fff';
    popup.style.border = '1px solid #ccc';
    popup.style.padding = '10px';
    popup.style.zIndex = '1000';
    popup.style.boxShadow = '0 2px 5px rgba(0,0,0,0.2)';
    popup.style.top = '540px'; // Kept your value
    popup.style.left = '50%';
    popup.style.transform = 'translateX(-50%)';

    // Create combo box
    var select = document.createElement('select');
    select.style.padding = '5px';
    select.style.minWidth = '150px';

    // Add loading option
    var loadingOption = document.createElement('option');
    loadingOption.text = 'Loading...';
    loadingOption.disabled = true;
    loadingOption.selected = true;
    select.appendChild(loadingOption);

    // Create close button
    var closeButton = document.createElement('button');
    closeButton.innerHTML = 'Anuluj';
    closeButton.style.marginLeft = '10px';
    closeButton.className = 'btn btn-primary';
    closeButton.onclick = () => document.body.removeChild(popup);

    // Create a container for the close button to center it
    var buttonContainer = document.createElement('div');
    buttonContainer.style.textAlign = 'center';
    buttonContainer.style.marginTop = '10px'; // Space above the button
    buttonContainer.appendChild(closeButton);

    // Append elements to popup
    popup.appendChild(select);
    popup.appendChild(buttonContainer);

    // Append popup to body
    document.body.appendChild(popup);

    // Fetch options and populate combo box
    fetchOptionsFromBackend().then(options => {
        // Clear loading option
        select.innerHTML = '';
        select.className = 'custom-select';

        // Add default "Select an option" prompt
        var defaultOption = document.createElement('option');
        defaultOption.text = 'Wybierz formularz';
        defaultOption.value = '';
        defaultOption.disabled = true;
        defaultOption.selected = true;
        select.appendChild(defaultOption);

        // Populate options
        options.forEach(opt => {
            var option = document.createElement('option');
            option.value = opt.ID;
            option.text = opt.Name;
            select.appendChild(option);
        });
    }).catch(err => {
        console.error('Failed to load options:', err);
        select.innerHTML = '<option>Error loading options</option>';
    });

    // Handle selection

    // Handle selection
    select.onchange = async function() {
        var selectedId = select.value;
        if (selectedId) {
            try {
                var deltaToInsert = await getCustomText(selectedId);
                console.log("Delta to insert:", deltaToInsert); // Debug
    
                // Ensure deltaToInsert is a valid Delta
                if (!deltaToInsert || !Array.isArray(deltaToInsert.ops)) {
                    console.error("deltaToInsert is not a valid Delta:", deltaToInsert);
                    deltaToInsert = { ops: [{ insert: "Error: Invalid Delta received" }] };
                }
    
                var range = QuillMailTextEditor.getSelection();
                if (range) {
                    QuillMailTextEditor.updateContents(deltaToInsert, 'user');
                } else {
                    QuillMailTextEditor.updateContents(deltaToInsert, 'user');
                }
            } catch (err) {
                console.error("Error inserting Delta:", err);
                alert("Nie udało się wstawić tekstu.");
                deltaToInsert = { ops: [{ insert: "Error: Failed to insert text" }] };
                QuillMailTextEditor.updateContents(deltaToInsert, 'user');
            }
            // Close popup after selection
            document.body.removeChild(popup);
        }
    };
}

// Add click event listener to the button
customButton.addEventListener('click', showPopup);

// Append the span (containing the button) as the last element in the toolbar
toolbar.container.appendChild(buttonWrapper);

// GetTemplate will call API and retrieve template for all form fields
function GetTemplate() {
    // Placeholder, unchanged
}

// Save function
function Save() {
    var finald = {};
    finald.id = $('#notification-id').val();
    finald.userid = $('#user-id').val();
    finald.name = $('#name').val();
    finald.subject = $('#subject').val();
    finald.text = QuillMailTextEditor.root.innerHTML;
    finald.sharable = $('#sharable').is(":checked");
    finald.relatedto = $('#related-to-select').val(); // events or forms
    finald.attachedfiles = $('#attachedfiles').val();
    finald.embeddedimgs = $('#embeddedimgs').val();

    console.log(finald.sharable);

    $.ajax({
        type: "POST",
        url: "/api/maed",
        data: JSON.stringify(finald),
        success: function(resp) {
            console.log(resp.msg);
            // Make button green
            $("#save").addClass("btn-success");
            // Set timer to make button blue again
            window.setTimeout(function() {
                $("#save").removeClass("btn-success");
            }, 2000);
            GoBack();
        },
        statusCode: {
            418: function(xhr) {
                $("#save").addClass("btn-danger");
                // Set timer to make button blue again
                window.setTimeout(function() {
                    $("#save").removeClass("btn-danger");
                }, 2000);
                alert(xhr.responseJSON.msg);
                console.log(xhr.responseJSON.msg);
            }
        },
        dataType: "json"
    });
}
