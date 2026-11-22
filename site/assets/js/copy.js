// Copy buttons for commands and code blocks. Progressive enhancement:
// buttons ship with `hidden` and only appear when this script runs and the
// Clipboard API is available; the text stays selectable either way.
(function () {
  "use strict";
  if (!navigator.clipboard || !window.isSecureContext) return;

  var status = document.createElement("p");
  status.className = "copy-status";
  status.setAttribute("aria-live", "polite");
  document.body.appendChild(status);

  document.querySelectorAll("[data-copy]").forEach(function (btn) {
    var scope = btn.closest(".cmd, .code");
    var source = scope && scope.querySelector("[data-copy-source]");
    if (!source) return;
    btn.hidden = false;
    btn.addEventListener("click", function () {
      navigator.clipboard.writeText(source.textContent.trim()).then(
        function () {
          btn.textContent = "Copied";
          btn.dataset.state = "done";
          status.textContent = "Copied to clipboard";
          setTimeout(function () {
            btn.textContent = "Copy";
            delete btn.dataset.state;
            status.textContent = "";
          }, 1600);
        },
        function () {
          status.textContent = "Copy failed. Select the text instead.";
        }
      );
    });
  });
})();
