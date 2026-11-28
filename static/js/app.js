document.addEventListener("DOMContentLoaded", function () {
    const tabs = document.querySelectorAll("[data-payment-tab]");

    if (tabs.length === 0) {
        return;
    }

    const panels = {
        angsuran: document.getElementById("panel-angsuran"),
        pelunasan: document.getElementById("panel-pelunasan")
    };

    function aktifkanTab(jenis) {
        tabs.forEach(function (tab) {
            const aktif = tab.dataset.paymentTab === jenis;

            tab.classList.toggle("active", aktif);
            tab.setAttribute("aria-selected", String(aktif));
            tab.setAttribute("tabindex", aktif ? "0" : "-1");
        });

        Object.entries(panels).forEach(function ([nama, panel]) {
            if (panel) {
                panel.hidden = nama !== jenis;
            }
        });
    }

    tabs.forEach(function (tab, index) {
        tab.addEventListener("click", function () {
            aktifkanTab(tab.dataset.paymentTab);
        });

        tab.addEventListener("keydown", function (event) {
            if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") {
                return;
            }

            event.preventDefault();

            const arah = event.key === "ArrowRight" ? 1 : -1;
            const tujuan = (index + arah + tabs.length) % tabs.length;

            tabs[tujuan].focus();
            aktifkanTab(tabs[tujuan].dataset.paymentTab);
        });
    });

    aktifkanTab("angsuran");
});
