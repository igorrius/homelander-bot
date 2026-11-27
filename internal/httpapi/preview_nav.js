(function () {
  // Injected by Homelander into draft previews only: a link to another page
  // of this draft switches the preview instead of leaving for the live site.
  var pages = __PAGES__;
  document.addEventListener('click', function (e) {
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    var a = e.target && e.target.closest ? e.target.closest('a[href]') : null;
    if (!a) return;
    var url;
    try { url = new URL(a.getAttribute('href'), location.href); } catch (_) { return; }
    if (url.origin !== location.origin) return;
    var path = url.pathname.length > 1 ? url.pathname.replace(/\/+$/, '') : url.pathname;
    if (pages.indexOf(path) < 0) return;
    e.preventDefault();
    parent.postMessage({ type: 'homelander:navigate', path: path }, '*');
  }, true);
})();
