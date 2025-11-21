//
// Cluster heatmap interactions (standalone page)
//

function updateForm({page, order_by} = {}) {
  const form = document.getElementById('clusterHeatmapForm');
  if (!form) return;

  const pageInput = form.elements.namedItem("page");
  const orderInput = form.elements.namedItem("order_by");

  if (page !== undefined && pageInput) {
    pageInput.value = page;
  }

  if (order_by !== undefined && orderInput) {
    orderInput.value = order_by;
    if (pageInput) pageInput.value = 1;
  }

  form.submit();
}

document.addEventListener('DOMContentLoaded', function () {
  const form = document.getElementById('clusterHeatmapForm');
  if (form) {
    form.addEventListener('submit', function () {
      const pageInput = form.elements.namedItem('page');
      if (pageInput) pageInput.value = 1;
    });
  }

  // Toggle cell menus
  document.querySelectorAll("td").forEach(cell => {
    cell.addEventListener("click", function(e){
      if (e.target.closest(".menu")) return;

      const menu = this.querySelector(".menu");
      if (!menu) return;

      const isOpen = getComputedStyle(menu).display !== "none";
      document.querySelectorAll(".menu").forEach(m => { m.style.display = "none"; });
      if (!isOpen) menu.style.display = "block";
    });
  });

  document.querySelectorAll(".menu").forEach(menu => {
    menu.addEventListener("click", function(e){
      e.stopPropagation();
    });
  });

  document.querySelectorAll(".close-menu").forEach(closeLink => {
    closeLink.addEventListener("click", function(event){
      event.preventDefault();
      event.stopPropagation();
      this.closest('.menu').style.display = "none";
    });
  });

  // Toggle all genomes button
  const toggleButton = document.getElementById("toggle-all-genomes");
  const checkboxes = document.querySelectorAll(".genome-checkbox");
  if (toggleButton && checkboxes.length) {
    toggleButton.addEventListener("click", function () {
      const anyUnchecked = Array.from(checkboxes).some(checkbox => !checkbox.checked);
      checkboxes.forEach(checkbox => {checkbox.checked = anyUnchecked;});
    });
  }
});
