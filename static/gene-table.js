// Interactions for the main gene table page

function submitGeneTableForm({ page, orderBy } = {}) {
  const form = document.getElementById('searchForm');
  if (!form) return;

  const pageInput = form.elements.namedItem('page');
  const orderInput = form.elements.namedItem('order_by');

  if (page !== undefined && pageInput) {
    pageInput.value = page;
  }

  if (orderBy !== undefined && orderInput) {
    orderInput.value = orderBy;
    if (pageInput) pageInput.value = 1;
  }

  form.submit();
}

function resetPageOnSearchSubmit() {
  const form = document.getElementById('searchForm');
  if (!form) return;

  form.addEventListener('submit', function () {
    const pageInput = form.elements.namedItem('page');
    if (pageInput) pageInput.value = 1; // reset to first page on any new search
  });
}

function attachCellMenus() {
  document.querySelectorAll('td').forEach(cell => {
    cell.addEventListener('click', function (e) {
      if (e.target.closest('.menu')) return;

      const menu = this.querySelector('.menu');
      if (!menu) return;

      const isOpen = getComputedStyle(menu).display !== 'none';
      document.querySelectorAll('.menu').forEach(m => { m.style.display = 'none'; });
      if (!isOpen) menu.style.display = 'block';
    });
  });

  document.querySelectorAll('.menu').forEach(menu => {
    menu.addEventListener('click', function (e) {
      e.stopPropagation();
    });
  });

  document.querySelectorAll('.close-menu').forEach(closeLink => {
    closeLink.addEventListener('click', function (event) {
      event.preventDefault();
      event.stopPropagation();
      this.closest('.menu').style.display = 'none';
    });
  });
}

function attachBlastFormHandler() {
  const form = document.getElementById('searchBLAST');
  if (!form) return;

  form.addEventListener('submit', function (e) {
    e.preventDefault();

    const formData = new FormData(form);
    const jsonData = {};
    formData.forEach((value, key) => {
      jsonData[key] = value;
    });

    const newWindow = window.open('', '_blank');

    fetch('/blast', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      body: JSON.stringify(jsonData),
    })
    .then(response => {
      if (!response.ok) {
        throw new Error('Network response was not ok');
      }
      return response.json();
    })
    .then(data => {
      const jobId = data.job_id;
      if (!jobId) {
        throw new Error('Missing job ID in response');
      }
      const targetUrl = `/blast/${encodeURIComponent(jobId)}`;
      if (newWindow) {
        newWindow.location = targetUrl;
      } else {
        window.open(targetUrl, '_blank');
      }
    })
    .catch((error) => {
      console.error('Error:', error);

      const errorMessage = '<h1>Error</h1><p>Unable to start BLAST search. Please try again later.</p>';
      if (newWindow) {
        newWindow.document.open();
        newWindow.document.write(errorMessage);
        newWindow.document.close();
      } else {
        alert('Unable to start BLAST search. Please try again later.');
      }
    });
  });
}

function attachGenomeToggle() {
  const toggleButton = document.getElementById('toggle-all-genomes');
  const checkboxes = document.querySelectorAll('.genome-checkbox');
  if (!toggleButton || !checkboxes.length) return;

  toggleButton.addEventListener('click', function () {
    const anyUnchecked = Array.from(checkboxes).some(checkbox => !checkbox.checked);
    checkboxes.forEach(checkbox => { checkbox.checked = anyUnchecked; });
  });
}

document.addEventListener('DOMContentLoaded', function () {
  resetPageOnSearchSubmit();
  attachCellMenus();
  attachBlastFormHandler();
  attachGenomeToggle();
});
