// This are definitions for the custom field "multiplierField".
// It is included in the a_form_editor.js file and tmpl/form_renderer.html

// Define the custom field template
const templates = {
  multiplierField: function(fieldData) {
    //console.log(fieldData);
    return {
      field: `<div>
<input type="number" class="multiplier-input ${fieldData.className}" name="${fieldData.name}-input" />
<span class="multiplier-pre"></span><span name="${fieldData.name}-span" class="${fieldData.name}-value"></span><span class="multiplier-post"></span><input type="hidden" name="${fieldData.name}" class="${fieldData.name}-hidden"></input>
</div>`,
      onRender: function() {
        const input = document.querySelector('.multiplier-input');
        const valueDisp = document.querySelector('.'+fieldData.name+'-value');
        const valueHidden = document.querySelector('.'+fieldData.name+'-hidden');
        const preDisp = document.querySelector('.multiplier-pre');
        const postDisp = document.querySelector('.multiplier-post');
        const multiplier = Number(fieldData.multipl) || 1; // Default multiplier
        const pre = fieldData.pre || '';
        const post = fieldData.post || '';

        input.addEventListener('input', function() {
          //const value = parseFloat(input.value) || 0;
          const value = Number(input.value) || 0;
          preDisp.textContent = `${pre}`;
          valueDisp.textContent = `${value * multiplier}`;
          valueHidden.value = `${value} * ${multiplier} = ${value * multiplier}`; // we read value from this hidden input
          postDisp.textContent = `${post}`;
        });
      }
    };
  }
};

 const customFields = [{
      label: 'Wielokrotność',
      type: 'multiplierField',
      icon: '×'
      //multiplier: 600 // Default multiplier value
    }]

const userAttrs = {
    multiplierField: {
      multipl: {
        label: 'Razy',
        type: 'text',
        value: '600' // Default value; this should be a number, but it doesn't work, so we convert later
      },
      pre: {
        label: 'Przed',
        type: 'text',
        value: '(' // Default value
      },
      post: {
        label: 'Po (waluta)',
        type: 'text',
        value: ' Kč)' // Default value
      },
    }
  }

