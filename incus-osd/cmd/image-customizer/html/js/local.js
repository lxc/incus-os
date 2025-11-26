function certificate() {
    // Send the request.
    fetch("/1.0/certificate", {
        method: "GET",
    }).then(response => response.json()).then(function(response) {
        if (response.status_code != 200) {
            alert("Unable to get generated certificate");
            return;
        }

        // Set the certificate.
        document.getElementById("applicationClientCertificate").value = response.metadata.certificate;

        // Download the various files onto the client.
        const blobCert = new Blob([response.metadata.certificate], {type: 'application/x-pem-file'});
        const urlCert = window.URL.createObjectURL(blobCert);
        const aCert = document.createElement("a");
        aCert.href = urlCert;
        aCert.download = "client.crt";
        aCert.click();

        const blobKey = new Blob([response.metadata.key], {type: 'application/x-pem-file'});
        const urlKey = window.URL.createObjectURL(blobKey);
        const aKey = document.createElement("a");
        aKey.href = urlKey;
        aKey.download = "client.key";
        aKey.click();



        const byteString = window.atob(response.metadata.pfx);
        var bytesPfx = new ArrayBuffer(byteString.length);
        var ia = new Uint8Array(bytesPfx);
        for (var i = 0; i < byteString.length; i++) {
            ia[i] = byteString.charCodeAt(i);
        }

        const blobPfx = new Blob([bytesPfx], {type: 'application/x-pkcs12'});
        const urlPfx = window.URL.createObjectURL(blobPfx);
        const aPfx = document.createElement("a");
        aPfx.href = urlPfx;
        aPfx.download = "client.pfx";
        aPfx.click();

        var modalDialog = new bootstrap.Modal(document.getElementById("certificateModal"), {});
        modalDialog.show();
    });
}

function download() {
    req = {
        "seeds": {}
    };

    // Validate image type.
    if (document.getElementById("imageTypeISO").checked) {
        req.type = "iso";
    } else if (document.getElementById("imageTypeUSB").checked) {
        req.type = "raw";
    } else {
        alert("Missing image type");
        return;
    }

    // Validate image architecture.
    if (document.getElementById("imageArchitectureX86_64").checked) {
        req.architecture = "x86_64";
    } else if (document.getElementById("imageArchitectureAARCH64").checked) {
        req.architecture = "aarch64";
    } else {
        alert("Missing image architecture");
        return;
    }

    // Validate image application.
    app = "";
    if (document.getElementById("imageApplicationIncus").checked) {
        app = "incus";
    } else if (document.getElementById("imageApplicationOperationsCenter").checked) {
        app = "operations-center";
    } else if (document.getElementById("imageApplicationMigrationManager").checked) {
        app = "migration-manager";
    } else {
        alert("Missing image application");
        return;
    }

    // Validate client certificate.
    if (document.getElementById("applicationClientCertificate").value == "") {
        alert("Missing client certificate");
        return;
    }

    // Generate installation seed.
    if (document.getElementById("imageUsageInstallation").checked) {
        install = {
            "version": "1"
        };

        if (document.getElementById("imageForceInstall").checked) {
            install.force_install = true;
        }

        if (document.getElementById("imageForceReboot").checked) {
            install.force_reboot = true;
        }

        if (document.getElementById("imageInstallTarget").value != "") {
            install.target = {
                "id": document.getElementById("imageInstallTarget").value
            };
        }

        req.seeds.install = install;
    }

    // Add the network seed if provided.
    if (document.getElementById("networkConfiguration").value != "") {
        network = JSON.parse(document.getElementById("networkConfiguration").value)

        req.seeds.network = network;
    }

    // Generate application seed.
    req.seeds.applications = {
       "version": "1",
       "applications": [{"name": app}]
    };

    if (app == "incus") {
        certificate = {
            "name": "admin",
            "type": "client",
            "description": "Initial admin client",
            "certificate": document.getElementById("applicationClientCertificate").value
        };

        incus = {
            "version": "1",
            "preseed": {
                "certificates": [certificate]
            }
        };

        if (document.getElementById("applicationDefaults").checked) {
            incus.apply_defaults = true;
        }

        req.seeds.incus = incus;
    } else {
        appSeed = {
            "version": "1",
            "trusted_client_certificates": [document.getElementById("applicationClientCertificate").value]
        };

        req.seeds[app] = appSeed;
    }

    // Send the request.
    fetch("/1.0/images", {
        method: "POST",
        body: JSON.stringify(req),
        headers: {
            "Content-Type": "application/json"
        }
    }).then(response => response.json()).then(function(response) {
        if (response.status_code != 200) {
            alert("Unable to generate the requested image");
            return;
        }

        window.location.href = document.location.origin+response.metadata.image;
    });
}

;(function () {
    const htmlElement = document.querySelector("html")
    if(htmlElement.getAttribute("data-bs-theme") === 'auto') {
        function updateTheme() {
            document.querySelector("html").setAttribute("data-bs-theme",
                window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
        }

        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', updateTheme)
        updateTheme()
    }
})()
