// document.querySelectorAll('.collapse-header').forEach(function(header) {
//     header.addEventListener('click', function() {
//         this.classList.toggle('active');
//         this.nextElementSibling.classList.toggle('active');
//     });
// });

(function () {
  const closeAll = (except) => {
    document.querySelectorAll('.collapsible').forEach(c => {
      if (c === except) return;
      c.querySelector('.collapse-header')?.classList.remove('active');
      c.querySelector('.collapse-content')?.classList.remove('active');
    });
  };

  // Set up each collapsible
  document.querySelectorAll('.collapsible').forEach(c => {
    const header = c.querySelector('.collapse-header');
    const content = c.querySelector('.collapse-content');
    if (!header || !content) return;

    // Toggle current; close the others
    header.addEventListener('click', (e) => {
      e.stopPropagation();
      const willOpen = !header.classList.contains('active');
      closeAll(willOpen ? c : null);
      header.classList.toggle('active', willOpen);
      content.classList.toggle('active', willOpen);
    });

    // Interacting inside shouldn't close it
    content.addEventListener('click', (e) => e.stopPropagation());
  });

  // Click anywhere else -> close all
  document.addEventListener('click', () => closeAll(null));

  // ESC to close
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeAll(null);
  });
})();