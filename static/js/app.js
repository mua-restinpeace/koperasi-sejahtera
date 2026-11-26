// Skrip ringan untuk tampilan. Tidak berisi aturan bisnis.
(function () {
  // Pesan sukses hilang sendiri setelah beberapa detik.
  document.querySelectorAll(".sukses").forEach(function (el) {
    setTimeout(function () {
      el.style.transition = "opacity .4s";
      el.style.opacity = "0";
      setTimeout(function () { el.remove(); }, 450);
    }, 5000);
  });
})();
