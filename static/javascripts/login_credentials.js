// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

(function () {
    var buttons = document.querySelectorAll('.copy-temp-btn');
    if (!buttons.length) {
        return;
    }
    buttons.forEach(function (btn) {
        btn.addEventListener('click', function () {
            var target = btn.getAttribute('data-copy');
            if (!target) return;
            navigator.clipboard.writeText(target).then(function () {
                var original = btn.textContent;
                btn.textContent = 'Copied!';
                btn.classList.add('btn-success');
                btn.classList.remove('btn-outline-secondary');
                setTimeout(function () {
                    btn.textContent = original;
                    btn.classList.remove('btn-success');
                    btn.classList.add('btn-outline-secondary');
                }, 1500);
            }).catch(function () {
                btn.textContent = 'Copy failed';
                setTimeout(function () {
                    btn.textContent = 'Copy';
                }, 1500);
            });
        });
    });
})();
