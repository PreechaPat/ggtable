// Re-set the value of url when changing
function updatePage(targetPage) {

    const form = document.getElementById('searchForm');
    const pageInput = form.elements.namedItem('page')

    // Set the target page
    pageInput.value = targetPage;
  
    // Submit the form
    form.submit();
}

// Apply event listener to all cells with menu
document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll("td").forEach(cell => {
        cell.addEventListener("click", function(){
            // Find the specific menu within this cell
            var menu = this.querySelector(".menu");
            if (menu.style.display === "block") {
                menu.style.display = "none";
            } else {
                // Hide any other open menus
                document.querySelectorAll(".menu").forEach(m => {
                    m.style.display = "none";
                });
                menu.style.display = "block";
            }
        });
    });

    document.querySelectorAll(".close-menu").forEach(closeLink => {
        closeLink.addEventListener("click", function(event){
            event.preventDefault();
            event.stopPropagation();
            this.closest('.menu').style.display = "none";
        });
    });
});


document.addEventListener('DOMContentLoaded', function () {
    const form = document.getElementById('searchBLAST');

    form.addEventListener('submit', function (e) {
        e.preventDefault();

        const formData = new FormData(form);
        const jsonData = {};
        formData.forEach((value, key) => {
            jsonData[key] = value;
        });

        // Open a new window
        const newWindow = window.open('', '_blank');

        fetch('/blast', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(jsonData),
        })
            .then(response => {
                if (!response.ok) {
                    throw new Error('Network response was not ok');
                }
                return response.text();
            })
            .then(data => {
                // Write the response into the new window
                newWindow.document.open();
                newWindow.document.write(data);
                newWindow.document.close();

                // Optionally, update the new window's URL (this doesn't affect the browser's history)
                newWindow.history.pushState({}, '', '/blast');
            })
            .catch((error) => {
                console.error('Error:', error);

                // Display an error message in the new window
                newWindow.document.open();
                newWindow.document.write('<h1>Error</h1><p>Unable to fetch data. Please try again later.</p>');
                newWindow.document.close();
            });
    });
});


//
// Function to save / load form value from local storage
// 
// function saveFormValues(form) {
//
//     const formData = new FormData(form);
//     const formId = form.id || `form_${Array.from(document.forms).indexOf(form)}`;
//
//     // for (const [key, value] of formData.entries()) {
//     //     localStorage.setItem(`${formId}_${key}`, value);
//     // }
//     Array.from(form.elements).forEach(element => {
//         if (element.name) {
//             if (element.type === 'checkbox' || element.type === 'radio') {
//                 localStorage.setItem(`${formId}_${element.name}`, element.checked);
//             } else {
//                 localStorage.setItem(`${formId}_${element.name}`, element.value);
//             }
//         }
//     });
// }
//
// // Function to load form values from localStorage
// function loadFormValues(form) {
//     const formId = form.id || `form_${Array.from(document.forms).indexOf(form)}`;
//
//     Array.from(form.elements).forEach(element => {
//         if (element.name) {
//             const savedValue = localStorage.getItem(`${formId}_${element.name}`);
//             if (savedValue !== null) {
//                 if (element.type === 'checkbox' || element.type === 'radio') {
//                     element.checked = savedValue === 'true';
//                 } else if (element.tagName === 'SELECT') {
//                     const option = Array.from(element.options).find(opt => opt.value === savedValue);
//                     if (option) option.selected = true;
//                 } else {
//                     element.value = savedValue;
//                 }
//             } else if (element.type === 'checkbox' && element.hasAttribute('checked')) {
//                 // Use default 'checked' if no saved value exists
//                 element.checked = true;
//             }
//         }
//     });
// }
//
// // Function to clear saved form data
// function clearSavedFormData(form) {
//     const formId = form.id || `form_${Array.from(document.forms).indexOf(form)}`;
//
//     Array.from(form.elements).forEach(element => {
//         if (element.name) {
//             localStorage.removeItem(`${formId}_${element.name}`);
//         }
//     });
// }
//
// document.addEventListener('DOMContentLoaded', function() {
//     const forms = document.forms;
//
//     // Add submit event listener to each form
//     Array.from(forms).forEach(form => {
//         form.addEventListener('submit', function(event) {
//             saveFormValues(this); // 'this' refers to the form element
//         });
//
//         // Load saved values for each form
//         loadFormValues(form);
//     });
// });

//
// Add toggle genomes button for select/deselect all genomes
//
document.addEventListener("DOMContentLoaded", function () {
    const toggleButton = document.getElementById("toggle-all-genomes");
    const checkboxes = document.querySelectorAll(".genome-checkbox");

    toggleButton.addEventListener("click", function () {
        // Determine if any checkboxes are unchecked
        const anyUnchecked = Array.from(checkboxes).some(checkbox => !checkbox.checked);

        // Set all checkboxes to checked if any are unchecked, otherwise uncheck all
        checkboxes.forEach(checkbox => {checkbox.checked = anyUnchecked;});
    });
});

document.addEventListener("DOMContentLoaded", function () {

    // Gene toggle logic
    const geneToggleButton = document.getElementById("toggle-all-genes");
    const geneCheckboxes = document.querySelectorAll(".gene-checkbox");

    geneToggleButton.addEventListener("click", function () {
        const anyUnchecked = Array.from(geneCheckboxes).some(cb => !cb.checked);
        geneCheckboxes.forEach(cb => {cb.checked = anyUnchecked;});
    });
});

