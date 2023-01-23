document.addEventListener('DOMContentLoaded', function() {
    const p = new URLSearchParams(location.search);
    const reload = p.get("reload");
    const timeout = p.get("timeout");
    const c = document.querySelector("#autoreaload");
    this.timeout_sec = parseInt(timeout, 10) || 60;

    if (reload == "on") {
        c.checked = true;
        this.tid = setTimeout(() => {
            location.reload();
          }, 1000 * this.timeout_sec);
    }

    c.addEventListener("change", (e) => {
        const checked = e.target.checked;
        if (!checked) {
            clearTimeout(this.tid);
            this.tid = null;
        } else {
            const np = new URLSearchParams({"reload": "on", "timeout": 60});
            location.href = "/?" + np;
        }
    });
});