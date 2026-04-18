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
    var resetModal = document.getElementById("resetPasswordModal");
    if (!editModal && !deleteModal && !resetModal) return;

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

    // Reset Password modal
    if (resetModal) {
        var resetForm = document.getElementById("resetPasswordForm");
        var resetTargetName = document.getElementById("reset-target-name");
        var resetPassword = document.getElementById("reset-password");
        var resetPasswordConfirm = document.getElementById("reset-password-confirm");

        document.querySelectorAll(".reset-btn").forEach(function (btn) {
            btn.addEventListener("click", function () {
                var id = btn.getAttribute("data-user-id");
                var username = btn.getAttribute("data-username");

                resetForm.action = "/staff/" + id + "/password";
                resetTargetName.textContent = username;
                resetPassword.value = "";
                resetPasswordConfirm.value = "";
            });
        });

        resetModal.addEventListener("hidden.bs.modal", function () {
            resetPassword.value = "";
            resetPasswordConfirm.value = "";
        });
    }

    attachFormValidation(document.getElementById("addStaffForm"), {
        password: "#add-password",
        confirm: "#add-password-confirm",
        modal: document.getElementById("addStaffModal"),
    });
    attachFormValidation(document.getElementById("editStaffForm"), {
        modal: editModal,
    });
    attachFormValidation(document.getElementById("resetPasswordForm"), {
        password: "#reset-password",
        confirm: "#reset-password-confirm",
        modal: resetModal,
    });
}

// attachFormValidation wires Bootstrap 5 per-field live validation onto
// a form. Each non-hidden input/select gets input+blur listeners that
// toggle .is-invalid or .is-valid on the field based on its validity,
// so the red/green feedback and .invalid-feedback message appear as the
// user types. Empty fields stay neutral (no red until the user has
// interacted or submitted) so the modal does not nag on open. On submit,
// every field is forced through validation so untouched empty requireds
// also light up red. Server-side validation remains the source of truth;
// this is the client-side short-circuit so the user does not round-trip
// for client-detectable mistakes.
function attachFormValidation(form, options) {
    if (!form) return;
    options = options || {};
    var passwordField = options.password ? form.querySelector(options.password) : null;
    var confirmField = options.confirm ? form.querySelector(options.confirm) : null;

    function checkMatch() {
        if (!passwordField || !confirmField) return;
        if (confirmField.value && passwordField.value !== confirmField.value) {
            confirmField.setCustomValidity("mismatch");
        } else {
            confirmField.setCustomValidity("");
        }
    }

    function markField(field, includeEmpty) {
        if (!includeEmpty && field.value === "") {
            field.classList.remove("is-invalid");
            field.classList.remove("is-valid");
            return;
        }
        if (field.checkValidity()) {
            field.classList.add("is-valid");
            field.classList.remove("is-invalid");
        } else {
            field.classList.add("is-invalid");
            field.classList.remove("is-valid");
        }
    }

    var fields = [];
    form.querySelectorAll("input, select, textarea").forEach(function (f) {
        if (f.type !== "hidden") fields.push(f);
    });

    fields.forEach(function (field) {
        field.addEventListener("input", function () {
            if (field === passwordField || field === confirmField) {
                checkMatch();
            }
            markField(field, false);
            if (field === passwordField && confirmField && confirmField.value) {
                markField(confirmField, false);
            }
        });
        field.addEventListener("blur", function () {
            if (field === passwordField || field === confirmField) {
                checkMatch();
            }
            markField(field, false);
        });
    });

    form.addEventListener("submit", function (e) {
        checkMatch();
        fields.forEach(function (field) {
            markField(field, true);
        });
        if (!form.checkValidity()) {
            e.preventDefault();
            e.stopPropagation();
        }
    });

    if (options.modal) {
        options.modal.addEventListener("hidden.bs.modal", function () {
            if (confirmField) confirmField.setCustomValidity("");
            fields.forEach(function (field) {
                field.classList.remove("is-invalid");
                field.classList.remove("is-valid");
            });
        });
    }
}
