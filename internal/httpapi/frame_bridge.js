(function () {
  // Injected by Homelander into every site page. It asks our header for the
  // viewer's data and hands it to the page as a homelander:data message (and
  // in window.homelanderData). The header answers only a ready that carries
  // the nonce of this frame load ('hl' in the URL), and only through the
  // MessageChannel made here: the channel belongs to this document, so once
  // the frame navigates elsewhere nothing more reaches the new document.
  var nonce = new URLSearchParams(location.search).get('hl');
  if (!nonce || parent === window || typeof MessageChannel !== 'function') return;
  function ready() {
    var ch = new MessageChannel();
    ch.port1.onmessage = function (e) {
      var d = e.data;
      if (!d || d.type !== 'homelander:data') return;
      window.homelanderData = d;
      window.dispatchEvent(new MessageEvent('message', { data: d, source: parent }));
    };
    parent.postMessage({ type: 'homelander:ready', nonce: nonce }, '*', [ch.port2]);
  }
  // After the page's own scripts, so their listeners are in place.
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', ready);
  else ready();
})();
