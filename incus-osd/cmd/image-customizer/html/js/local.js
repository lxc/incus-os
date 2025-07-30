function download() {
    req = {}
    req["seeds"] = {}

    // Process image type.
    if (document.getElementById("imageTypeISO").checked) {
        req["type"] = "iso"
    } else if (document.getElementById("imageTypeUSB").checked) {
        req["type"] = "raw"
    } else {
        alert("Missing image type")
        return
    }

    // Generate installation seed.
    if (document.getElementById("imageUsageInstallation").checked) {
        install = {}
        install["version"] = "1"

        if (document.getElementById("imageForceInstall").checked) {
            install["force_install"] = true
        }

        if (document.getElementById("imageForceReboot").checked) {
            install["force_reboot"] = true
        }

        req["seeds"]["install"] = install
    }

    // Generate Incus seed.
    if (document.getElementById("incusClientCertificate").value == "") {
        alert("Missing Incus client certificate")
        return
    }

    incus = {}
    incus["version"] = "1"

    if (document.getElementById("incusDefaults").checked) {
        incus["apply_defaults"] = true
    }

    certificate = {}
    certificate["name"] = "admin"
    certificate["type"] = "client"
    certificate["description"] = "Initial admin client"
    certificate["certificate"] = document.getElementById("incusClientCertificate").value

    incus["preseed"] = {}
    incus["preseed"]["certificates"] = [certificate]

    req["seeds"]["incus"] = incus

    // Send the request.
    fetch("/1.0/images", {
        method: "POST",
        body: JSON.stringify(req),
        headers: {
            "Content-Type": "application/json"
        }
    }).then(response => response.json()).then(function(response) {
        if (response["status_code"] != 200) {
            alert("Unable to generate the requested image")
            return
        }

        window.location.href = document.location.origin+response["metadata"]["image"];
    })
}
