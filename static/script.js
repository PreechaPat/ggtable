//
// Forms
//

// Ensure/replace a hidden field on the form
// function upsertHidden(form, name, value) {
//   let input = form.elements.namedItem(name);
//   if (!input) {
//     input = document.createElement('input');
//     input.type = 'hidden';
//     input.name = name;
//     form.appendChild(input);
//   }
//   input.value = value;
// }

// Re-set the value of url when changing
// Currently used for changing page 
// function updatePage(targetPage) {

//     const form = document.getElementById('searchForm');
//     if (!form) {
//         console.error("Form with id 'searchForm' not found.");
//         return;
//     }
//     const pageInput = form.elements.namedItem('page')

//     // Set the target page
//     pageInput.value = targetPage;
  
//     // Submit the form
//     form.submit();
// }

function updateForm({page, order_by} = {}) {
  const form = document.getElementById('searchForm');
  if (!form) return;

  const params = new URLSearchParams(new FormData(form));

  if (page !== undefined) {
    form.elements.namedItem("page").value = page
  }

  if (order_by !== undefined) {
    form.elements.namedItem("order_by").value = order_by
    // Reset page to one
    form.elements.namedItem("page").value = 1
  }

  // Submit the form
  form.submit();
}

// Make submit button always go to page 1 when searching:
document.addEventListener('DOMContentLoaded', function () {
  const form = document.getElementById('searchForm');
  if (!form) return;

  form.addEventListener('submit', function () {
    const pageInput = form.elements.namedItem('page');
    if (pageInput) pageInput.value = 1; // reset to first page on any new search
  });
});

// Apply event listener to all cells with menu
document.addEventListener('DOMContentLoaded', function() {
  // Toggle menu only when clicking the TD *outside* the menu
  document.querySelectorAll("td").forEach(cell => {
    cell.addEventListener("click", function(e){
      // If the click was inside a .menu, ignore it
      if (e.target.closest(".menu")) return;

      const menu = this.querySelector(".menu");
      if (!menu) return;

      const isOpen = getComputedStyle(menu).display !== "none";

      // Close all menus first
      document.querySelectorAll(".menu").forEach(m => { m.style.display = "none"; });

      // Open this one if it wasn't open
      if (!isOpen) menu.style.display = "block";
    });
  });

  // Prevent clicks inside the menu from bubbling to the TD
  document.querySelectorAll(".menu").forEach(menu => {
    menu.addEventListener("click", function(e){
      e.stopPropagation();
    });
  });

  // Close button behavior
  document.querySelectorAll(".close-menu").forEach(closeLink => {
    closeLink.addEventListener("click", function(event){
      event.preventDefault();
      event.stopPropagation();
      this.closest('.menu').style.display = "none";
    });
  });
});

// Menu on heatmap cell
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

// document.addEventListener("DOMContentLoaded", function () {

//     // Gene toggle logic
//     const geneToggleButton = document.getElementById("toggle-all-genes");
//     const geneCheckboxes = document.querySelectorAll(".gene-checkbox");

//     geneToggleButton.addEventListener("click", function () {
//         const anyUnchecked = Array.from(geneCheckboxes).some(cb => !cb.checked);
//         geneCheckboxes.forEach(cb => {cb.checked = anyUnchecked;});
//     });
// });

