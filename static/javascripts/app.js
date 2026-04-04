document.addEventListener("DOMContentLoaded", function () {
    var searchInput = document.getElementById("catalog-search");
    var genreFilter = document.getElementById("genre-filter");
    var availableFilter = document.getElementById("available-filter");
    var bookCount = document.getElementById("book-count");
    var cards = document.querySelectorAll(".book-card");

    if (!searchInput || !cards.length) return;

    // Populate genre dropdown from book data
    var genres = [];
    cards.forEach(function (card) {
        var genre = card.getAttribute("data-genre");
        if (genre && genres.indexOf(genre) === -1) {
            genres.push(genre);
        }
    });
    genres.sort();
    genres.forEach(function (genre) {
        var option = document.createElement("option");
        option.value = genre;
        option.textContent = genre;
        genreFilter.appendChild(option);
    });

    function filterBooks() {
        var query = searchInput.value.toLowerCase();
        var genre = genreFilter.value;
        var onlyAvailable = availableFilter.checked;
        var visible = 0;

        cards.forEach(function (card) {
            var title = card.getAttribute("data-title").toLowerCase();
            var authors = card.getAttribute("data-authors").toLowerCase();
            var isbn = card.getAttribute("data-isbn").toLowerCase();
            var cardGenre = card.getAttribute("data-genre");
            var available = parseInt(card.getAttribute("data-available"), 10);

            var matchesSearch = !query || title.indexOf(query) !== -1 ||
                authors.indexOf(query) !== -1 || isbn.indexOf(query) !== -1;
            var matchesGenre = !genre || cardGenre === genre;
            var matchesAvailable = !onlyAvailable || available > 0;

            if (matchesSearch && matchesGenre && matchesAvailable) {
                card.style.display = "";
                visible++;
            } else {
                card.style.display = "none";
            }
        });

        bookCount.textContent = "Showing " + visible + " of " + cards.length + " books";
    }

    searchInput.addEventListener("input", filterBooks);
    genreFilter.addEventListener("change", filterBooks);
    availableFilter.addEventListener("change", filterBooks);
});
