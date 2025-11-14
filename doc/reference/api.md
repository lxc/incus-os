# REST API

```{note}
The IncusOS API is typically proxied through an installed application, such as [Incus](applications/incus.md).

If interacting with the API manually, you will need to prefix `/os/` to correctly reach the IncusOS endpoints. For example, to get a list of applications you could run `curl https://1.2.3.4:8443/os/1.0/applications`.
```

```{warning}
The IncusOS debug API endpoints have no guarantee of API stability, and should not be used
in normal day-to-day operations.
```

<link rel="stylesheet" type="text/css" href="../../_static/swagger-ui/swagger-ui.css" ></link>
<link rel="stylesheet" type="text/css" href="../../_static/swagger-override.css" ></link>
<div id="swagger-ui"></div>

<script src="../../_static/swagger-ui/swagger-ui-bundle.js" charset="UTF-8"> </script>
<script src="../../_static/swagger-ui/swagger-ui-standalone-preset.js" charset="UTF-8"> </script>
<script>
window.onload = function() {
  // Begin Swagger UI call region
  const ui = SwaggerUIBundle({
    url: window.location.pathname +"../../rest-api.yaml",
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [],
    validatorUrl: "none",
    defaultModelsExpandDepth: -1,
    supportedSubmitMethods: []
  })
  // End Swagger UI call region

  window.ui = ui
}
</script>
