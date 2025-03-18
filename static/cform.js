document.querySelectorAll('.collapse-header').forEach(function(header) {
    header.addEventListener('click', function() {
        this.classList.toggle('active');
        this.nextElementSibling.classList.toggle('active');
    });
});