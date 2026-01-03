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

    // Validate release channel.
    if (document.getElementById("imageChannel").value != "") {
        req.channel = document.getElementById("imageChannel").value;
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

        if (document.getElementById("installSecurityNoTPM").checked) {
            install.security = {
                "missing_tpm": true
            };
        } else if (document.getElementById("installSecurityNoSecureBoot").checked) {
            install.security = {
                "missing_secure_boot": true
            };
        }

        req.seeds.install = install;
    }

    // Add the network seed if provided.
    if (document.getElementById("networkConfiguration").value != "") {
        network = JSON.parse(document.getElementById("networkConfiguration").value);

        req.seeds.network = network;
    }

    // Generate application seed.
    req.seeds.applications = {
       "version": "1",
       "applications": [{"name": app}]
    };

    hasOIDC = false;
    if (document.getElementById("applicationOIDCIssuer").value != "" && document.getElementById("applicationOIDCClientID").value != "") {
        hasOIDC = true;
    }

    if (app == "incus") {
        var certificate = {
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

        if (hasOIDC) {
            var config = {
                "oidc.issuer": document.getElementById("applicationOIDCIssuer").value,
                "oidc.client.id": document.getElementById("applicationOIDCClientID").value
            };

            if (document.getElementById("applicationOIDCClaim").value != "") {
                config["oidc.claim"] = document.getElementById("applicationOIDCClaim").value;
            }

            if (document.getElementById("applicationOIDCScopes").value != "") {
                config["oidc.scopes"] = document.getElementById("applicationOIDCScopes").value;
            }

            incus.preseed.config = config;
        }

        req.seeds.incus = incus;
    } else {
        appSeed = {
            "version": "1",
            "trusted_client_certificates": [document.getElementById("applicationClientCertificate").value]
        };

        if (hasOIDC) {
            var oidc = {
                "issuer": document.getElementById("applicationOIDCIssuer").value,
                "client_id": document.getElementById("applicationOIDCClientID").value
            };

            if (document.getElementById("applicationOIDCClaim").value != "") {
                oidc.claim = document.getElementById("applicationOIDCClaim").value;
            }

            if (document.getElementById("applicationOIDCScopes").value != "") {
                oidc.scopes = document.getElementById("applicationOIDCScopes").value;
            }

            appSeed.preseed = {};
            appSeed.preseed.system_security = {};
            appSeed.preseed.system_security.oidc = oidc;
        }

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
    const htmlElement = document.querySelector("html");
    if (htmlElement.getAttribute("data-bs-theme") === 'auto') {
        function updateTheme() {
            document.querySelector("html").setAttribute("data-bs-theme",
                window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
        }

        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', updateTheme);
        updateTheme();
    }
})();

function oidc() {
    var modalDialog = new bootstrap.Modal(document.getElementById("oidcModal"), {});
    modalDialog.show();
}

function oidcGenerate() {
    // Send the request.
    fetch("/1.0/oidc?username="+document.getElementById("oidcModalUsername").value, {
        method: "GET",
    }).then(response => response.json()).then(function(response) {
        if (response.status_code != 200) {
            alert("Unable to get generate an OIDC client, make sure the username is valid");
            return;
        }

        // Set the fields.
        document.getElementById("applicationOIDCIssuer").value = response.metadata.issuer;
        document.getElementById("applicationOIDCClientID").value = response.metadata.client_id;
        document.getElementById("applicationOIDCClaim").value = "preferred_username";
        document.getElementById("applicationOIDCScopes").value = "openid,offline_access";

        var modalDialog = new bootstrap.Modal(document.getElementById("oidcModal"), {});
        modalDialog.hide();
    });
}
