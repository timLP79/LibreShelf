document.addEventListener("DOMContentLoaded", function () {
    initCatalogFilter();
    initStaffManagement();
});

function initCatalogFilter() {
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
}

function initStaffManagement() {
    var editModal = document.getElementById("editStaffModal");
    var deleteModal = document.getElementById("deleteStaffModal");
    if (!editModal && !deleteModal) return;

    // Populate Edit modal from clicked row's data attributes
    if (editModal) {
        var editForm = document.getElementById("editStaffForm");
        var editUsername = document.getElementById("edit-username");
        var editRole = document.getElementById("edit-role");
        var editNote = document.getElementById("edit-role-note");

        document.querySelectorAll(".edit-btn").forEach(function (btn) {
            btn.addEventListener("click", function () {
                var id = btn.getAttribute("data-user-id");
                var username = btn.getAttribute("data-username");
                var role = btn.getAttribute("data-role");
                var isSelf = btn.getAttribute("data-is-self") === "true";
                var isLastAdmin = btn.getAttribute("data-is-last-admin") === "true";

                editForm.action = "/staff/" + id + "/edit";
                editUsername.value = username;
                editRole.value = role;

                // Reset options
                Array.from(editRole.options).forEach(function (opt) {
                    opt.disabled = false;
                });
                editRole.disabled = false;
                editNote.textContent = "";

                if (isSelf) {
                    // Cannot change own role
                    editRole.disabled = true;
                    editNote.textContent = "You cannot change your own role.";
                } else if (isLastAdmin) {
                    // Cannot demote the last admin
                    Array.from(editRole.options).forEach(function (opt) {
                        if (opt.value === "staff") opt.disabled = true;
                    });
                    editNote.textContent = "This is the last admin account. Create another admin before demoting.";
                }
            });
        });
    }

    // Type-to-confirm delete
    if (deleteModal) {
        var deleteForm = document.getElementById("deleteStaffForm");
        var deleteTargetName = document.getElementById("delete-target-name");
        var deleteTargetRole = document.getElementById("delete-target-role");
        var deleteInput = document.getElementById("delete-confirm-input");
        var deleteBtn = document.getElementById("delete-confirm-btn");
        var expectedUsername = "";

        document.querySelectorAll(".delete-btn").forEach(function (btn) {
            btn.addEventListener("click", function () {
                var id = btn.getAttribute("data-user-id");
                expectedUsername = btn.getAttribute("data-username");
                var role = btn.getAttribute("data-role");

                deleteForm.action = "/staff/" + id + "/delete";
                deleteTargetName.textContent = expectedUsername;
                deleteTargetRole.textContent = role;
                deleteInput.value = "";
                deleteBtn.disabled = true;
            });
        });

        deleteInput.addEventListener("input", function () {
            deleteBtn.disabled = deleteInput.value !== expectedUsername;
        });

        // Reset on close
        deleteModal.addEventListener("hidden.bs.modal", function () {
            deleteInput.value = "";
            deleteBtn.disabled = true;
        });
    }
}
