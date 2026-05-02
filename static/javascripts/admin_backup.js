(function () {
    var modal = document.getElementById('importModal');
    if (!modal) {
        return;
    }
    var checkbox = modal.querySelector('#confirmCheckbox');
    var fileInput = modal.querySelector('#backupZipInput');
    var confirmBtn = modal.querySelector('#importConfirmBtn');
    function update() {
        confirmBtn.disabled = !(checkbox.checked && fileInput.files && fileInput.files.length > 0);
    }
    checkbox.addEventListener('change', update);
    fileInput.addEventListener('change', update);
})();
